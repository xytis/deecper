package main

import (
	"os"

	"github.com/codegangsta/cli"
	"github.com/docker/go-plugins-helpers/ipam"
	"github.com/docker/go-plugins-helpers/network"
	. "github.com/xytis/polyp/common"
	dipam "github.com/xytis/polyp/ipam"
	dnet "github.com/xytis/polyp/net"
)

var version = "(unreleased version)"

func main() {

	var flagLogLevel = cli.StringFlag{
		Name:  "log-level",
		Value: "info",
		Usage: "logging level (debug, info, warning, error)",
	}

	var flagNoIPAM = cli.BoolFlag{
		Name:  "no-ipam",
		Usage: "Disable IPAM plugin",
	}

	var flagNoNet = cli.BoolFlag{
		Name:  "no-network",
		Usage: "Disable Network plugin",
	}

	var flagClusterStore = cli.StringFlag{
		Name:  "cluster-store, s",
		Value: "consul://127.0.0.1:8500",
		Usage: "cluster store for shared polyp data",
	}

	var flagInterface = cli.StringFlag{
		Name:  "interface, i",
		Value: "eth0",
		Usage: "primary interface for vlan binds",
	}

	app := cli.NewApp()
	app.Name = "polyp"
	app.Usage = "Docker dhcp enabled Networking"
	app.Version = version
	app.Flags = []cli.Flag{
		flagLogLevel,
		flagNoIPAM,
		flagNoNet,
		flagClusterStore,
		flagInterface,
	}

	app.Action = Run
	app.Run(os.Args)

}

func Run(ctx *cli.Context) {
	SetLogLevel(ctx.String("log-level"))

	var (
		derr, ierr chan error
	)

	//Bind to cluster store
	store, err := NewStore(ctx.String("cluster-store"))
	if err != nil {
		panic(err)
	}

	if !ctx.Bool("no-network") {
		d, err := dnet.NewDriver("global", ctx.String("interface"), store)
		if err != nil {
			panic(err)
		}
		h := network.NewHandler(d)
		derr = make(chan error)
		go func() {
			derr <- h.ServeUnix("root", "dnet")
		}()
		Log.Infof("Running Driver plugin 'dnet', bound on interface %s", ctx.String("interface"))
	}

	if !ctx.Bool("no-ipam") {
		i, err := dipam.NewIpam()
		if err != nil {
			panic(err)
		}
		h := ipam.NewHandler(i)
		ierr = make(chan error)
		go func() {
			ierr <- h.ServeUnix("root", "dhcp")
		}()
		Log.Infof("Running IPAM plugin 'dhcp'")
	}

	if derr == nil && ierr == nil {
		Log.Errorf("You started the daemon without anything to do")
		os.Exit(127)
	}

	select {
	case err := <-derr:
		panic(err)
	case err := <-ierr:
		panic(err)
	}
}
