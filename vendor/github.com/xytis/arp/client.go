package arp

import (
	"errors"
	"net"
	"time"

	"github.com/mdlayher/ethernet"
	"github.com/mdlayher/raw"
)

var (
	// errNoIPv4Addr is returned when an interface does not have an IPv4
	// address.
	errNoIPv4Addr = errors.New("no IPv4 address available for interface")
)

// A Client is an ARP client, which can be used to send and receive
// ARP packets.
type Client struct {
	ifi *net.Interface
	p   net.PacketConn
}

// NewClient creates a new Client using the specified network interface.
// NewClient retrieves the IPv4 address of the interface and binds a raw socket
// to send and receive ARP packets.
func NewClient(ifi *net.Interface) (*Client, error) {
	// Open raw socket to send and receive ARP packets using ethernet frames
	// we build ourselves
	p, err := raw.ListenPacket(ifi, raw.ProtocolARP)
	if err != nil {
		return nil, err
	}

	// Check for usable IPv4 addresses for the Client
	addrs, err := ifi.Addrs()
	if err != nil {
		return nil, err
	}

	return newClient(ifi, p, addrs)
}

// newClient is the internal, generic implementation of newClient.  It is used
// to allow an arbitrary net.PacketConn to be used in a Client, so testing
// is easier to accomplish.
func newClient(ifi *net.Interface, p net.PacketConn, addrs []net.Addr) (*Client, error) {
	return &Client{
		ifi: ifi,
		p:   p,
	}, nil
}

// Close closes the Client's raw socket and stops sending and receiving
// ARP packets.
func (c *Client) Close() error {
	return c.p.Close()
}

func (c *Client) BroadcastChange(ip net.IP, mac net.HardwareAddr) error {
	// (sender mac, sender ip, target mac, target ip)
	arp, err := NewPacket(OperationRequest, mac, ip, ethernet.Broadcast, ip)
	if err != nil {
		return err
	}
	return c.WriteTo(arp, ethernet.Broadcast)
}

// Read reads a single ARP packet and returns it, together with its
// ethernet frame.
func (c *Client) Read() (*Packet, *ethernet.Frame, error) {
	buf := make([]byte, 128)
	for {
		n, _, err := c.p.ReadFrom(buf)
		if err != nil {
			return nil, nil, err
		}

		p, eth, err := parsePacket(buf[:n])
		if err != nil {
			if err == errInvalidARPPacket {
				continue
			}
			return nil, nil, err
		}
		return p, eth, nil
	}
}

// WriteTo writes a single ARP packet to addr. Note that addr should,
// but doesn't have to, match the target hardware address of the ARP
// packet.
func (c *Client) WriteTo(p *Packet, addr net.HardwareAddr) error {
	pb, err := p.MarshalBinary()
	if err != nil {
		return err
	}

	f := &ethernet.Frame{
		Destination: p.TargetHardwareAddr,
		Source:      p.SenderHardwareAddr,
		EtherType:   ethernet.EtherTypeARP,
		Payload:     pb,
	}

	fb, err := f.MarshalBinary()
	if err != nil {
		return err
	}

	_, err = c.p.WriteTo(fb, &raw.Addr{HardwareAddr: addr})
	return err
}

// Reply constructs and sends a reply to an ARP request. On the ARP
// layer, it will be addressed to the sender address of the packet. On
// the ethernet layer, it will be sent to the actual remote address
// from which the request was received.
//
// For more fine-grained control, use WriteTo to write a custom
// response.
func (c *Client) Reply(req *Packet, hwAddr net.HardwareAddr, ip net.IP) error {
	p, err := NewPacket(OperationReply, hwAddr, ip, req.SenderHardwareAddr, req.SenderIP)
	if err != nil {
		return err
	}
	return c.WriteTo(p, req.SenderHardwareAddr)
}

// Copyright (c) 2012 The Go Authors. All rights reserved.
// Source code in this file is based on src/net/interface_linux.go,
// from the Go standard library.  The Go license can be found here:
// https://golang.org/LICENSE.

// Documentation taken from net.PacketConn interface.  Thanks:
// http://golang.org/pkg/net/#PacketConn.

// SetDeadline sets the read and write deadlines associated with the
// connection.
func (c *Client) SetDeadline(t time.Time) error {
	return c.p.SetDeadline(t)
}

// SetReadDeadline sets the deadline for future raw socket read calls.
// If the deadline is reached, a raw socket read will fail with a timeout
// (see type net.Error) instead of blocking.
// A zero value for t means a raw socket read will not time out.
func (c *Client) SetReadDeadline(t time.Time) error {
	return c.p.SetReadDeadline(t)
}

// SetWriteDeadline sets the deadline for future raw socket write calls.
// If the deadline is reached, a raw socket write will fail with a timeout
// (see type net.Error) instead of blocking.
// A zero value for t means a raw socket write will not time out.
// Even if a write times out, it may return n > 0, indicating that
// some of the data was successfully written.
func (c *Client) SetWriteDeadline(t time.Time) error {
	return c.p.SetWriteDeadline(t)
}

// firstIPv4Addr attempts to retrieve the first detected IPv4 address from an
// input slice of network addresses.
func firstIPv4Addr(addrs []net.Addr) (net.IP, error) {
	for _, a := range addrs {
		if a.Network() != "ip+net" {
			continue
		}

		ip, _, err := net.ParseCIDR(a.String())
		if err != nil {
			return nil, err
		}

		// "If ip is not an IPv4 address, To4 returns nil."
		// Reference: http://golang.org/pkg/net/#IP.To4
		if ip4 := ip.To4(); ip4 != nil {
			return ip4, nil
		}
	}

	return nil, errNoIPv4Addr
}
