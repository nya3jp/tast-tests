// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package network provides general CrOS network goodies.
package network

import (
	"context"
	"net"
	"reflect"
	"testing"
	"time"

	"chromiumos/tast/errors"
)

// receivePacket receives a packet on the given socket and returns a dhcpPacket
// object.
func receivePacket(ctx context.Context, conn *net.UDPConn, timeout time.Duration) (*dhcpPacket, error) {
	var data []byte
	buffer := make([]byte, 2048)
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	for len(data) == 0 {
		select {
		case <-ctx.Done():
			return nil, errors.New("timed out before we received a response from the server")
		default:
			n, _, err := conn.ReadFromUDP(buffer)
			opErr, ok := err.(*net.OpError)
			if ok && opErr.Timeout() {
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
	if err = packet.isValid(); err != nil {
		return nil, errors.Wrap(err, "received an invalid response from DHCP server")
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
	)
	macAddr := []byte{1, 2, 3, 4, 5, 6}
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
	rule2.isFinalHandler = true
	rules := []dhcpHandlingRule{*rule1, *rule2}

	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, testTimeout)
	defer cancel()
	testFunc := func(context.Context) error {
		conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IP(serverIPParsed[:]), Port: 8068})
		if err = conn.SetDeadline(time.Now().Add(100 * time.Millisecond)); err != nil {
			return err
		}

		discoveryMessageStr, err := discoveryMessage.marshal()
		if err != nil {
			return err
		}
		conn.WriteToUDP([]byte(discoveryMessageStr), &net.UDPAddr{IP: net.IP(serverIPParsed[:]), Port: 8067})
		offerPacket, err := receivePacket(ctx, conn, 1*time.Second)
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
		offerIP := offerPacket.field(yourIP)
		offerIPStr, ok := offerIP.(string)
		if !ok {
			return errors.Errorf("offered IP is type %v, expected type string", reflect.TypeOf(offerIP))
		}
		if offerIPStr != intendedIP {
			return errors.Errorf("server offered IP %s, expected %s", offerIPStr, intendedIP)
		}

		requestMessage.setOption(serverID, offerPacket.option(serverID))
		requestMessage.setOption(subnetMask, offerPacket.option(subnetMask))
		requestMessage.setOption(ipLeaseTime, offerPacket.option(ipLeaseTime))
		requestMessage.setOption(requestedIP, offerPacket.option(requestedIP))
		requestMessageStr, err := requestMessage.marshal()
		if err != nil {
			return err
		}
		conn.WriteToUDP([]byte(requestMessageStr), &net.UDPAddr{IP: net.IP(serverIPParsed[:]), Port: 8067})
		ackPacket, err := receivePacket(ctx, conn, time.Second)
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
		ackIP := ackPacket.field(yourIP)
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
