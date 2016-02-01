package plugin

import (
	"errors"
	"github.com/docker/libnetwork/driverapi"
	"github.com/docker/libnetwork/netlabel"

	. "github.com/xytis/deecper/common"
)

const (
	networkType         = "deecper"
	vethPrefix          = "veth"
	vethLen             = 7
	containerVethPrefix = "eth"
)

type driver struct {
	dhcpserver string
	version    string
	networks   networks
}

func New(version string, dhcpserver string) (driverapi.Driver, error) {
	driver := &driver{
		dhcpserver: dhcpserver,
		version:    version,
		networks:   networksNew(),
	}

	return driver, nil
}

// === INTERFACE ===

// CreateNetwork invokes the driver method to create a network passing
// the network id and network specific config. The config mechanism will
// eventually be replaced with labels which are yet to be introduced.
func (driver *driver) CreateNetwork(nid string, options map[string]interface{}, ipV4Data, ipV6Data []driverapi.IPAMData) error {
	Log.Debugf("Create network request %s %+v", nid, options)
	var (
		ifname string
		brname string
	)
	if args, ok := options[netlabel.GenericData].(map[string]interface{}); !ok {
		return ErrMissingParameterMap{}
	} else {
		if ifname, ok = args["iface"].(string); !ok || ifname == "" {
			return ErrMissingParam("iface")
		}
		if brname, ok = args["bridge"].(string); !ok {
			brname = "br_" + ifname
		}
	}
	return driver.networks.create(nid, ifname, brname)
}

// DeleteNetwork invokes the driver method to delete network passing
// the network id.
func (driver *driver) DeleteNetwork(nid string) error {
	Log.Debugf("Delete network request %s", nid)
	return driver.networks.delete(nid)
}

// CreateEndpoint invokes the driver method to create an endpoint
// passing the network id, endpoint id endpoint information and driver
// specific config. The endpoint information can be either consumed by
// the driver or populated by the driver. The config mechanism will
// eventually be replaced with labels which are yet to be introduced.
func (driver *driver) CreateEndpoint(nid, eid string, ifInfo driverapi.InterfaceInfo, options map[string]interface{}) error {
	Log.Debugf("Create endpoint request %s:%s", nid, eid)
	if ifInfo == nil {
		return errors.New("invalid interface info passed")
	}

	// Get the network handler and make sure it exists
	ni, err := driver.networks.get(nid)
	if err != nil {
		return err
	}
	if err := ni.endpoints.vacant(eid); err != nil {
		return err
	}

	return ni.endpoints.create(eid, ifInfo, ni.config)
}

// DeleteEndpoint invokes the driver method to delete an endpoint
// passing the network id and endpoint id.
func (driver *driver) DeleteEndpoint(nid, eid string) error {
	Log.Debugf("Delete endpoint request %s:%s", nid, eid)
	ni, err := driver.networks.get(nid)
	if err != nil {
		return err
	}
	return ni.endpoints.delete(eid)
}

// EndpointOperInfo retrieves from the driver the operational data related to the specified endpoint
func (driver *driver) EndpointOperInfo(nid, eid string) (map[string]interface{}, error) {
	return nil, nil
}

// Join method is invoked when a Sandbox is attached to an endpoint.
func (driver *driver) Join(nid, eid string, sboxKey string, jinfo driverapi.JoinInfo, options map[string]interface{}) error {
	Log.Debugf("Join requested %s:%s, sbox:%s", nid, eid, sboxKey)

	ni, err := driver.networks.get(nid)
	if err != nil {
		return err
	}

	ep, err := ni.endpoints.get(eid)
	if err != nil {
		return err
	}

	iNames := jinfo.InterfaceName()
	err = iNames.SetNames(ep.ifname, containerVethPrefix)
	if err != nil {
		return err
	}
	/*
		err = jinfo.SetGateway(ni.bridge.gatewayIPv4)
		if err != nil {
			return err
		}

		err = jinfo.SetGatewayIPv6(network.bridge.gatewayIPv6)
		if err != nil {
			return err
		}

		if !network.config.EnableICC {
			return d.link(network, endpoint, options, true)
		}
	*/

	return nil
}

// Leave method is invoked when a Sandbox detaches from an endpoint.
func (driver *driver) Leave(nid, eid string) error {

	return nil
}

// DiscoverNew is a notification for a new discovery event, Example:a new node joining a cluster
func (driver *driver) DiscoverNew(dType driverapi.DiscoveryType, data interface{}) error {
	return nil
}

// DiscoverDelete is a notification for a discovery delete event, Example:a node leaving a cluster
func (driver *driver) DiscoverDelete(dType driverapi.DiscoveryType, data interface{}) error {
	return nil
}

// Type returns the the type of this driver, the network type this driver manages
func (driver *driver) Type() string {
	return networkType
}

// === PRIVATE ===

func (driver *driver) lock() {
}
func (driver *driver) unlock() {
}
