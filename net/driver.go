package plugin

import (
	"errors"
	driverapi "github.com/docker/go-plugins-helpers/network"
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
	scope    string
	networks networks
}

func NewDriver(scope string) (driverapi.Driver, error) {
	driver := &driver{
		scope:    scope,
		networks: networksNew(),
	}

	return driver, nil
}

func (driver *driver) GetCapabilities() (res *driverapi.CapabilitiesResponse, err error) {
	res = &driverapi.CapabilitiesResponse{
		Scope: driver.scope,
	}
	return
}

func (driver *driver) CreateNetwork(rq *driverapi.CreateNetworkRequest) error {
	Log.Debugf("Create network request %s %+v", rq.NetworkID, rq.Options)
	Log.Debugf("IPAM datas %v | %v", rq.IPv4Data, rq.IPv6Data)
	var (
		ifname string
		brname string
		labels map[string]interface{}
		ok     bool
	)
	if labels, ok = rq.Options[netlabel.GenericData].(map[string]interface{}); !ok {
		return ErrMissingParameterMap{}
	}
	if ifname, ok = labels["iface"].(string); !ok || ifname == "" {
		return ErrMissingParam("iface")
	}
	if brname, ok = labels["bridge"].(string); !ok {
		brname = "br_" + ifname
	}
	config := networkConfig{
		ParentName: ifname,
		BridgeName: brname,
		Mtu:        1500, //????
		EnableIPv6: false,
	}
	if err := config.parseIPAM(rq.NetworkID, rq.IPv4Data, rq.IPv6Data); err != nil {
		return err
	}
	if err := config.parseLabels(labels); err != nil {
		return err
	}
	if config.EnableIPv6 {
		Log.Warnf("IPV6 not supported. Go code it yourself!")
	}
	return driver.networks.create(rq.NetworkID, config)
}

func (driver *driver) DeleteNetwork(rq *driverapi.DeleteNetworkRequest) error {
	Log.Debugf("Delete network request %s", rq.NetworkID)
	return driver.networks.delete(rq.NetworkID)
}

func (driver *driver) CreateEndpoint(rq *driverapi.CreateEndpointRequest) error {
	Log.Debugf("Create endpoint request %s:%s", rq.NetworkID, rq.EndpointID)
	if rq.Interface == nil {
		return errors.New("invalid interface info passed")
	}

	// Get the network handler and make sure it exists
	ni, err := driver.networks.get(rq.NetworkID)
	if err != nil {
		return err
	}
	if err := ni.endpoints.vacant(rq.EndpointID); err != nil {
		return err
	}

	return ni.endpoints.create(rq.EndpointID, rq.Interface, ni.config)
}

func (driver *driver) DeleteEndpoint(rq *driverapi.DeleteEndpointRequest) error {
	Log.Debugf("Delete endpoint request %s:%s", rq.NetworkID, rq.EndpointID)
	ni, err := driver.networks.get(rq.NetworkID)
	if err != nil {
		return err
	}
	return ni.endpoints.delete(rq.EndpointID)
}

func (driver *driver) EndpointInfo(rq *driverapi.InfoRequest) (res *driverapi.InfoResponse, err error) {
	Log.Debugf("Info requested %s:%s", rq.NetworkID, rq.EndpointID)
	return
}

func (driver *driver) Join(rq *driverapi.JoinRequest) (res *driverapi.JoinResponse, err error) {
	Log.Debugf("Join requested %s:%s, sbox:%s", rq.NetworkID, rq.EndpointID, rq.SandboxKey)
	defer func() { Log.Debugf("Join response: res: %v, err: %v", res, err) }()

	ni, err := driver.networks.get(rq.NetworkID)
	if err != nil {
		return
	}

	ep, err := ni.endpoints.get(rq.EndpointID)
	if err != nil {
		return
	}

	res = &driverapi.JoinResponse{
		Gateway:       ni.config.GatewayIPv4.String(),
		InterfaceName: driverapi.InterfaceName{ep.ifname, containerVethPrefix},
	}

	return
}

func (driver *driver) Leave(rq *driverapi.LeaveRequest) error {
	Log.Debugf("Leave requested %s:%s", rq.NetworkID, rq.EndpointID)

	return nil
}
