// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package network provides general CrOS network goodies.
package network

import (
	"context"
	"reflect"
	"syscall"
	"testing"
	"time"

	"chromiumos/tast/errors"
)

// receivePacket receives a packet on the given socket and returns a dhcpPacket
// object.
func receivePacket(ctx context.Context, socket int, timeout time.Duration) (*dhcpPacket, error) {
	var data []byte
	buffer := make([]byte, 2048)
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	for len(data) == 0 {
		select {
		case <-ctx.Done():
			return nil, errors.New("timed out before we received a response from the server")
		default:
			n, _, err := syscall.Recvfrom(socket, buffer, 0)
			if err == syscall.EAGAIN {
				continue
			} else if err != nil {
				return nil, err
			}
			data = buffer[:n]
		}
	}

	packet, err := newDHCPPacket(data)
	if err != nil {
		return nil, err
	}
	if !packet.isValid() {
		return nil, errors.New("received an invalid response from DHCP server")
	}

	return packet, nil
}

// simpleServerExchange runs a simple DHCP exchange on the given server. The
// server will be setup to respond to one discovery packet and then one request
// packet.
func simpleServerExchange(server *dhcpTestServer) error {
	const (
		intendedIP       = "127.0.0.42"
		subnet           = "255.255.255.0"
		serverIP         = "127.0.0.1"
		leaseTimeSeconds = uint32(60)
		testTimeout      = 3 * time.Second
		macAddr          = "\x01\x02\x03\x04\x05\x06"
	)
	serverIPParsed := [4]byte{127, 0, 0, 1}

	discoveryMessage, err := createDiscovery(macAddr)
	if err != nil {
		return err
	}
	discoveryMessage.setOption(parameterRequestList, defaultParameterRequestList)
	txnID, err := discoveryMessage.txnID()
	if err != nil {
		return err
	}
	requestMessage, err := createRequest(txnID, macAddr)
	if err != nil {
		return err
	}
	requestMessage.setOption(parameterRequestList, defaultParameterRequestList)
	dhcpServerConfig := optionMap{
		serverID:    serverIP,
		subnetMask:  subnet,
		ipLeaseTime: leaseTimeSeconds,
		requestedIP: intendedIP,
	}
	rule1 := newRespondToDiscovery(intendedIP, serverIP, dhcpServerConfig, fieldMap{}, true)
	rule2 := newRespondToRequest(intendedIP, serverIP, dhcpServerConfig, fieldMap{}, true, "", "", true)
	rule2.IsFinalHandler = true
	rules := []dhcpHandlingRule{*rule1, *rule2}

	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, testTimeout)
	defer cancel()
	testFunc := func(context.Context) error {
		clientSocket, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_DGRAM, syscall.IPPROTO_UDP)
		if err != nil {
			return err
		}
		// timeout is 100000usec == 100ms
		timeout := syscall.Timeval{0, 100000}
		if err = syscall.SetsockoptTimeval(clientSocket, syscall.SOL_SOCKET, syscall.SO_RCVTIMEO, &timeout); err != nil {
			return err
		}
		if err = syscall.SetsockoptTimeval(clientSocket, syscall.SOL_SOCKET, syscall.SO_SNDTIMEO, &timeout); err != nil {
			return err
		}
		if err = syscall.Bind(clientSocket, &syscall.SockaddrInet4{Addr: serverIPParsed, Port: 8068}); err != nil {
			return err
		}

		discoveryMessageStr, err := discoveryMessage.toBinary()
		if err != nil {
			return err
		}
		syscall.Sendto(clientSocket, []byte(discoveryMessageStr), 0, &syscall.SockaddrInet4{Addr: serverIPParsed, Port: 8067})
		offerPacket, err := receivePacket(ctx, clientSocket, 1*time.Second)
		if err != nil {
			return err
		}
		offerType, err := offerPacket.msgType()
		if err != nil {
			return err
		}
		if offerType != offer {
			return errors.Errorf("type of DHCP response is %v, expected %v", offerType, offer)
		}
		offerIP := offerPacket.getField(yourIP)
		offerIPStr, ok := offerIP.(string)
		if !ok {
			return errors.Errorf("offered IP is type %v, expected type string", reflect.TypeOf(offerIP))
		}
		if offerIPStr != intendedIP {
			return errors.Errorf("server offered IP %s, expected %s", offerIPStr, intendedIP)
		}

		requestMessage.setOption(serverID, offerPacket.getOption(serverID))
		requestMessage.setOption(subnetMask, offerPacket.getOption(subnetMask))
		requestMessage.setOption(ipLeaseTime, offerPacket.getOption(ipLeaseTime))
		requestMessage.setOption(requestedIP, offerPacket.getOption(requestedIP))
		requestMessageStr, err := requestMessage.toBinary()
		if err != nil {
			return err
		}
		syscall.Sendto(clientSocket, []byte(requestMessageStr), 0, &syscall.SockaddrInet4{Addr: serverIPParsed, Port: 8067})
		ackPacket, err := receivePacket(ctx, clientSocket, time.Second)
		if err != nil {
			return err
		}
		ackType, err := ackPacket.msgType()
		if err != nil {
			return err
		}
		if ackType != ack {
			return errors.Errorf("type of DHCP response is %v, expected %v", ackType, ack)
		}
		ackIP := ackPacket.getField(yourIP)
		ackIPStr, ok := ackIP.(string)
		if !ok {
			return errors.Errorf("given IP is type %v, expected type string", reflect.TypeOf(ackIP))
		}
		if ackIPStr != intendedIP {
			return errors.Errorf("server ack'ed IP %s, expected %s", ackIPStr, intendedIP)
		}
		return nil
	}
	return server.runTest(ctx, rules, testFunc)
}

func TestServerDialogue(t *testing.T) {
	ip := []byte{127, 0, 0, 1}
	server := newDHCPTestServer("", ip, ip, 8067, 8068)
	if server == nil {
		t.Fatalf("failed to start DHCP test server")
	}
	if err := simpleServerExchange(server); err != nil {
		t.Fatalf("test failed with error: %v", err)
	}
}
