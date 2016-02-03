package plugin

import (
	"github.com/docker/libnetwork/netlabel"
	"github.com/docker/libnetwork/types"
	"github.com/vishvananda/netlink"
	. "github.com/xytis/deecper/common"
	driverapi "github.com/xytis/go-plugins-helpers/network"
	"net"
	"strconv"
	"sync"
)

type networks struct {
	sync.RWMutex
	store map[string]network
}

type network struct {
	endpoints endpoints
	config    networkConfig
}

type networkConfig struct {
	ParentName string
	BridgeName string
	Mtu        int
	EnableIPv6 bool
	// Internal fields set after ipam data parsing
	GatewayIPv4 net.IP
	GatewayIPv6 net.IP
	// Other stuff
	dbIndex  uint64
	dbExists bool
	Internal bool
}

func networksNew() networks {
	return networks{
		store: make(map[string]network),
	}
}

func networkNew(config networkConfig) network {
	return network{
		endpoints: endpointsNew(),
		config:    config,
	}
}

func (n *networks) create(nid string, config networkConfig) (err error) {
	la := netlink.NewLinkAttrs()
	la.Name = config.BridgeName
	br := &netlink.Bridge{la}
	if err := netlink.LinkAdd(br); err != nil {
		return ErrNetlinkError{"create bridge", err}
	}
	if li, err := netlink.LinkByName(config.ParentName); err != nil {
		netlink.LinkDel(br)
		return ErrNetlinkError{"find iface by name (" + config.ParentName + ")", err}
	} else if err := netlink.LinkSetMaster(li, br); err != nil {
		netlink.LinkDel(br)
		return ErrNetlinkError{"set bridge master", err}
	}
	if err := netlink.LinkSetUp(br); err != nil {
		return ErrNetlinkError{"bring bridge up", err}
	}

	n.add(nid, network{endpointsNew(), config})

	return
}

func (n *networks) delete(nid string) error {
	ni, err := n.get(nid)
	if err != nil {
		return err
	}
	la := netlink.NewLinkAttrs()
	la.Name = ni.config.BridgeName
	br := &netlink.Bridge{la}
	if err := netlink.LinkSetDown(br); err != nil {
		return ErrNetlinkError{"bring bridge down", err}
	}
	if err := netlink.LinkDel(br); err != nil {
		return ErrNetlinkError{"delete bridge", err}
	}
	n.rm(nid)
	return nil
}

func (n *networks) add(nid string, network network) {
	n.RLock()
	n.store[nid] = network
	n.RUnlock()
}

func (n *networks) get(nid string) (network, error) {
	n.RLock()
	ni, ok := n.store[nid]
	n.RUnlock()
	if !ok {
		return network{}, ErrNoNetwork(nid)
	}
	return ni, nil
}

func (n *networks) rm(nid string) {
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
