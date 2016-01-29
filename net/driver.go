package plugin

import (
	"errors"
	"github.com/docker/libnetwork/driverapi"
	"github.com/docker/libnetwork/netlabel"
	"github.com/docker/libnetwork/netutils"
	"github.com/docker/libnetwork/types"
	"net"

	. "github.com/xytis/deecper/common"

	"github.com/vishvananda/netlink"
)

const (
	networkType         = "deecper"
	vethPrefix          = "veth"
	vethLen             = 7
	containerVethPrefix = "eth"
)

type endpoint struct {
	addr       *net.IPNet
	addrv6     *net.IPNet
	macAddress net.HardwareAddr
}

type network struct {
	brname    string
	endpoints map[string]endpoint
	config    networkConfiguration
}

func (n *network) getConfig() networkConfiguration {
	return n.config
}

type networkConfiguration struct {
	ID                 string
	BridgeName         string
	EnableIPv6         bool
	EnableIPMasquerade bool
	EnableICC          bool
	Mtu                int
	DefaultBindingIP   net.IP
	DefaultBridge      bool
	// Internal fields set after ipam data parsing
	AddressIPv4        *net.IPNet
	AddressIPv6        *net.IPNet
	DefaultGatewayIPv4 net.IP
	DefaultGatewayIPv6 net.IP
	dbIndex            uint64
	dbExists           bool
	Internal           bool
}

type driver struct {
	dhcpserver string
	version    string
	networks   map[string]network
}

func (driver *driver) lock() {
}
func (driver *driver) unlock() {
}

func New(version string, dhcpserver string) (driverapi.Driver, error) {
	driver := &driver{
		dhcpserver: dhcpserver,
		version:    version,
		networks:   make(map[string]network),
	}

	return driver, nil
}

// CreateNetwork invokes the driver method to create a network passing
// the network id and network specific config. The config mechanism will
// eventually be replaced with labels which are yet to be introduced.
func (driver *driver) CreateNetwork(nid string, options map[string]interface{}, ipV4Data, ipV6Data []driverapi.IPAMData) error {
	Log.Debugf("Create network request %s %+v", nid, options)
	var (
		iface  string
		brname string
	)
	if args, ok := options[netlabel.GenericData].(map[string]interface{}); !ok {
		return ErrMissingParameterMap{}
	} else {
		if iface, ok = args["iface"].(string); !ok || iface == "" {
			return ErrMissingParam("iface")
		}
		if brname, ok = args["bridge"].(string); !ok {
			brname = "br_" + iface
		}
	}
	la := netlink.NewLinkAttrs()
	la.Name = brname
	br := &netlink.Bridge{la}
	if err := netlink.LinkAdd(br); err != nil {
		return ErrNetlinkError{"create bridge", err}
	}
	if li, err := netlink.LinkByName(iface); err != nil {
		netlink.LinkDel(br)
		return ErrNetlinkError{"find iface by name (" + iface + ")", err}
	} else if err := netlink.LinkSetMaster(li, br); err != nil {
		netlink.LinkDel(br)
		return ErrNetlinkError{"set bridge master", err}
	}
	if err := netlink.LinkSetUp(br); err != nil {
		return ErrNetlinkError{"bring bridge up", err}
	}

	n := network{
		brname,
		make(map[string]endpoint),
		networkConfiguration{},
	}
	driver.networks[nid] = n
	return nil
}

// DeleteNetwork invokes the driver method to delete network passing
// the network id.
func (driver *driver) DeleteNetwork(nid string) error {
	Log.Debugf("Delete network request %s", nid)
	n := driver.networks[nid]
	la := netlink.NewLinkAttrs()
	la.Name = n.brname
	br := &netlink.Bridge{la}
	if err := netlink.LinkSetDown(br); err != nil {
		return ErrNetlinkError{"bring bridge down", err}
	}
	if err := netlink.LinkDel(br); err != nil {
		return ErrNetlinkError{"delete bridge", err}
	}
	return nil
}

