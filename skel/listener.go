package skel

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"

	"github.com/gorilla/mux"

	"github.com/docker/libnetwork/driverapi"
	dapi "github.com/docker/libnetwork/drivers/remote/api"
	"github.com/docker/libnetwork/ipamapi"
	iapi "github.com/docker/libnetwork/ipams/remote/api"
	. "github.com/xytis/deecper/common"
)

const (
	networkReceiver = driverapi.NetworkPluginEndpointType
	ipamReceiver    = ipamapi.PluginEndpointType
)

type listenerNetwork struct {
	d driverapi.Driver
}
type listenerIpam struct {
	i ipamapi.Ipam
}

func Listen(socket net.Listener, networkDriver driverapi.Driver, ipamDriver ipamapi.Ipam) error {
	router := mux.NewRouter()
	router.NotFoundHandler = http.HandlerFunc(notFound)

	listenerNetwork := &listenerNetwork{networkDriver}
	listenerIpam := &listenerIpam{ipamDriver}

	router.Methods("POST").Path("/Plugin.Activate").HandlerFunc(handshake)

	handleMethod := func(receiver, method string, h http.HandlerFunc) {
		router.Methods("POST").Path(fmt.Sprintf("/%s.%s", receiver, method)).HandlerFunc(h)
	}

	handleMethod(networkReceiver, "GetCapabilities", listenerNetwork.getCapabilities)
	handleMethod(networkReceiver, "CreateNetwork", listenerNetwork.createNetwork)
	handleMethod(networkReceiver, "DeleteNetwork", listenerNetwork.deleteNetwork)
	handleMethod(networkReceiver, "CreateEndpoint", listenerNetwork.createEndpoint)
	handleMethod(networkReceiver, "DeleteEndpoint", listenerNetwork.deleteEndpoint)
	handleMethod(networkReceiver, "EndpointOperInfo", listenerNetwork.infoEndpoint)
	handleMethod(networkReceiver, "Join", listenerNetwork.joinEndpoint)
	handleMethod(networkReceiver, "Leave", listenerNetwork.leaveEndpoint)

	handleMethod(ipamReceiver, "GetCapabilities", listenerIpam.getCapabilities)
	handleMethod(ipamReceiver, "GetDefaultAddressSpaces", listenerIpam.getDefaultAddressSpaces)
	handleMethod(ipamReceiver, "RequestPool", listenerIpam.requestPool)
	handleMethod(ipamReceiver, "ReleasePool", listenerIpam.releasePool)
	handleMethod(ipamReceiver, "RequestAddress", listenerIpam.requestAddress)
	handleMethod(ipamReceiver, "ReleaseAddress", listenerIpam.releaseAddress)

	return http.Serve(socket, router)
}

func decode(w http.ResponseWriter, r *http.Request, v interface{}) error {
	err := json.NewDecoder(r.Body).Decode(v)
	if err != nil {
		sendError(w, "Unable to decode JSON payload: "+err.Error(), http.StatusBadRequest)
	}
	return err
}

// === protocol handlers

func handshake(w http.ResponseWriter, r *http.Request) {
	resp := struct {
		Implements []string
	}{}
	resp.Implements = append(resp.Implements, ipamReceiver, networkReceiver)
	err := json.NewEncoder(w).Encode(&resp)
	if err != nil {
		sendError(w, "encode error", http.StatusInternalServerError)
		return
	}
}

// === Network handlers

func (listener *listenerNetwork) getCapabilities(w http.ResponseWriter, r *http.Request) {
	var caps = &dapi.GetCapabilityResponse{
		Scope: "global",
	}
	objectOrErrorResponse(w, caps, nil)
}

func (listener *listenerNetwork) createNetwork(w http.ResponseWriter, r *http.Request) {
	var rq dapi.CreateNetworkRequest
	if err := decode(w, r, &rq); err != nil {
		return
	}
	emptyOrErrorResponse(w, listener.d.CreateNetwork(rq.NetworkID, rq.Options, rq.IPv4Data, rq.IPv6Data))
}

