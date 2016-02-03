package plugin

import (
	"fmt"
	driverapi "github.com/docker/go-plugins-helpers/network"
	"github.com/docker/libnetwork/netutils"
	"github.com/docker/libnetwork/types"
	"github.com/vishvananda/netlink"
	. "github.com/xytis/deecper/common"
	"net"
	"sync"
)

type endpoint struct {
	ifname string
	addr   *net.IPNet
	addrv6 *net.IPNet
	mac    net.HardwareAddr
}

type endpoints struct {
	sync.RWMutex
	store map[string]endpoint
}

func endpointsNew() endpoints {
	return endpoints{
		store: make(map[string]endpoint),
	}
}

func (e *endpoints) create(eid string, ifInfo *driverapi.EndpointInterface, niConfig networkConfig) (err error) {
	ep := endpoint{}

	// Generate a name for what will be the host side pipe interface
	hostIfName, err := netutils.GenerateIfaceName(vethPrefix, vethLen)
	if err != nil {
		return err
	}

	// Generate a name for what will be the sandbox side pipe interface
	containerIfName, err := netutils.GenerateIfaceName(vethPrefix, vethLen)
	if err != nil {
		return err
	}

	// Generate and add the interface pipe host <-> sandbox
	veth := &netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{Name: hostIfName, TxQLen: 0},
		PeerName:  containerIfName}
	if err = netlink.LinkAdd(veth); err != nil {
		return types.InternalErrorf("failed to add the host (%s) <=> sandbox (%s) pair interfaces: %v", hostIfName, containerIfName, err)
	}

	// Get the host side pipe interface handler
	host, err := netlink.LinkByName(hostIfName)
	if err != nil {
		return types.InternalErrorf("failed to find host side interface %s: %v", hostIfName, err)
	}
	defer func() {
		if err != nil {
			netlink.LinkDel(host)
		}
	}()

	// Get the sandbox side pipe interface handler
	sbox, err := netlink.LinkByName(containerIfName)
	if err != nil {
		return types.InternalErrorf("failed to find sandbox side interface %s: %v", containerIfName, err)
	}
	defer func() {
		if err != nil {
			netlink.LinkDel(sbox)
		}
	}()

	// Add bridge inherited attributes to pipe interfaces
	if niConfig.Mtu != 0 {
		err = netlink.LinkSetMTU(host, niConfig.Mtu)
		if err != nil {
			return types.InternalErrorf("failed to set MTU on host interface %s: %v", hostIfName, err)
		}
		err = netlink.LinkSetMTU(sbox, niConfig.Mtu)
		if err != nil {
			return types.InternalErrorf("failed to set MTU on sandbox interface %s: %v", containerIfName, err)
		}
	}

	// Attach host side pipe interface into the bridge
	br, err := netlink.LinkByName(niConfig.BridgeName)
	if err != nil {
		return types.InternalErrorf("failed to find bridge by name %s: %v", niConfig.BridgeName, err)
	}
	if err = netlink.LinkSetMaster(host, br.(*netlink.Bridge)); err != nil {
		return fmt.Errorf("adding interface %s to bridge %s failed: %v", hostIfName, niConfig.BridgeName, err)
	}

	// Create the sandbox side pipe interface
	ep.ifname = containerIfName
	_, ep.addr, err = net.ParseCIDR(ifInfo.Address)
	if err != nil {
		return fmt.Errorf("ipv4 adress unparseable")
	}
	/*
		_, ep.addrv6, err = net.ParseCIDR(ifInfo.AddressIPv6)
		if err != nil {
			return fmt.Errorf("ipv6 adress unparseable")
		}
	*/

	if ifInfo.MacAddress != "" {
		ep.mac, err = net.ParseMAC(ifInfo.MacAddress)
		if err != nil {
			return fmt.Errorf("mac adress unparseable")
		}
		// Down the interface before configuring mac address.
		if err = netlink.LinkSetDown(sbox); err != nil {
			return fmt.Errorf("could not set link down for container interface %s: %v", containerIfName, err)
		}

		err = netlink.LinkSetHardwareAddr(sbox, ep.mac)
		if err != nil {
			return fmt.Errorf("could not set mac address for container interface %s: %v", containerIfName, err)
		}
	}

	// Up the host interface after finishing all netlink configuration
	if err = netlink.LinkSetUp(host); err != nil {
		return fmt.Errorf("could not set link up for host interface %s: %v", hostIfName, err)
	}

	if ep.addrv6 == nil && niConfig.EnableIPv6 {
		return fmt.Errorf("IPV6 is not supported. Go and code it yourself.")
	}

	e.add(eid, ep)
	return nil
}

func (e *endpoints) delete(eid string) (err error) {
	ep, err := e.get(eid)
	if err != nil {
		return err
	}
	//Set up ep return in case we fail mid way.
	//We remove ep early in order to mark resource as unavailable during removal.
	defer func() {
		if err != nil {
			if e.vacant(eid) == nil {
				e.add(eid, ep)
			}
		}
	}()
	e.rm(eid)

	// Also make sure defer does not see this error either.
	if link, err := netlink.LinkByName(ep.ifname); err == nil {
		return netlink.LinkDel(link)
	}

	return
}

func (e *endpoints) add(eid string, endpoint endpoint) {
	e.RLock()
	e.store[eid] = endpoint
	e.RUnlock()
}

func (e *endpoints) vacant(eid string) error {
	e.RLock()
	_, ok := e.store[eid]
	e.RUnlock()
	if ok {
		return ErrEndpointExists(eid)
	}
	return nil
}

func (e *endpoints) get(eid string) (endpoint, error) {
	e.RLock()
	ep, ok := e.store[eid]
	e.RUnlock()
	if !ok {
		return endpoint{}, ErrNoEndpoint(eid)
	}
	return ep, nil
}

func (e *endpoints) rm(eid string) {
	e.RLock()
	delete(e.store, eid)
	e.RUnlock()
}
