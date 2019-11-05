// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package network provides general CrOS network goodies.
package network

import (
	"net"
	"sync"
	"syscall"
	"time"

	"chromiumos/tast/errors"
)

type DHCPTestServer struct {
	iface            string
	ingressAddress   net.IP
	ingressPort      int
	broadcastAddress net.IP
	broadcastPort    int
	socket           int
	stopped          bool
	testInProgress   bool
	lastTestPassed   bool
	alive            bool
	handlingRules    []DHCPHandlingRuleInterface
	mutex            sync.Mutex
	testTimeout      time.Time
	serverErrorChan  chan error
	testErrorChan    chan error
}

func NewDHCPTestServer(iface string, ingressAddress net.IP, ingressPort int, broadcastAddress net.IP, broadcastPort int) *DHCPTestServer {
	server := DHCPTestServer{iface: iface, ingressAddress: ingressAddress, ingressPort: ingressPort, broadcastAddress: broadcastAddress, broadcastPort: broadcastPort}
	return &server
}

func (server *DHCPTestServer) AtomicSetAlive(isAlive bool) {
	server.mutex.Lock()
	defer server.mutex.Unlock()
	server.alive = isAlive
}

func (server *DHCPTestServer) AtomicStopped() bool {
	server.mutex.Lock()
	defer server.mutex.Unlock()
	return server.stopped
}

func (server *DHCPTestServer) AtomicIsHealthy() bool {
	server.mutex.Lock()
	defer server.mutex.Unlock()
	return server.socket > 0
}

func (server *DHCPTestServer) AtomicTestInProgress() bool {
	server.mutex.Lock()
	defer server.mutex.Unlock()
	return server.testInProgress
}

func (server *DHCPTestServer) AtomicLastTestPassed() bool {
	server.mutex.Lock()
	defer server.mutex.Unlock()
	return server.lastTestPassed
}

func (server *DHCPTestServer) AtomicCurrentRule() DHCPHandlingRuleInterface {
	server.mutex.Lock()
	defer server.mutex.Unlock()
	return server.handlingRules[0]
}

func (server *DHCPTestServer) StartServer() error {
	if server.alive {
		return errors.New("server already started")
	}

	socket, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_DGRAM, syscall.IPPROTO_UDP)
	if err != nil {
		return err
	}
	server.socket = socket
	if err := syscall.SetsockoptInt(server.socket, syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1); err != nil {
		return err
	}
	if err := syscall.SetsockoptInt(server.socket, syscall.SOL_SOCKET, syscall.SO_BROADCAST, 1); err != nil {
		return err
	}
	timeout := syscall.Timeval{0, 1000}
	if err := syscall.SetsockoptTimeval(server.socket, syscall.SOL_SOCKET, syscall.SO_RCVTIMEO, &timeout); err != nil {
		return err
	}
	if err := syscall.SetsockoptTimeval(server.socket, syscall.SOL_SOCKET, syscall.SO_SNDTIMEO, &timeout); err != nil {
		return err
	}
	if len(server.iface) > 0 {
		if err := syscall.SetsockoptString(server.socket, syscall.SOL_SOCKET, syscall.SO_BINDTODEVICE, server.iface); err != nil {
			return err
		}
	}

	addr := syscall.SockaddrInet4{Port: server.ingressPort}
	copy(addr.Addr[:], server.ingressAddress.To4())
	if err := syscall.Bind(server.socket, &addr); err != nil {
		return err
	}
	server.run()
	return nil
}

func (server *DHCPTestServer) Stop() {
	server.mutex.Lock()
	defer server.mutex.Unlock()
	server.stopped = true
	server.serverErrorChan = nil
}

func (server *DHCPTestServer) StartTest(handlingRules []DHCPHandlingRuleInterface, testTimeout time.Duration) {
	server.mutex.Lock()
	defer server.mutex.Unlock()
	server.testTimeout = time.Now().Add(testTimeout)
	server.handlingRules = handlingRules
	server.testInProgress = true
	server.testErrorChan = make(chan error, 1)
	server.lastTestPassed = false
}

func (server *DHCPTestServer) WaitForTestToFinish() error {
	var ret error
	for {
		select {
		case err, more := <-server.testErrorChan:
			if !more {
				return ret
			} else {
				ret = err
			}
		}
	}
	return ret
}

func (server *DHCPTestServer) AbortTest() {
	server.mutex.Lock()
	defer server.mutex.Unlock()
	server.endTestUnsafe(false)
}

func (server *DHCPTestServer) teardown() {
	server.mutex.Lock()
	defer server.mutex.Unlock()
	_ = syscall.Close(server.socket)
	server.socket = 0
}

func (server *DHCPTestServer) endTestUnsafe(passed bool) {
	if !server.testInProgress {
		return
	}
	server.testInProgress = false
	close(server.testErrorChan)
	server.lastTestPassed = passed
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

func (server *DHCPTestServer) loopBody() {
	server.mutex.Lock()
	defer server.mutex.Unlock()
	if server.testInProgress && server.testTimeout.Before(time.Now()) {
		server.testErrorChan <- errors.New("test in progress has timed out")
		server.endTestUnsafe(false)
	}
	buffer := make([]byte, 1024)
	n, _, err := syscall.Recvfrom(server.socket, buffer, 0)
	if err == syscall.EAGAIN {
		return
	} else if err != nil {
		if server.testInProgress {
			server.testErrorChan <- errors.Wrap(err, "recvfrom failed")
			server.endTestUnsafe(false)
		}
		return
	}
	if !server.testInProgress {
		return
	}
	packet, err := newDHCPPacket(buffer[:n])
	if err != nil {
		return
	}
	if !packet.isValid() {
		return
	}

	if len(server.handlingRules) < 1 {
		server.testErrorChan <- errors.New("no handling rule for packet")
		server.endTestUnsafe(false)
		return
	}

	handlingRule := server.handlingRules[0]
	responseCode := handlingRule.handle(packet)
	if responseCode&responsePopHandler > 0 {
		server.handlingRules = server.handlingRules[1:]
	}

	if responseCode&responseHaveResponse > 0 {
		for responseInstance := 0; responseInstance < handlingRule.responsePacketCount(); responseInstance++ {
			response, _ := handlingRule.respond(packet)
			if err := server.sendResponseUnsafe(response); err != nil {
				server.testErrorChan <- errors.Wrap(err, "failed to send packet")
				server.endTestUnsafe(false)
				return
			}
		}
	}

	if responseCode&responseTestFailed > 0 {
		server.testErrorChan <- errors.New("handling rule rejected packet")
		server.endTestUnsafe(false)
		return
	}

	if responseCode&responseTestSucceeded > 0 {
		server.endTestUnsafe(true)
		return
	}
}

func (server *DHCPTestServer) run() {
	server.serverErrorChan = make(chan error, 1)
	go func() {
		server.AtomicSetAlive(true)
		defer close(server.serverErrorChan)
		defer server.AtomicSetAlive(false)
		if !server.AtomicIsHealthy() {
			server.serverErrorChan <- errors.New("failed to create server socket, exiting")
			return
		}

		for !server.stopped {
			server.loopBody()
			time.Sleep(10 * time.Millisecond)
		}

		server.mutex.Lock()
		server.endTestUnsafe(false)
		server.mutex.Unlock()
		server.teardown()
	}()
}