func (listener *listenerNetwork) deleteNetwork(w http.ResponseWriter, r *http.Request) {
	var rq dapi.DeleteNetworkRequest
	if err := decode(w, r, &rq); err != nil {
		return
	}
	emptyOrErrorResponse(w, listener.d.DeleteNetwork(rq.NetworkID))
}

type InterfaceInfoWrap struct {
	i *dapi.EndpointInterface
}

func (i *InterfaceInfoWrap) SetMacAddress(mac net.HardwareAddr) error {
	i.i.MacAddress = mac.String()
	return nil
}
func (i *InterfaceInfoWrap) SetIPAddress(ip *net.IPNet) error {
	i.i.Address = ip.String()
	i.i.AddressIPv6 = ip.String()
	return nil
}
func (i *InterfaceInfoWrap) MacAddress() net.HardwareAddr {
	hw, err := net.ParseMAC(i.i.MacAddress)
	if err != nil {
		Log.Warnln("Could not parse given mac address (%s)", i.i.MacAddress)
		return nil
	}
	return hw
}
func (i *InterfaceInfoWrap) Address() *net.IPNet {
	if _, net, err := net.ParseCIDR(i.i.Address); err != nil {
		return nil
	} else {
		return net
	}
}
func (i *InterfaceInfoWrap) AddressIPv6() *net.IPNet {
	if _, net, err := net.ParseCIDR(i.i.AddressIPv6); err != nil {
		return nil
	} else {
		return net
	}
}

func (listener *listenerNetwork) createEndpoint(w http.ResponseWriter, r *http.Request) {
	var rq dapi.CreateEndpointRequest
	if err := decode(w, r, &rq); err != nil {
		return
	}
	ifwrap := InterfaceInfoWrap{rq.Interface}
	err := listener.d.CreateEndpoint(rq.NetworkID, rq.EndpointID, &ifwrap, rq.Options)
	resp := &dapi.CreateEndpointResponse{}
	objectOrErrorResponse(w, resp, err)
}

func (listener *listenerNetwork) deleteEndpoint(w http.ResponseWriter, r *http.Request) {
	var rq dapi.DeleteEndpointRequest
	if err := decode(w, r, &rq); err != nil {
		return
	}
	emptyOrErrorResponse(w, listener.d.DeleteEndpoint(rq.NetworkID, rq.EndpointID))
}

func (listener *listenerNetwork) infoEndpoint(w http.ResponseWriter, r *http.Request) {
	var rq dapi.EndpointInfoRequest
	if err := decode(w, r, &rq); err != nil {
		return
	}
	info, err := listener.d.EndpointOperInfo(rq.NetworkID, rq.EndpointID)
	resp := &dapi.EndpointInfoResponse{Value: info}
	objectOrErrorResponse(w, resp, err)
}

func (listener *listenerNetwork) joinEndpoint(w http.ResponseWriter, r *http.Request) {
	var rq dapi.JoinRequest
	if err := decode(w, r, &rq); err != nil {
		return
	}
	err := listener.d.Join(rq.NetworkID, rq.NetworkID, rq.SandboxKey, nil, rq.Options)
	resp := &dapi.JoinResponse{
		InterfaceName: nil,
	}
	objectOrErrorResponse(w, resp, err)
}

func (listener *listenerNetwork) leaveEndpoint(w http.ResponseWriter, r *http.Request) {
	var rq dapi.LeaveRequest
	if err := decode(w, r, &rq); err != nil {
		return
	}
	emptyOrErrorResponse(w, listener.d.Leave(rq.NetworkID, rq.EndpointID))
}

func (listener *listenerNetwork) discoverNew(w http.ResponseWriter, r *http.Request) {
	var rq dapi.DiscoveryNotification
	if err := decode(w, r, &rq); err != nil {
		return
	}
	emptyOrErrorResponse(w, listener.d.DiscoverNew(rq.DiscoveryType, rq.DiscoveryData))
}

func (listener *listenerNetwork) discoverDelete(w http.ResponseWriter, r *http.Request) {
	var rq dapi.DiscoveryNotification
	if err := decode(w, r, &rq); err != nil {
		return
	}
	emptyOrErrorResponse(w, listener.d.DiscoverDelete(rq.DiscoveryType, rq.DiscoveryData))
}

