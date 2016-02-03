package ipamplugin

import (
	"fmt"
	"net"
	"strings"

	client "github.com/d2g/dhcp4client"
	"github.com/docker/libnetwork/netlabel"
	. "github.com/xytis/deecper/common"
	ipamapi "github.com/xytis/go-plugins-helpers/ipam"
)

const (
	PoolName    = `dhcp`
	LocalSpace  = `dhcp-local`
	GlobalSpace = `dhcp-global`
)

type ipam struct {
	client *client.Client
}

func NewIpam() (ipamapi.Ipam, error) {
	localAddr := client.SetLocalAddr(net.UDPAddr{IP: net.IPv4(0, 0, 0, 0), Port: 2068})
	remoteAddr := client.SetRemoteAddr(net.UDPAddr{IP: net.ParseIP("192.168.72.254"), Port: 67})
	sock, err := client.NewInetSock(
		localAddr,
		remoteAddr,
	)
	if err != nil {
		return nil, err
	}
	if client, err := client.New(client.Connection(sock)); err != nil {
		return nil, err
	} else {
		return &ipam{
			client,
		}, nil
	}
}

func (i *ipam) GetCapabilities() (res *ipamapi.CapabilitiesResponse, err error) {
	res = &ipamapi.CapabilitiesResponse{
		RequiresMACAddress: true,
	}
	return
}

func (i *ipam) GetDefaultAddressSpaces() (res *ipamapi.AddressSpacesResponse, err error) {
	Log.Debugln("GetDefaultAddressSpaces")
	res = &ipamapi.AddressSpacesResponse{
		LocalDefaultAddressSpace:  LocalSpace,
		GlobalDefaultAddressSpace: GlobalSpace,
	}
	return
}

func (i *ipam) RequestPool(rq *ipamapi.RequestPoolRequest) (res *ipamapi.RequestPoolResponse, err error) {
	Log.Debugf("RequestPool %v", rq)
	defer func() { Log.Debugf("RequestPool returning res: %v, err: %v", res, err) }()

	p, err := i.client.SendDiscoverPacket()
	Log.Infof("Response: %v, (err: %v)", p, err)
	o, err := i.client.GetOffer(&p)
	Log.Infof("Offer: %v, (err: %v)", o, err)

	var (
		subnet, iprange *net.IPNet
	)
	if rq.Pool == "" {
		_, subnet, err = net.ParseCIDR("172.13.0.1/24")
	} else {
		_, subnet, err = net.ParseCIDR(rq.Pool)
	}
	if err != nil {
		return
	}
	iprange = subnet
	if rq.SubPool != "" {
		if _, iprange, err = net.ParseCIDR(rq.SubPool); err != nil {
			return
		}
	}
	// Cunningly-constructed pool "name" which gives us what we need later
	poolname := strings.Join([]string{PoolName, subnet.String(), iprange.String()}, "-")
	// Pass back a fake "gateway address"; we don't actually use it,
	// so just give the network address.
	data := map[string]string{
		netlabel.Gateway: "172.13.0.1/24",
	}
	res = &ipamapi.RequestPoolResponse{
		PoolID: poolname,
		Pool:   subnet.String(),
		Data:   data,
	}
	return
}

func (i *ipam) ReleasePool(rq *ipamapi.ReleasePoolRequest) error {
	Log.Debugln("ReleasePool", rq.PoolID)
	return nil
}

func (i *ipam) RequestAddress(rq *ipamapi.RequestAddressRequest) (res *ipamapi.RequestAddressResponse, err error) {
	Log.Debugln("RequestAddress %v", rq)
	defer func() { Log.Debugln("RequestAddress returned res: %v, err: %v", res, err) }()
	options := rq.Options
	macAddr, err := net.ParseMAC(options[netlabel.MacAddress])
	if err != nil {
		err = fmt.Errorf("Mac address not understood %v", options[netlabel.MacAddress])
		return
	}
	parts := strings.Split(rq.PoolID, "-")
	if len(parts) != 3 || parts[0] != PoolName {
		err = fmt.Errorf("Unrecognized pool ID: %s", rq.PoolID)
		return
	}
	var subnet, iprange *net.IPNet
	if _, subnet, err = net.ParseCIDR(parts[1]); err != nil {
		return
	}
	if _, iprange, err = net.ParseCIDR(parts[2]); err != nil {
		return
	}
	Log.Debugf("We should query DHCP with: mac %v, subnet %v, iprange %v", macAddr, subnet, iprange)
	res = &ipamapi.RequestAddressResponse{
		Address: "172.13.0.84/24",
		Data: map[string]string{
			"ohay": "Good day to you too, sir!",
		},
	}
	return
}

func (i *ipam) ReleaseAddress(rq *ipamapi.ReleaseAddressRequest) (err error) {
	Log.Debugln("ReleaseAddress %v", rq)
	return
}
