package skel

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"

	"github.com/gorilla/mux"

	//"github.com/docker/libnetwork/drivers/remote/api"
	"github.com/docker/libnetwork/ipamapi"
	iapi "github.com/docker/libnetwork/ipams/remote/api"
)

const (
	ipamReceiver = ipamapi.PluginEndpointType
)

type listener struct {
	i ipamapi.Ipam
}

func Listen(socket net.Listener, ipamDriver ipamapi.Ipam) error {
	router := mux.NewRouter()
	router.NotFoundHandler = http.HandlerFunc(notFound)

	listener := &listener{ipamDriver}

	router.Methods("POST").Path("/Plugin.Activate").HandlerFunc(listener.handshake)

	handleMethod := func(receiver, method string, h http.HandlerFunc) {
		router.Methods("POST").Path(fmt.Sprintf("/%s.%s", receiver, method)).HandlerFunc(h)
	}

	handleMethod(ipamReceiver, "GetCapabilities", listener.getCapabilities)
	handleMethod(ipamReceiver, "GetDefaultAddressSpaces", listener.getDefaultAddressSpaces)
	handleMethod(ipamReceiver, "RequestPool", listener.requestPool)
	handleMethod(ipamReceiver, "ReleasePool", listener.releasePool)
	handleMethod(ipamReceiver, "RequestAddress", listener.requestAddress)
	handleMethod(ipamReceiver, "ReleaseAddress", listener.releaseAddress)

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

type handshakeResp struct {
	Implements []string
}

func (listener *listener) handshake(w http.ResponseWriter, r *http.Request) {
	var resp handshakeResp
	if listener.i != nil {
		resp.Implements = append(resp.Implements, "IpamDriver")
	}
	err := json.NewEncoder(w).Encode(&resp)
	if err != nil {
		sendError(w, "encode error", http.StatusInternalServerError)
		return
	}
}

func (listener *listener) getCapabilities(w http.ResponseWriter, r *http.Request) {
	caps := struct {
		RequiresMACAddress bool
	}{
		true,
	}
	objectOrErrorResponse(w, caps, nil)
}

func (listener *listener) getDefaultAddressSpaces(w http.ResponseWriter, r *http.Request) {
	local, global, err := listener.i.GetDefaultAddressSpaces()
	response := &iapi.GetAddressSpacesResponse{
		LocalDefaultAddressSpace:  local,
		GlobalDefaultAddressSpace: global,
	}
	objectOrErrorResponse(w, response, err)
}

func (listener *listener) requestPool(w http.ResponseWriter, r *http.Request) {
	var rq iapi.RequestPoolRequest
	if err := decode(w, r, &rq); err != nil {
		return
	}
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

func (listener *listener) releasePool(w http.ResponseWriter, r *http.Request) {
	var rq iapi.ReleasePoolRequest
	if err := decode(w, r, &rq); err != nil {
		return
	}
	err := listener.i.ReleasePool(rq.PoolID)
	emptyOrErrorResponse(w, err)
}

func (listener *listener) requestAddress(w http.ResponseWriter, r *http.Request) {
	var rq iapi.RequestAddressRequest
	if err := decode(w, r, &rq); err != nil {
		return
	}
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

func (listener *listener) releaseAddress(w http.ResponseWriter, r *http.Request) {
	var rq iapi.ReleaseAddressRequest
	if err := decode(w, r, &rq); err != nil {
		return
	}
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
