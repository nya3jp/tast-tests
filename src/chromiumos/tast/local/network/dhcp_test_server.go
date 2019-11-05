// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package network provides general CrOS network goodies.
package network

import (
	"context"
	"net"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"

	"chromiumos/tast/errors"
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
	socket    int
}

var timeout = syscall.Timeval{0, 100000}

type testFunction func() error

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
func (s *dhcpTestServer) setupAndBindSocket() error {
	socket, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_DGRAM, syscall.IPPROTO_UDP)
	if err != nil {
		return nil
	}
	s.socket = socket
	if err := syscall.SetsockoptInt(s.socket, syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1); err != nil {
		return nil
	}
	if err := syscall.SetsockoptInt(s.socket, syscall.SOL_SOCKET, syscall.SO_BROADCAST, 1); err != nil {
		return nil
	}
	if err := syscall.SetsockoptTimeval(s.socket, syscall.SOL_SOCKET, syscall.SO_RCVTIMEO, &timeout); err != nil {
		return nil
	}
	if err := syscall.SetsockoptTimeval(s.socket, syscall.SOL_SOCKET, syscall.SO_SNDTIMEO, &timeout); err != nil {
		return nil
	}
	if len(s.iface) > 0 {
		if err := syscall.SetsockoptString(s.socket, syscall.SOL_SOCKET, syscall.SO_BINDTODEVICE, s.iface); err != nil {
			return nil
		}
	}

	addr := syscall.SockaddrInet4{Port: s.inPort}
	copy(addr.Addr[:], s.inAddr.To4())
	return syscall.Bind(s.socket, &addr)
}

func (s *dhcpTestServer) sendResponse(packet *dhcpPacket) error {
	if packet == nil {
		return errors.New("handling rule failed to return a packet")
	}
	binaryStr, err := packet.toBinary()
	if err != nil {
		return errors.Wrap(err, "packet failed to serialize to binary string")
	}

	addr := syscall.SockaddrInet4{Port: s.bcastPort}
	copy(addr.Addr[:], s.bcastAddr.To4())
	if err = syscall.Sendto(s.socket, []byte(binaryStr), 0, &addr); err != nil {
		return errors.Wrap(err, "sendto failed")
	}
	return nil
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
		select {
		case <-time.After(10 * time.Millisecond):
			n, _, err := syscall.Recvfrom(s.socket, buffer, 0)
			if err == syscall.EAGAIN {
				continue
			} else if err != nil {
				return errors.Wrap(err, "recvfrom failed")
			} else if n == 0 {
				return errors.New("recvfrom returned 0 bytes")
			}
			packet, err := newDHCPPacket(buffer[:n])
			if err != nil {
				continue
			}
			if !packet.isValid() {
				continue
			}

			rule := rules[0]
			code := rule.handle(packet)
			if code&popHandler != 0 {
				rules = rules[1:]
			}

			if code&haveResponse != 0 {
				for instance := 0; instance < rule.respPktCnt; instance++ {
					response, err := rule.respond(packet)
					if err != nil {
						errors.Wrap(err, "failed to generate response")
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
		case <-ctx.Done():
			return errors.New("timeout reached")
		}
	}
}

// runTest runs |testFunc| against a server with the given handling rules.
func (s *dhcpTestServer) runTest(ctx context.Context, rules []dhcpHandlingRule, testFunc testFunction) error {
	if err := s.setupAndBindSocket(); err != nil {
		return err
	}
	defer syscall.Close(s.socket)
	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		return s.runLoop(ctx, rules)
	})
	g.Go(testFunc)
	return g.Wait()
}