// CreateEndpoint invokes the driver method to create an endpoint
// passing the network id, endpoint id endpoint information and driver
// specific config. The endpoint information can be either consumed by
// the driver or populated by the driver. The config mechanism will
// eventually be replaced with labels which are yet to be introduced.
func (driver *driver) CreateEndpoint(nid, eid string, ifInfo driverapi.InterfaceInfo, options map[string]interface{}) error {
	var err error
	if ifInfo == nil {
		return errors.New("invalid interface info passed")
	}

	// Get the network handler and make sure it exists
	driver.lock()
	n, ok := driver.networks[nid]
	driver.unlock()

	if !ok {
		return driverapi.ErrNoNetwork(nid)
	}

	ep, ok := n.endpoints[eid]

	// Endpoint with that id exists either on desired or other sandbox
	if ok {
		return driverapi.ErrEndpointExists(eid)
	}

	ep = endpoint{}
	n.endpoints[eid] = ep

	// On failure make sure to remove the endpoint
	defer func() {
		if err != nil {
			delete(n.endpoints, eid)
		}
	}()

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

	config := n.getConfig()

	// Add bridge inherited attributes to pipe interfaces
	if config.Mtu != 0 {
		err = netlink.LinkSetMTU(host, config.Mtu)
		if err != nil {
			return types.InternalErrorf("failed to set MTU on host interface %s: %v", hostIfName, err)
		}
		err = netlink.LinkSetMTU(sbox, config.Mtu)
		if err != nil {
			return types.InternalErrorf("failed to set MTU on sandbox interface %s: %v", containerIfName, err)
		}
	}

	// Attach host side pipe interface into the bridge
	if err = addToBridge(hostIfName, config.BridgeName); err != nil {
		return fmt.Errorf("adding interface %s to bridge %s failed: %v", hostIfName, config.BridgeName, err)
	}

	if !dconfig.EnableUserlandProxy {
		err = setHairpinMode(host, true)
		if err != nil {
			return err
		}
	}

	// Create the sandbox side pipe interface
	endpoint.srcName = containerIfName
	endpoint.macAddress = ifInfo.MacAddress()
	endpoint.addr = ifInfo.Address()
	endpoint.addrv6 = ifInfo.AddressIPv6()

	// Down the interface before configuring mac address.
	if err = netlink.LinkSetDown(sbox); err != nil {
		return fmt.Errorf("could not set link down for container interface %s: %v", containerIfName, err)
	}

	// Set the sbox's MAC. If specified, use the one configured by user, otherwise generate one based on IP.
	if endpoint.macAddress == nil {
		endpoint.macAddress = electMacAddress(epConfig, endpoint.addr.IP)
		if err := ifInfo.SetMacAddress(endpoint.macAddress); err != nil {
			return err
		}
	}
	err = netlink.LinkSetHardwareAddr(sbox, endpoint.macAddress)
	if err != nil {
		return fmt.Errorf("could not set mac address for container interface %s: %v", containerIfName, err)
	}

	// Up the host interface after finishing all netlink configuration
	if err = netlink.LinkSetUp(host); err != nil {
		return fmt.Errorf("could not set link up for host interface %s: %v", hostIfName, err)
	}

	if endpoint.addrv6 == nil && config.EnableIPv6 {
		var ip6 net.IP
		network := n.bridge.bridgeIPv6
		if config.AddressIPv6 != nil {
			network = config.AddressIPv6
		}

		ones, _ := network.Mask.Size()
		if ones > 80 {
			err = types.ForbiddenErrorf("Cannot self generate an IPv6 address on network %v: At least 48 host bits are needed.", network)
			return err
		}

		ip6 = make(net.IP, len(network.IP))
		copy(ip6, network.IP)
		for i, h := range endpoint.macAddress {
			ip6[i+10] = h
		}

		endpoint.addrv6 = &net.IPNet{IP: ip6, Mask: network.Mask}
		if err := ifInfo.SetIPAddress(endpoint.addrv6); err != nil {
			return err
		}
	}

	// Program any required port mapping and store them in the endpoint
	endpoint.portMapping, err = n.allocatePorts(epConfig, endpoint, config.DefaultBindingIP, d.config.EnableUserlandProxy)
	if err != nil {
		return err
	}

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
	return networkType
}
