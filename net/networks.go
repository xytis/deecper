package plugin

import (
	"encoding/json"
	"fmt"
	"github.com/docker/libkv/store"
	"github.com/docker/libnetwork/netlabel"
	"github.com/docker/libnetwork/types"
	"github.com/vishvananda/netlink"
	. "github.com/xytis/deecper/common"
	driverapi "github.com/xytis/go-plugins-helpers/network"
	"net"
	"strconv"
	"sync"
)

func _network(nid string) string {
	return "deecper/network/" + nid
}

type networks struct {
	sync.RWMutex
	parent netlink.Link
	store  map[string]network
	shared store.Store
}

type network struct {
	endpoints endpoints
	config    networkConfig
	watcher   chan struct{}
}

type networkConfig struct {
	LinkName   string
	BridgeName string
	Vlan       int
	Mtu        int
	EnableIPv6 bool
	// Internal fields set after ipam data parsing
	GatewayIPv4 net.IP
	GatewayIPv6 net.IP
}

func networksNew(li netlink.Link, st store.Store) networks {
	return networks{
		parent: li,
		store:  make(map[string]network),
		shared: st,
	}
}

func networkNew(config networkConfig) network {
	return network{
		endpoints: endpointsNew(),
		config:    config,
	}
}

func (n *networks) createLink(config networkConfig) error {
	//Link creation starts from checking if current vlan interface exists
	if _, err := netlink.LinkByName(config.LinkName); err != nil {
		//Try creating the link
		la := netlink.NewLinkAttrs()
		la.Name = config.LinkName
		la.ParentIndex = n.parent.Attrs().Index
		vl := &netlink.Vlan{la, config.Vlan}
		if err := netlink.LinkAdd(vl); err != nil {
			return ErrNetlinkError{"create vlan iface", err}
		}
		if err := netlink.LinkSetUp(vl); err != nil {
			return ErrNetlinkError{"bring vlan iface up", err}
		}
	}
	//Now check if bridge exists
	if _, err := netlink.LinkByName(config.BridgeName); err != nil {
		//Try creating the bridge
		la := netlink.NewLinkAttrs()
		la.Name = config.BridgeName
		br := &netlink.Bridge{la}
		if err := netlink.LinkAdd(br); err != nil {
			return ErrNetlinkError{"create bridge", err}
		}
		//Link bridge to new interface
		if li, err := netlink.LinkByName(config.LinkName); err != nil {
			netlink.LinkDel(br)
			return ErrNetlinkError{"find iface by name (" + config.LinkName + ")", err}
		} else if err := netlink.LinkSetMaster(li, br); err != nil {
			netlink.LinkDel(br)
			return ErrNetlinkError{"set bridge master", err}
		}
		if err := netlink.LinkSetUp(br); err != nil {
			return ErrNetlinkError{"bring bridge up", err}
		}
	}

	return nil
}

func (n *networks) deleteLink(config networkConfig) error {
	if li, err := netlink.LinkByName(config.BridgeName); err == nil {
		if err := netlink.LinkSetDown(li); err != nil {
			return ErrNetlinkError{"bring bridge down", err}
		}
		if err := netlink.LinkDel(li); err != nil {
			return ErrNetlinkError{"delete bridge", err}
		}
	}
	if li, err := netlink.LinkByName(config.LinkName); err == nil {
		if err := netlink.LinkSetDown(li); err != nil {
			return ErrNetlinkError{"bring vlan down", err}
		}
		if err := netlink.LinkDel(li); err != nil {
			return ErrNetlinkError{"delete vlan", err}
		}
	}
	return nil
}

func (n *networks) create(nid string, config networkConfig) (err error) {
	//Check if remote store contains the network configuration
	if n.existLocal(nid) || n.existGlobal(nid) {
		return fmt.Errorf("should not re-create existing network")
	}

	//Save config information to global storage
	if err := n.addGlobal(nid, config); err != nil {
		return err
	}

	//Save runtime information to local storage
	if err := n.addLocal(
		nid,
		network{
			endpointsNew(),
			config,
			make(chan struct{}),
		},
	); err != nil {
		return err
	}

	return
}

func (n *networks) delete(nid string) error {
	if n.existLocal(nid) {
		if ni, err := n.getLocal(nid); err != nil {
			return err
		} else {
			n.deleteLink(ni.config)
		}
		n.rmLocal(nid)
	}
	if n.existGlobal(nid) {
		if err := n.rmGlobal(nid); err != nil {
			return err
		}
	}
	return nil
}