// === Ipam handlers

func (listener *listenerIpam) getCapabilities(w http.ResponseWriter, r *http.Request) {
	Log.Debugln("Capabilities requested")
	caps := struct {
		RequiresMACAddress bool
	}{
		true,
	}
	objectOrErrorResponse(w, caps, nil)
}

func (listener *listenerIpam) getDefaultAddressSpaces(w http.ResponseWriter, r *http.Request) {
	Log.Debugln("Default address spaces requested")
	local, global, err := listener.i.GetDefaultAddressSpaces()
	response := &iapi.GetAddressSpacesResponse{
		LocalDefaultAddressSpace:  local,
		GlobalDefaultAddressSpace: global,
	}
	objectOrErrorResponse(w, response, err)
}

func (listener *listenerIpam) requestPool(w http.ResponseWriter, r *http.Request) {
	var rq iapi.RequestPoolRequest
	if err := decode(w, r, &rq); err != nil {
		return
	}
	Log.Debugln("Pool requested", rq)
	poolID, pool, data, err := listener.i.RequestPool(rq.AddressSpace, rq.Pool, rq.SubPool, rq.Options, rq.V6)
	if err != nil {
		errorResponse(w, err.Error())
		return
	}
	response := &iapi.RequestPoolResponse{
		PoolID: poolID,
		Pool:   pool.String(),
		Data:   data,
	}
	objectResponse(w, response)
}

func (listener *listenerIpam) releasePool(w http.ResponseWriter, r *http.Request) {
	var rq iapi.ReleasePoolRequest
	if err := decode(w, r, &rq); err != nil {
		return
	}
	Log.Debugln("Poll release requested", rq)
	err := listener.i.ReleasePool(rq.PoolID)
	emptyOrErrorResponse(w, err)
}

func (listener *listenerIpam) requestAddress(w http.ResponseWriter, r *http.Request) {
	var rq iapi.RequestAddressRequest
	if err := decode(w, r, &rq); err != nil {
		return
	}
	Log.Debugln("Address requested", rq)
	address, data, err := listener.i.RequestAddress(rq.PoolID, net.ParseIP(rq.Address), rq.Options)
	if err != nil {
		errorResponse(w, err.Error())
		return
	}
	response := &iapi.RequestAddressResponse{
		Address: address.String(),
		Data:    data,
	}
	objectResponse(w, response)
}

func (listener *listenerIpam) releaseAddress(w http.ResponseWriter, r *http.Request) {
	var rq iapi.ReleaseAddressRequest
	if err := decode(w, r, &rq); err != nil {
		return
	}
	Log.Debugln("Address release requested", rq)
	err := listener.i.ReleaseAddress(rq.PoolID, net.ParseIP(rq.Address))
	emptyOrErrorResponse(w, err)
}

// ===

func notFound(w http.ResponseWriter, r *http.Request) {
	http.NotFound(w, r)
}

func sendError(w http.ResponseWriter, msg string, code int) {
	http.Error(w, msg, code)
}

func errorResponse(w http.ResponseWriter, fmtString string, item ...interface{}) {
	json.NewEncoder(w).Encode(map[string]string{
		"Err": fmt.Sprintf(fmtString, item...),
	})
}

func objectResponse(w http.ResponseWriter, obj interface{}) {
	if err := json.NewEncoder(w).Encode(obj); err != nil {
		sendError(w, "Could not JSON encode response", http.StatusInternalServerError)
		return
	}
}

func emptyResponse(w http.ResponseWriter) {
	json.NewEncoder(w).Encode(map[string]string{})
}

func objectOrErrorResponse(w http.ResponseWriter, obj interface{}, err error) {
	if err != nil {
		errorResponse(w, err.Error())
		return
	}
	objectResponse(w, obj)
}

func emptyOrErrorResponse(w http.ResponseWriter, err error) {
	if err != nil {
		errorResponse(w, err.Error())
		return
	}
	emptyResponse(w)
}
