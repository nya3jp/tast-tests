// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package network provides general CrOS network goodies.
package network

import (
	"context"
	"fmt"
	"net"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"
	"golang.org/x/sys/unix"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// dhcpTestServer is a simple DHCP server you can program with expectations of
// future packets and responses to those packets.  The server is basically a
// thin wrapper around a server socket with some utility logic to make setting
// up tests easier. To write a test, you start a server, construct a sequence of
// handling rules, and write a test function.
//
// Handling rules let you set up expectations of future packets of certain
// types. Handling rules are processed in order, and only the first remaining
// handler handles a given packet. In theory you could write the entire test
// into a single handling rule and keep an internal state machine for how far
// that handler has gotten through the test. This would be poor style however.
// Correct style is to write (or reuse) a handler for each packet the server
// should see, leading us to a happy land where any conceivable packet handler
// has already been written for us.
type dhcpTestServer struct {
	iface     string
	inAddr    net.IP
	inPort    int
	bcastAddr net.IP
	bcastPort int

	conn *net.UDPConn
}

type testFunction func(context.Context) error

func newDHCPTestServer(iface string, inAddr, bcastAddr net.IP, inPort, bcastPort int) *dhcpTestServer {
	return &dhcpTestServer{
		iface:     iface,
		inAddr:    inAddr,
		inPort:    inPort,
		bcastAddr: bcastAddr,
		bcastPort: bcastPort,
	}
}

// setupAndBindSocket creates, sets the appropriate socket options for, and
// binds to the server socket.
func (s *dhcpTestServer) setupAndBindSocket(ctx context.Context) error {
	lc := net.ListenConfig{Control: func(network, address string, c syscall.RawConn) error {
		var err error
		if cerr := c.Control(func(fd uintptr) {
			if err = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_REUSEADDR, 1); err != nil {
				return
			}
			if err = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_BROADCAST, 1); err != nil {
				return
			}
			if len(s.iface) > 0 {
				if err = unix.SetsockoptString(int(fd), unix.SOL_SOCKET, unix.SO_BINDTODEVICE, s.iface); err != nil {
					return
				}
			}
		}); cerr != nil {
			return cerr
		}
		return err
	}}
	conn, err := lc.ListenPacket(ctx, "udp", fmt.Sprintf("%s:%d", s.inAddr.String(), s.inPort))
	if err != nil {
		conn.Close()
		return errors.Wrapf(err, "unable to listen on %s:%d", s.inAddr.String(), s.inPort)
	}
	udpconn, ok := conn.(*net.UDPConn)
	if !ok {
		conn.Close()
		return errors.New("incorrect socket type, expected UDP")
	}
	s.conn = udpconn
	return nil
}

func (s *dhcpTestServer) sendResponse(packet *dhcpPacket) error {
	if packet == nil {
		return errors.New("handling rule failed to return a packet")
	}
	binaryStr, err := packet.marshal()
	if err != nil {
		return errors.Wrap(err, "packet failed to serialize to binary string")
	}
	if err = s.conn.SetWriteDeadline(time.Now().Add(100 * time.Millisecond)); err != nil {
		return errors.Wrap(err, "unable to set deadline")
	}
	_, err = s.conn.WriteToUDP([]byte(binaryStr), &net.UDPAddr{IP: s.bcastAddr, Port: s.bcastPort})
	return err
}

// runLoop is the loop body of the test server. It receives and handles DHCP
// packets coming from the client and responds to them according to the given
// handling rules.
func (s *dhcpTestServer) runLoop(ctx context.Context, rules []dhcpHandlingRule) error {
	buffer := make([]byte, 2048)
	for {
		if len(rules) < 1 {
			return errors.New("no handling rules left")
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := s.conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond)); err != nil {
			return errors.Wrap(err, "unable to set deadline")
		}
		n, _, err := s.conn.ReadFromUDP(buffer)
		if opErr, ok := err.(*net.OpError); ok && opErr.Timeout() {
			continue
		} else if err != nil {
			return errors.Wrap(err, "read failed")
		} else if n == 0 {
			testing.ContextLog(ctx, "Read returned 0 bytes")
			continue
		}
		packet, err := newDHCPPacket(buffer[:n])
		if err != nil {
			testing.ContextLog(ctx, "Unable to create DHCP packet: ", err)
			continue
		}
		if err = packet.isValid(); err != nil {
			testing.ContextLog(ctx, "Invalid DHCP packet: ", err)
			continue
		}

		rule := rules[0]
		code := rule.handle(packet)
		if code&popHandler != 0 {
			rules = rules[1:]
		}

		if code&haveResponse != 0 {
			for i := 0; i < rule.respPktCnt; i++ {
				response, err := rule.respond(packet)
				if err != nil {
					return errors.Wrap(err, "failed to generate response")
				}
				if err = s.sendResponse(response); err != nil {
					return errors.Wrap(err, "failed to send packet")
				}
			}
		}

		if code&testFailed > 0 {
			return errors.New("handling rule rejected packet")
		}

		if code&testSucceeded > 0 {
			return nil
		}
	}
}

// runTest runs testFunc against a server with the given handling rules.
func (s *dhcpTestServer) runTest(ctx context.Context, rules []dhcpHandlingRule, testFunc testFunction) error {
	if err := s.setupAndBindSocket(ctx); err != nil {
		return err
	}
	defer s.conn.Close()
	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		return s.runLoop(ctx, rules)
	})
	g.Go(func() error {
		return testFunc(ctx)
	})
	return g.Wait()
}