func (n *networks) addGlobal(nid string, config networkConfig) error {
	//Upload new network definition to shared storage
	c, err := json.Marshal(config)
	if err != nil {
		return err
	}
	if err := n.shared.Put(_network(nid), c, nil); err != nil {
		return fmt.Errorf("could not write key %s, %v", nid, err)
	}
	return nil
}

func (n *networks) addLocal(nid string, network network) error {
	//Setup watcher for key delete
	go func() {
		stop := make(chan struct{})
		defer close(stop)
		Log.Errorf("Binding watcher for %s", nid)
		events, err := n.shared.Watch(_network(nid), stop)
		if err != nil {
			Log.Errorf("Could not watch for %s, %v", nid, err)
		}
		for {
			select {
			case pair := <-events:
				if pair == nil {
					//If local link exist:
					if net, err := n.getLocal(nid); err != nil {
						defer n.deleteLink(net.config)
					}
					defer n.rmLocal(nid)
					return
				}
			case <-network.watcher:
				return
			}
		}
	}()

	n.RLock()
	n.store[nid] = network
	n.RUnlock()
	return nil
}

func (n *networks) get(nid string) (network, error) {
	if n.existLocal(nid) {
		return n.getLocal(nid)
	}
	//Lookup global storage
	net, err := n.getGlobal(nid)
	if err == nil {
		// Cache localy
		n.addLocal(nid, net)
	}
	return net, err
}

func (n *networks) existLocal(nid string) bool {
	_, ok := n.store[nid]
	return ok
}

func (n *networks) existGlobal(nid string) bool {
	if exist, err := n.shared.Exists(_network(nid)); err != nil {
		return false
	} else {
		return exist
	}
}

func (n *networks) getLocal(nid string) (network, error) {
	//Check local storage
	n.RLock()
	ni, ok := n.store[nid]
	n.RUnlock()
	if !ok {
		return network{}, ErrNoNetwork(nid)
	}
	return ni, nil
}

func (n *networks) getGlobal(nid string) (network, error) {
	//Check if remote store contains the network configuration
	if pair, err := n.shared.Get(_network(nid)); err == nil {
		var c networkConfig
		if err := json.Unmarshal(pair.Value, &c); err != nil {
			return network{}, err
		}
		return networkNew(c), nil
	} else {
		return network{}, err
	}
}

func (n *networks) rmGlobal(nid string) error {
	if err := n.shared.Delete(_network(nid)); err != nil {
		return fmt.Errorf("could not delete key %s, %v", nid, err)
	}
	return nil
}

func (n *networks) rmLocal(nid string) {
	n.RLock()
	delete(n.store, nid)
	n.RUnlock()
}

func (c *networkConfig) parseIPAM(id string, ipamV4Data, ipamV6Data []*driverapi.IPAMData) error {
	if len(ipamV4Data) > 1 || len(ipamV6Data) > 1 {
		return types.ForbiddenErrorf("bridge driver doesni't support multiple subnets")
	}

	if len(ipamV4Data) == 0 {
		return types.BadRequestErrorf("bridge network %s requires ipv4 configuration", id)
	}

	if ipamV4Data[0].Gateway == "" {
		return types.BadRequestErrorf("bridge network %s requires ipv4 gateway from IPAM", id)
	}
	if gw, _, err := net.ParseCIDR(ipamV4Data[0].Gateway); err != nil {
		return err
	} else {
		Log.Debugf("IPAM: %v", *ipamV4Data[0])
		c.GatewayIPv4 = gw
	}

	return nil
}

func (c *networkConfig) parseLabels(labels map[string]interface{}) error {
	var err error
	for label, tlval := range labels {
		value := tlval.(string)
		switch label {
		case netlabel.DriverMTU:
			if c.Mtu, err = strconv.Atoi(value); err != nil {
				return parseErr(label, value, err.Error())
			}
		case netlabel.EnableIPv6:
			if c.EnableIPv6, err = strconv.ParseBool(value); err != nil {
				return parseErr(label, value, err.Error())
			}
		}
	}

	return nil
}

func parseErr(label, value, errString string) error {
	return types.BadRequestErrorf("failed to parse %s value: %v (%s)", label, value, errString)
}
