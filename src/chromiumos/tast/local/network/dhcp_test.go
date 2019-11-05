// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package network provides general CrOS network goodies.
package network

import (
	"io/ioutil"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"

	"chromiumos/tast/errors"
)

const (
	testDataPathPrefix           = "dhcp_test_data/"
	testClasslessStaticRouteData = "\x12\x0a\x09\xc0\xac\x1f\x9b\x0a\x00\xc0\xa8\x00\xfe"

	testDomainSearchListCompressed = "\x03eng\x06google\x03com\x00\x09marketing\xC0\x04"

	testDomainSearchListExpected = "\x03eng\x06google\x03com\x00\x09marketing\x06google\x03com\x00"

	testDomainSearchList1 = "w\x10\x03eng\x06google\x03com\x00"

	testDomainSearchList2 = "w\x16\x09marketing\x06google\x03com\x00"
)

var (
	testClasslessStaticRouteListParsed = []staticRoute{
		staticRoute{uint8(18), "10.9.192.0", "172.31.155.10"},
		staticRoute{uint8(0), "0.0.0.0", "192.168.0.254"},
	}

	testDomainSearchListParsed = []string{
		"eng.google.com",
		"marketing.google.com",
	}
)

func TestPacketSerialization(t *testing.T) {
	data, err := ioutil.ReadFile(testDataPathPrefix + "dhcp_discovery.log")
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
	generatedBytes, err := discoveryPacket.toBinaryString()
	if err != nil {
		t.Error("Failed to generate string from packet object.")
		return
	}
	if string(generatedBytes) != string(data) {
		t.Errorf("Packets didn't match: \n"+
			"Generated: \n%x\n"+
			"Expected: \n%x\n", generatedBytes, data)
		return
	}
	t.Log("TestPacketSerialization PASSED")
}

func TestClasslessStaticRouteParsing(t *testing.T) {
	var opt classlessStaticRoutesOption
	parsedRoutes, err := opt.unpack([]byte(testClasslessStaticRouteData))
	if err != nil {
		t.Errorf("Failed to unpack test data: %v", err)
		return
	}
	if !reflect.DeepEqual(parsedRoutes, testClasslessStaticRouteListParsed) {
		t.Errorf("Parsed binary domain list and got %v but expected %v", parsedRoutes, testClasslessStaticRouteListParsed)
		return
	}
	t.Log("TestClasslessStaticRouteParsing PASSED")
}

func TestClasslessStaticRouteSerialization(t *testing.T) {
	var opt classlessStaticRoutesOption
	bytes, err := opt.pack(testClasslessStaticRouteListParsed)
	if err != nil {
		t.Errorf("Failed to pack test data: %v", err)
		return
	}
	if string(bytes) != testClasslessStaticRouteData {
		t.Errorf("Expected to serialize %v to %x but instead got %x.", testClasslessStaticRouteListParsed, testClasslessStaticRouteData, bytes)
		return
	}
	t.Log("TestClasslessStaticRouteSerialization PASSED")
}

func TestDomainSearchListParsing(t *testing.T) {
	var opt domainListOption
	parsedDomains, err := opt.unpack([]byte(testDomainSearchListCompressed))
	if err != nil {
		t.Errorf("Failed to unpack test data: %v", err)
		return
	}
	if !reflect.DeepEqual(parsedDomains, testDomainSearchListParsed) {
		t.Errorf("Parsed binary domain list and got %v but expected %v", parsedDomains, testDomainSearchListExpected)
		return
	}
	t.Log("TestDomainSearchListParsing PASSED")
}

func TestDomainSearchListSerialization(t *testing.T) {
	var opt domainListOption
	bytes, err := opt.pack(testDomainSearchListParsed)
	if err != nil {
		t.Errorf("Failed to pack test data: %v", err)
		return
	}
	if string(bytes) != testDomainSearchListExpected {
		t.Errorf("Expected to serialize %v to %x but instead got %x.", testDomainSearchListParsed, testDomainSearchListExpected, bytes)
		return
	}
	t.Log("TestDomainSearchListSerialization PASSED")
}

func TestBrokenDomainSearchListParsing(t *testing.T) {
	byteStr := strings.Repeat("\x00", 240) + testDomainSearchList1 + testDomainSearchList2 + "\xff"
	packet, err := newDHCPPacket([]byte(byteStr))
	if err != nil {

	}
	if len(packet.options) != 1 {
		t.Errorf("Expected domain list of length 1")
		return
	}
	for _, v := range packet.options {
		if !reflect.DeepEqual(v, testDomainSearchListParsed) {
			t.Errorf("Expected binary domain list and got %v but expected %v", v, testDomainSearchListParsed)
			return
		}
	}
	t.Log("TestBrokenDomainSearchListParsing PASSED")
}

