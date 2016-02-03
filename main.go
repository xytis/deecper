package main

import (
	"os"

	"github.com/codegangsta/cli"
	"github.com/docker/go-plugins-helpers/ipam"
	"github.com/docker/go-plugins-helpers/network"
	. "github.com/xytis/deecper/common"
	dipam "github.com/xytis/deecper/ipam"
	dnet "github.com/xytis/deecper/net"
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

	var flagScope = cli.StringFlag{
		Name:  "scope",
		Value: "global",
		Usage: "Driver scope",
	}

	app := cli.NewApp()
	app.Name = "deecper"
	app.Usage = "Docker dhcp enabled Networking"
	app.Version = version
	app.Flags = []cli.Flag{
		flagLogLevel,
		flagNoIPAM,
		flagNoNet,
		flagScope,
	}

	app.Action = Run
	app.Run(os.Args)

}

func Run(ctx *cli.Context) {
	SetLogLevel(ctx.String("log-level"))

	var (
		derr, ierr chan error
	)

	if !ctx.Bool("no-network") {
		d, err := dnet.NewDriver(ctx.String("scope"))
		if err != nil {
			panic(err)
		}
		h := network.NewHandler(d)
		derr = make(chan error)
		go func() {
			derr <- h.ServeUnix("root", "dnet")
		}()
		Log.Infof("Running Driver plugin 'dnet'")
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
