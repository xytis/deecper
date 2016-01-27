package ipamplugin

import (
	"fmt"
	"net"
	"strings"

	"github.com/docker/libnetwork/ipamapi"
	"github.com/docker/libnetwork/netlabel"
	. "github.com/xytis/deecper/common"
)

const (
	WeaveContainer = "weave"
)

type ipam struct {
}

func NewIpam(version string) (ipamapi.Ipam, error) {
	return &ipam{}, nil
}

func (i *ipam) GetDefaultAddressSpaces() (string, string, error) {
	Log.Debugln("GetDefaultAddressSpaces")
	return "dhcplocal", "dhcpglobal", nil
}

func (i *ipam) RequestPool(addressSpace, pool, subPool string, options map[string]string, v6 bool) (poolname string, subnet *net.IPNet, data map[string]string, err error) {
	Log.Debugln("RequestPool", addressSpace, pool, subPool, options)
	defer func() { Log.Debugln("RequestPool returning", poolname, subnet, data, err) }()
	if pool == "" {
		//subnet, err = i.weave.DefaultSubnet()
	} else {
		_, subnet, err = net.ParseCIDR(pool)
	}
	if err != nil {
		return
	}
	iprange := subnet
	if subPool != "" {
		if _, iprange, err = net.ParseCIDR(subPool); err != nil {
			return
		}
	}
	// Cunningly-constructed pool "name" which gives us what we need later
	poolname = strings.Join([]string{"weave", subnet.String(), iprange.String()}, "-")
	// Pass back a fake "gateway address"; we don't actually use it,
	// so just give the network address.
	data = map[string]string{netlabel.Gateway: subnet.String()}
	return
}

func (i *ipam) ReleasePool(poolID string) error {
	Log.Debugln("ReleasePool", poolID)
	return nil
}

func (i *ipam) RequestAddress(poolID string, address net.IP, options map[string]string) (ip *net.IPNet, _ map[string]string, err error) {
	Log.Debugln("RequestAddress", poolID, address, options)
	defer func() { Log.Debugln("allocateIP returned", ip, err) }()
	// If we pass magic string "_" to weave IPAM it stores the address under its own string
	if poolID == "weavepool" { // old-style
		//ip, err = i.weave.AllocateIP("_")
		return
	}
	parts := strings.Split(poolID, "-")
	if len(parts) != 3 || parts[0] != "weave" {
		err = fmt.Errorf("Unrecognized pool ID: %s", poolID)
		return
	}
	var subnet, iprange *net.IPNet
	if _, subnet, err = net.ParseCIDR(parts[1]); err != nil {
		return
	}
	if _, iprange, err = net.ParseCIDR(parts[2]); err != nil {
		return
	}
	// We are lying slightly to IPAM here: the range is not a subnet
	Log.Debugln(iprange)
	//if ip, err = i.weave.AllocateIPInSubnet("_", iprange); err != nil {
	//	return
	//}
	ip.Mask = subnet.Mask // fix up the subnet we lied about
	return
}

func (i *ipam) ReleaseAddress(poolID string, address net.IP) error {
	Log.Debugln("ReleaseAddress", poolID, address)
	//return i.weave.ReleaseIP(address.String())
	return nil
}
