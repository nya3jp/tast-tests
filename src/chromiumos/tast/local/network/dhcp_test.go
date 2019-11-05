// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package network provides general CrOS network goodies.
package network

import (
	"context"
	"io/ioutil"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"

	"chromiumos/tast/errors"
)

const (
	dataPathPrefix             = "dhcp_test_data/"
	classlessStaticRouteData   = "\x12\x0a\x09\xc0\xac\x1f\x9b\x0a\x00\xc0\xa8\x00\xfe"
	domainSearchListCompressed = "\x03eng\x06google\x03com\x00\x09marketing\xC0\x04"
	domainSearchListExpected   = "\x03eng\x06google\x03com\x00\x09marketing\x06google\x03com\x00"
	domainSearchList1          = "w\x10\x03eng\x06google\x03com\x00"
	domainSearchList2          = "w\x16\x09marketing\x06google\x03com\x00"
)

var (
	classlessStaticRouteListParsed = []staticRoute{
		staticRoute{uint8(18), "10.9.192.0", "172.31.155.10"},
		staticRoute{uint8(0), "0.0.0.0", "192.168.0.254"},
	}

	domainSearchListParsed = []string{
		"eng.google.com",
		"marketing.google.com",
	}
)

func TestPacketSerialization(t *testing.T) {
	data, err := ioutil.ReadFile(dataPathPrefix + "dhcp_discovery.log")
	if err != nil {
		t.Errorf("unable to read log file: %v", err)
	}
	discoveryPacket, err := newDHCPPacket(data)
	if err != nil {
		t.Errorf("Unable to create DHCP packet: %v", err)
		return
	}
	if !discoveryPacket.isValid() {
		t.Error("Invalid DHCP Packet")
		return
	}
	generatedBytes, err := discoveryPacket.toBinary()
	if err != nil {
		t.Errorf("Failed to generate string from packet object: %v", err)
		return
	}
	if string(generatedBytes) != string(data) {
		t.Errorf("Packets didn't match: \n"+
			"Generated: \n%x\n"+
			"Expected: \n%x\n", generatedBytes, data)
		return
	}
}

func TestClasslessStaticRouteParsing(t *testing.T) {
	var opt classlessStaticRoutesOption
	parsedRoutes, err := opt.unpack([]byte(classlessStaticRouteData))
	if err != nil {
		t.Errorf("Failed to unpack test data: %v", err)
		return
	}
	if !reflect.DeepEqual(parsedRoutes, classlessStaticRouteListParsed) {
		t.Errorf("Parsed binary domain list and got %v but expected %v", parsedRoutes, classlessStaticRouteListParsed)
		return
	}
}

func TestClasslessStaticRouteSerialization(t *testing.T) {
	var opt classlessStaticRoutesOption
	bytes, err := opt.pack(classlessStaticRouteListParsed)
	if err != nil {
		t.Errorf("Failed to pack test data: %v", err)
		return
	}
	if string(bytes) != classlessStaticRouteData {
		t.Errorf("Expected to serialize %v to %x but instead got %x.", classlessStaticRouteListParsed, classlessStaticRouteData, bytes)
		return
	}
}

func TestDomainSearchListParsing(t *testing.T) {
	var opt domainListOption
	parsedDomains, err := opt.unpack([]byte(domainSearchListCompressed))
	if err != nil {
		t.Errorf("Failed to unpack test data: %v", err)
		return
	}
	if !reflect.DeepEqual(parsedDomains, domainSearchListParsed) {
		t.Errorf("Parsed binary domain list and got %v but expected %v", parsedDomains, domainSearchListExpected)
		return
	}
}

func TestDomainSearchListSerialization(t *testing.T) {
	var opt domainListOption
	bytes, err := opt.pack(domainSearchListParsed)
	if err != nil {
		t.Errorf("Failed to pack test data: %v", err)
		return
	}
	if string(bytes) != domainSearchListExpected {
		t.Errorf("Expected to serialize %v to %x but instead got %x.", domainSearchListParsed, domainSearchListExpected, bytes)
		return
	}
}

func TestBrokenDomainSearchListParsing(t *testing.T) {
	byteStr := strings.Repeat("\x00", 240) + domainSearchList1 + domainSearchList2 + "\xff"
	packet, err := newDHCPPacket([]byte(byteStr))
	if err != nil {

	}
	if len(packet.options) != 1 {
		t.Errorf("Expected domain list of length 1")
		return
	}
	for _, v := range packet.options {
		if !reflect.DeepEqual(v, domainSearchListParsed) {
			t.Errorf("Expected binary domain list and got %v but expected %v", v, domainSearchListParsed)
			return
		}
	}
}

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

func simpleServerExchange(server *dhcpTestServer) error {
	intendedIP := "127.0.0.42"
	subnet := "255.255.255.0"
	serverIP := "127.0.0.1"
	serverIPParsed := [4]byte{127, 0, 0, 1}
	leaseTimeSeconds := uint32(60)
	testTimeout := 3 * time.Second
	macAddr := "\x01\x02\x03\x04\x05\x06"

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
	DHCPServerConfig := optionMap{
		serverID:    serverIP,
		subnetMask:  subnet,
		ipLeaseTime: leaseTimeSeconds,
		requestedIP: intendedIP,
	}
	rule1 := newRespondToDiscovery(intendedIP, serverIP, DHCPServerConfig, fieldMap{}, true)
	rule2 := newRespondToRequest(intendedIP, serverIP, DHCPServerConfig, fieldMap{}, true, "", "", true)
	rule2.IsFinalHandler = true
	rules := []dhcpHandlingRule{*rule1, *rule2}

	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, testTimeout)
	defer cancel()
	testFunc := func() error {
		clientSocket, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_DGRAM, syscall.IPPROTO_UDP)
		if err != nil {
			return err
		}
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
			return errors.New("type of DHCP repsonse is not offer")
		}
		offerIP := offerPacket.getField(yourIP)
		offerIPStr, ok := offerIP.(string)
		if !ok {
			return errors.New("offered IP is not string type")
		}
		if offerIPStr != intendedIP {
			return errors.New("server didn't give us the IP we expected")
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
		ackPacket, err := receivePacket(ctx, clientSocket, 1*time.Second)
		if err != nil {
			return err
		}
		ackType, err := ackPacket.msgType()
		if err != nil {
			return err
		}
		if ackType != ack {
			return errors.New("type of DHCP response is not acknowledgment")
		}
		ackIP := ackPacket.getField(yourIP)
		ackIPStr, ok := ackIP.(string)
		if !ok {
			return errors.New("given IP is not string type")
		}
		if ackIPStr != intendedIP {
			return errors.New("server didn't give us the IP we expected")
		}
		return nil
	}
	return server.runTest(ctx, rules, testFunc)
}

func TestServerDialogue(t *testing.T) {
	ip := []byte{127, 0, 0, 1}
	server := newDHCPTestServer("", ip, ip, 8067, 8068)
	if server == nil {
		t.Errorf("failed to start DHCP test server")
		return
	}
	if err := simpleServerExchange(server); err != nil {
		t.Errorf("test failed with error: %v", err)
	}
}
