package plugin

import (
	"github.com/docker/libnetwork/driverapi"

	. "github.com/xytis/deecper/common"

	//"github.com/vishvananda/netlink"
)

type endpoint struct {
}

type driver struct {
	dhcpserver string
	version    string
	endpoints  map[string]endpoint
}

func New(version string, dhcpserver string) (driverapi.Driver, error) {
	driver := &driver{
		dhcpserver: dhcpserver,
		version:    version,
		endpoints:  make(map[string]endpoint),
	}

	return driver, nil
}

// CreateNetwork invokes the driver method to create a network passing
// the network id and network specific config. The config mechanism will
// eventually be replaced with labels which are yet to be introduced.
func (driver *driver) CreateNetwork(nid string, options map[string]interface{}, ipV4Data, ipV6Data []driverapi.IPAMData) error {
	Log.Debugf("Create network request %s %+v", nid, options)
	return nil
}

// DeleteNetwork invokes the driver method to delete network passing
// the network id.
func (driver *driver) DeleteNetwork(nid string) error {
	Log.Debugf("Delete network request %s", nid)
	return nil
}

// CreateEndpoint invokes the driver method to create an endpoint
// passing the network id, endpoint id endpoint information and driver
// specific config. The endpoint information can be either consumed by
// the driver or populated by the driver. The config mechanism will
// eventually be replaced with labels which are yet to be introduced.
func (driver *driver) CreateEndpoint(nid, eid string, ifInfo driverapi.InterfaceInfo, options map[string]interface{}) error {
	return nil
}

// DeleteEndpoint invokes the driver method to delete an endpoint
// passing the network id and endpoint id.
func (driver *driver) DeleteEndpoint(nid, eid string) error {
	return nil
}

// EndpointOperInfo retrieves from the driver the operational data related to the specified endpoint
func (driver *driver) EndpointOperInfo(nid, eid string) (map[string]interface{}, error) {
	return nil, nil
}

// Join method is invoked when a Sandbox is attached to an endpoint.
func (driver *driver) Join(nid, eid string, sboxKey string, jinfo driverapi.JoinInfo, options map[string]interface{}) error {
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
	return "deecper"
}
