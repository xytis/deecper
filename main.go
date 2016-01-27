package main

import (
	"flag"
	"fmt"
	"net"
	"os"

	"github.com/docker/libnetwork/ipamapi"
	. "github.com/xytis/deecper/common"
	ipamplugin "github.com/xytis/deecper/ipam"
	"github.com/xytis/deecper/skel"
)

var version = "(unreleased version)"

func main() {
	var (
		justVersion      bool
		address          string
		nameserver       string
		meshAddress      string
		logLevel         string
		noMulticastRoute bool
		err              error
	)

	flag.BoolVar(&justVersion, "version", false, "print version and exit")
	flag.StringVar(&logLevel, "log-level", "info", "logging level (debug, info, warning, error)")
	flag.StringVar(&address, "socket", "/run/docker/plugins/weave.sock", "socket on which to listen")
	flag.StringVar(&nameserver, "nameserver", "", "nameserver to provide to containers")
	flag.StringVar(&meshAddress, "meshsocket", "/run/docker/plugins/weavemesh.sock", "socket on which to listen in mesh mode")
	flag.BoolVar(&noMulticastRoute, "no-multicast-route", false, "do not add a multicast route to network endpoints")

	flag.Parse()

	if justVersion {
		fmt.Printf("weave plugin %s\n", version)
		os.Exit(0)
	}

	SetLogLevel(logLevel)

	Log.Println("Weave plugin", version, "Command line options:", os.Args[1:])

	endChan := make(chan error, 1)
	if address != "" {
		globalListener, err := listenAndServe(address, nameserver, noMulticastRoute, endChan, "global", false)
		if err != nil {
			Log.Fatalf("unable to create driver: %s", err)
		}
		defer globalListener.Close()
	}
	if meshAddress != "" {
		meshListener, err := listenAndServe(meshAddress, nameserver, noMulticastRoute, endChan, "local", true)
		if err != nil {
			Log.Fatalf("unable to create driver: %s", err)
		}
		defer meshListener.Close()
	}

	err = <-endChan
	if err != nil {
		Log.Errorf("Error from listener: %s", err)
		os.Exit(1)
	}
}

func listenAndServe(address, nameserver string, noMulticastRoute bool, endChan chan<- error, scope string, withIpam bool) (net.Listener, error) {
	var i ipamapi.Ipam
	var err error
	if withIpam {
		if i, err = ipamplugin.NewIpam(version); err != nil {
			return nil, err
		}
	}

	var listener net.Listener

	// remove sockets from last invocation
	if err := os.Remove(address); err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	listener, err = net.Listen("unix", address)
	if err != nil {
		return nil, err
	}
	Log.Printf("Listening on %s for %s scope", address, scope)

	go func() {
		endChan <- skel.Listen(listener, i)
	}()

	return listener, nil
}
