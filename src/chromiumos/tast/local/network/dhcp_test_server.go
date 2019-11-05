// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package network provides general CrOS network goodies.
package network

import (
	"context"
	"net"
	"golang.org/x/sync/errgroup"
	"syscall"

	"chromiumos/tast/errors"
)

type DHCPTestServer struct {
	iface            string
	ingressAddress   net.IP
	ingressPort      int
	broadcastAddress net.IP
	broadcastPort    int
	socket           int
}

type testFunction func() error

func NewDHCPTestServer(iface string, ingressAddress net.IP, ingressPort int, broadcastAddress net.IP, broadcastPort int) *DHCPTestServer {
	return &DHCPTestServer{iface: iface, ingressAddress: ingressAddress, ingressPort: ingressPort, broadcastAddress: broadcastAddress, broadcastPort: broadcastPort}
}

func (s *DHCPTestServer) SetupAndBindSocket() error {
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
	timeout := syscall.Timeval{0, 1000}
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

	addr := syscall.SockaddrInet4{Port: s.ingressPort}
	copy(addr.Addr[:], s.ingressAddress.To4())
	return syscall.Bind(s.socket, &addr)
}

func (server *DHCPTestServer) sendResponseUnsafe(packet *DHCPPacket) error {
	if packet == nil {
		return errors.New("handling rule failed to return a packet")
	}
	binaryStr, err := packet.toBinaryString()
	if err != nil {
		return errors.Wrap(err, "packet failed to serialize to binary string")
	}

	addr := syscall.SockaddrInet4{Port: server.broadcastPort}
	copy(addr.Addr[:], server.broadcastAddress.To4())
	err = syscall.Sendto(server.socket, []byte(binaryStr), 0, &addr)
	if err != nil {
		return errors.Wrap(err, "sendto failed")
	}
	return nil
}

func (server *DHCPTestServer) runLoop(rules []DHCPHandlingRule) error {
	for {
		buffer := make([]byte, 1024)
		n, _, err := syscall.Recvfrom(server.socket, buffer, 0)
		if err == syscall.EAGAIN {
			continue
		} else if err != nil {
			return errors.Wrap(err, "recvfrom failed")
		}
		packet, err := newDHCPPacket(buffer[:n])
		if err != nil {
			continue
		}
		if !packet.isValid() {
			continue
		}

		if len(rules) < 1 {
			return errors.New("no handling rule for packet")
		}

		handlingRule := rules[0]
		responseCode := handlingRule.handle(packet)
		if responseCode&responsePopHandler > 0 {
			rules = rules[1:]
		}

		if responseCode&responseHaveResponse > 0 {
			for responseInstance := 0; responseInstance < handlingRule.responsePacketCount; responseInstance++ {
				response, _ := handlingRule.respond(packet)
				if err := server.sendResponseUnsafe(response); err != nil {
					return errors.Wrap(err, "failed to send packet")
				}
			}
		}

		if responseCode&responseTestFailed > 0 {
			return errors.New("handling rule rejected packet")
		}

		if responseCode&responseTestSucceeded > 0 {
			return nil
		}
	}
	return errors.New("Not reached")
}

func (s *DHCPTestServer) RunTest(ctx context.Context, rules []DHCPHandlingRule, testFunc testFunction) error {
	if err := s.SetupAndBindSocket(); err != nil {
		return err
	}
	defer syscall.Close(s.socket)
	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		return s.runLoop(rules)
	})
	g.Go(testFunc)
	return g.Wait()
}
