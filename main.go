package main

import (
	"flag"
	"fmt"
	"net"
	"os"

	"github.com/docker/libnetwork/driverapi"
	"github.com/docker/libnetwork/ipamapi"
	. "github.com/xytis/deecper/common"
	ipamplugin "github.com/xytis/deecper/ipam"
	netplugin "github.com/xytis/deecper/net"
	"github.com/xytis/deecper/skel"
)

var version = "(unreleased version)"

func main() {
	var (
		justVersion bool
		socket      string
		dhcpserver  string
		logLevel    string
		err         error
	)

	flag.BoolVar(&justVersion, "version", false, "print version and exit")
	flag.StringVar(&logLevel, "log-level", "info", "logging level (debug, info, warning, error)")
	flag.StringVar(&socket, "socket", "/run/docker/plugins/deecper.sock", "socket on which to listen")
	flag.StringVar(&dhcpserver, "server", "127.0.0.1", "dhcp server to relay queries to")

	flag.Parse()

	if justVersion {
		fmt.Printf("deecper plugin %s\n", version)
		os.Exit(0)
	}

	SetLogLevel(logLevel)

	Log.Println("Deecper plugin", version, "Command line options:", os.Args[1:])

	endChan := make(chan error, 1)
	if socket != "" {
		globalListener, err := listenAndServe(socket, dhcpserver, endChan)
		if err != nil {
			Log.Fatalf("unable to create driver: %s", err)
		}
		defer globalListener.Close()
	}

	err = <-endChan
	if err != nil {
		Log.Errorf("Error from listener: %s", err)
		os.Exit(1)
	}
}

func listenAndServe(socket string, dhcpserver string, endChan chan<- error) (net.Listener, error) {
	var i ipamapi.Ipam
	var d driverapi.Driver
	var err error
	if i, err = ipamplugin.New(version); err != nil {
		return nil, err
	}
	if d, err = netplugin.New(version, dhcpserver); err != nil {
		return nil, err
	}

	var listener net.Listener

	// remove sockets from last invocation
	if err := os.Remove(socket); err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	listener, err = net.Listen("unix", socket)
	if err != nil {
		return nil, err
	}
	Log.Printf("Listening on %s", socket)

	go func() {
		endChan <- skel.Listen(listener, d, i)
	}()

	return listener, nil
}