func receivePacket(socket int, timeout time.Duration) (*DHCPPacket, error) {
	var data []byte
	startTime := time.Now()
	for len(data) == 0 && startTime.Add(timeout).After(time.Now()) {
		buffer := make([]byte, 1024)
		n, _, err := syscall.Recvfrom(socket, buffer, 0)
		if err == syscall.EAGAIN {
			continue
		} else if err != nil {
			return nil, err
		}
		data = buffer[:n]
	}
	if len(data) == 0 {
		return nil, errors.New("timed out before we received a response from the server.")
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

func simpleServerExchange(server *DHCPTestServer) error {
	intendedIP := "127.0.0.42"
	subnetMask := "255.255.255.0"
	serverIP := "127.0.0.1"
	serverIPParsed := [4]byte{127, 0, 0, 1}
	leaseTimeSeconds := uint32(60)
	testTimeout := 3 * time.Second
	macAddr := "\x01\x02\x03\x04\x05\x06"

	discoveryMessage, err := createDiscoveryPacket(macAddr)
	if err != nil {
		return err
	}
	discoveryMessage.setOption(optionParameterRequestList, optionValueParameterRequestListDefault)
	transactionID, err := discoveryMessage.transactionID()
	if err != nil {
		return err
	}
	requestMessage, err := createRequestPacket(transactionID, macAddr)
	if err != nil {
		return err
	}
	requestMessage.setOption(optionParameterRequestList, optionValueParameterRequestListDefault)
	DHCPServerConfig := map[optionInterface]interface{}{
		optionServerID:    serverIP,
		optionSubnetMask:  subnetMask,
		optionIPLeaseTime: leaseTimeSeconds,
		optionRequestedIP: intendedIP,
	}
	rule1 := NewRespondToDiscoveryRule(intendedIP, serverIP, DHCPServerConfig, map[fieldInterface]interface{}{}, true)
	rule2 := NewRespondToRequestRule(intendedIP, serverIP, DHCPServerConfig, map[fieldInterface]interface{}{}, true, "", "", true)
	rule2.IsFinalHandler = true
	rules := []DHCPHandlingRuleInterface{rule1, rule2}
	server.StartTest(rules, testTimeout)

	clientSocket, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_DGRAM, syscall.IPPROTO_UDP)
	if err != nil {
		return err
	}
	timeout := syscall.Timeval{0, 1000}
	if err = syscall.SetsockoptTimeval(clientSocket, syscall.SOL_SOCKET, syscall.SO_RCVTIMEO, &timeout); err != nil {
		return err
	}
	if err = syscall.SetsockoptTimeval(clientSocket, syscall.SOL_SOCKET, syscall.SO_SNDTIMEO, &timeout); err != nil {
		return err
	}
	if err = syscall.Bind(clientSocket, &syscall.SockaddrInet4{Addr: serverIPParsed, Port: 8068}); err != nil {
		return err
	}

	discoveryMessageStr, err := discoveryMessage.toBinaryString()
	if err != nil {
		return err
	}
	syscall.Sendto(clientSocket, []byte(discoveryMessageStr), 0, &syscall.SockaddrInet4{Addr: serverIPParsed, Port: 8067})
	offerPacket, err := receivePacket(clientSocket, 1*time.Second)
	if err != nil {
		return err
	}
	offerType, err := offerPacket.messageType()
	if err != nil {
		return err
	}
	if offerType != messageTypeOffer {
		return errors.New("type of DHCP repsonse is not offer")
	}
	offerIP := offerPacket.getField(fieldYourIP)
	offerIPStr, ok := offerIP.(string)
	if !ok {
		return errors.New("offered IP is not string type")
	}
	if offerIPStr != intendedIP {
		return errors.New("server didn't give us the IP we expected")
	}

	requestMessage.setOption(optionServerID, offerPacket.getOption(optionServerID))
	requestMessage.setOption(optionSubnetMask, offerPacket.getOption(optionSubnetMask))
	requestMessage.setOption(optionIPLeaseTime, offerPacket.getOption(optionIPLeaseTime))
	requestMessage.setOption(optionRequestedIP, offerPacket.getOption(optionRequestedIP))
	requestMessageStr, err := requestMessage.toBinaryString()
	if err != nil {
		return err
	}
	syscall.Sendto(clientSocket, []byte(requestMessageStr), 0, &syscall.SockaddrInet4{Addr: serverIPParsed, Port: 8067})
	ackPacket, err := receivePacket(clientSocket, 1*time.Second)
	if err != nil {
		return err
	}
	ackType, err := ackPacket.messageType()
	if err != nil {
		return err
	}
	if ackType != messageTypeAck {
		return errors.New("type of DHCP response is not acknowledgment")
	}
	ackIP := ackPacket.getField(fieldYourIP)
	ackIPStr, ok := ackIP.(string)
	if !ok {
		return errors.New("given IP is not string type")
	}
	if ackIPStr != intendedIP {
		return errors.New("server didn't give us the IP we expected")
	}
	server.WaitForTestToFinish()
	if !server.lastTestPassed {
		return errors.New("server is unhappy with the test result")
	}
	return nil
}

func TestServerDialogue(t *testing.T) {
	ip := []byte{127, 0, 0, 1}
	server := NewDHCPTestServer("", ip, 8067, ip, 8068)
	server.StartServer()
	if server.AtomicIsHealthy() {
		err := simpleServerExchange(server)
		if err != nil {
			t.Errorf("test failed with error: %v", err)
		}
	} else {
		t.Error("server isn't healthy, aborting")
	}
	server.Stop()
}
