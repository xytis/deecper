package plugin

import (
	"github.com/docker/libnetwork/driverapi"
	"github.com/vishvananda/netlink"
	. "github.com/xytis/deecper/common"
	"net"
	"sync"
)

type networks struct {
	sync.RWMutex
	store map[string]network
}

type network struct {
	brname    string
	endpoints endpoints
	config    networkConfiguration
}

type networkConfiguration struct {
	ID         string
	BridgeName string
	EnableIPv6 bool
	Mtu        int
	// Internal fields set after ipam data parsing
	AddressIPv4        *net.IPNet
	AddressIPv6        *net.IPNet
	DefaultGatewayIPv4 net.IP
	DefaultGatewayIPv6 net.IP
	dbIndex            uint64
	dbExists           bool
	Internal           bool
}

func networksNew() networks {
	return networks{
		store: make(map[string]network),
	}
}

func networkNew(brname string, config networkConfiguration) network {
	return network{
		brname:    brname,
		endpoints: endpointsNew(),
		config:    config,
	}
}

func (n *networks) create(nid, ifname, brname string) (err error) {
	defer func() { Log.Debugf("network create nid: %s, bridge: %s, error: %s", nid, brname, err) }()
	la := netlink.NewLinkAttrs()
	la.Name = brname
	br := &netlink.Bridge{la}
	if err := netlink.LinkAdd(br); err != nil {
		return ErrNetlinkError{"create bridge", err}
	}
	if li, err := netlink.LinkByName(ifname); err != nil {
		netlink.LinkDel(br)
		return ErrNetlinkError{"find iface by name (" + ifname + ")", err}
	} else if err := netlink.LinkSetMaster(li, br); err != nil {
		netlink.LinkDel(br)
		return ErrNetlinkError{"set bridge master", err}
	}
	if err := netlink.LinkSetUp(br); err != nil {
		return ErrNetlinkError{"bring bridge up", err}
	}

	n.add(nid, network{
		brname,
		endpointsNew(),
		networkConfiguration{
			ID:         nid,
			BridgeName: brname,
			EnableIPv6: false,
			//Mtu: ????,
		},
	})

	return
}

func (n *networks) delete(nid string) error {
	ni, err := n.get(nid)
	if err != nil {
		return err
	}
	la := netlink.NewLinkAttrs()
	la.Name = ni.brname
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
		return network{}, driverapi.ErrNoNetwork(nid)
	}
	return ni, nil
}

func (n *networks) rm(nid string) {
	n.RLock()
	delete(n.store, nid)
	n.RUnlock()
}
