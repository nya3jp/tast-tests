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
	TEST_DATA_PATH_PREFIX            = "dhcp_test_data/"
	TEST_CLASSLESS_STATIC_ROUTE_DATA = "\x12\x0a\x09\xc0\xac\x1f\x9b\x0a\x00\xc0\xa8\x00\xfe"

	TEST_DOMAIN_SEARCH_LIST_COMPRESSED = "\x03eng\x06google\x03com\x00\x09marketing\xC0\x04"

	TEST_DOMAIN_SEARCH_LIST_EXPECTED = "\x03eng\x06google\x03com\x00\x09marketing\x06google\x03com\x00"

	TEST_DOMAIN_SEARCH_LIST1 = "w\x10\x03eng\x06google\x03com\x00"

	TEST_DOMAIN_SEARCH_LIST2 = "w\x16\x09marketing\x06google\x03com\x00"
)

var (
	TEST_CLASSLESS_STATIC_ROUTE_LIST_PARSED = [][3]interface{}{
		[3]interface{}{uint8(18), "10.9.192.0", "172.31.155.10"},
		[3]interface{}{uint8(0), "0.0.0.0", "192.168.0.254"},
	}

	TEST_DOMAIN_SEARCH_LIST_PARSED = []string{
		"eng.google.com",
		"marketing.google.com",
	}
)

func TestPacketSerialization(t *testing.T) {
	data, err := ioutil.ReadFile(TEST_DATA_PATH_PREFIX + "dhcp_discovery.log")
	if err != nil {
		t.Errorf("unable to read log file: %v", err)
	}
	binaryDiscoveryPacket := string(data)
	discoveryPacket, err := NewDHCPPacket(binaryDiscoveryPacket)
	if err != nil {
		t.Errorf("Unable to create DHCP packet: %v", err)
		return
	}
	if !discoveryPacket.isValid() {
		t.Error("Invalid DHCP Packet")
		return
	}
	generatedString, err := discoveryPacket.toBinaryString()
	if err != nil {
		t.Error("Failed to generate string from packet object.")
		return
	}
	if generatedString != binaryDiscoveryPacket {
		t.Errorf("Packets didn't match: \n"+
			"Generated: \n%x\n"+
			"Expected: \n%x\n", generatedString, binaryDiscoveryPacket)
		return
	}
	t.Log("TestPacketSerialization PASSED")
}

func TestClasslessStaticRouteParsing(t *testing.T) {
	var opt ClasslessStaticRoutesOption
	parsedRoutes, err := opt.Unpack(TEST_CLASSLESS_STATIC_ROUTE_DATA)
	if err != nil {
		t.Errorf("Failed to unpack test data: %v", err)
		return
	}
	if !reflect.DeepEqual(parsedRoutes, TEST_CLASSLESS_STATIC_ROUTE_LIST_PARSED) {
		t.Errorf("Parsed binary domain list and got %v but expected %v", parsedRoutes, TEST_CLASSLESS_STATIC_ROUTE_LIST_PARSED)
		return
	}
	t.Log("TestClasslessStaticRouteParsing PASSED")
}

func TestClasslessStaticRouteSerialization(t *testing.T) {
	var opt ClasslessStaticRoutesOption
	byteStr, err := opt.Pack(TEST_CLASSLESS_STATIC_ROUTE_LIST_PARSED)
	if err != nil {
		t.Errorf("Failed to pack test data: %v", err)
		return
	}
	if byteStr != TEST_CLASSLESS_STATIC_ROUTE_DATA {
		t.Errorf("Expected to serialize %v to %x but instead got %x.", TEST_CLASSLESS_STATIC_ROUTE_LIST_PARSED, TEST_CLASSLESS_STATIC_ROUTE_DATA, byteStr)
		return
	}
	t.Log("TestClasslessStaticRouteSerialization PASSED")
}

func TestDomainSearchListParsing(t *testing.T) {
	var opt DomainListOption
	parsedDomains, err := opt.Unpack(TEST_DOMAIN_SEARCH_LIST_COMPRESSED)
	if err != nil {
		t.Errorf("Failed to unpack test data: %v", err)
		return
	}
	if !reflect.DeepEqual(parsedDomains, TEST_DOMAIN_SEARCH_LIST_PARSED) {
		t.Errorf("Parsed binary domain list and got %v but expected %v", parsedDomains, TEST_DOMAIN_SEARCH_LIST_EXPECTED)
		return
	}
	t.Log("TestDomainSearchListParsing PASSED")
}

func TestDomainSearchListSerialization(t *testing.T) {
	var opt DomainListOption
	byteStr, err := opt.Pack(TEST_DOMAIN_SEARCH_LIST_PARSED)
	if err != nil {
		t.Errorf("Failed to pack test data: %v", err)
		return
	}
	if byteStr != TEST_DOMAIN_SEARCH_LIST_EXPECTED {
		t.Errorf("Expected to serialize %v to %x but instead got %x.", TEST_DOMAIN_SEARCH_LIST_PARSED, TEST_DOMAIN_SEARCH_LIST_EXPECTED, byteStr)
		return
	}
	t.Log("TestDomainSearchListSerialization PASSED")
}

func TestBrokenDomainSearchListParsing(t *testing.T) {
	byteStr := strings.Repeat("\x00", 240) + TEST_DOMAIN_SEARCH_LIST1 + TEST_DOMAIN_SEARCH_LIST2 + "\xff"
	packet, err := NewDHCPPacket(byteStr)
	if err != nil {

	}
	if len(packet.options) != 1 {
		t.Errorf("Expected domain list of length 1")
		return
	}
	for _, v := range packet.options {
		if !reflect.DeepEqual(v, TEST_DOMAIN_SEARCH_LIST_PARSED) {
			t.Errorf("Expected binary domain list and got %v but expected %v", v, TEST_DOMAIN_SEARCH_LIST_PARSED)
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

	packet, err := NewDHCPPacket(string(data))
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

	discoveryMessage, err := CreateDiscoveryPacket(macAddr)
	if err != nil {
		return err
	}
	discoveryMessage.setOption(OPTION_PARAMETER_REQUEST_LIST, OPTION_VALUE_PARAMETER_REQUEST_LIST_DEFAULT)
	transactionID, err := discoveryMessage.transactionID()
	if err != nil {
		return err
	}
	requestMessage, err := CreateRequestPacket(transactionID, macAddr)
	if err != nil {
		return err
	}
	requestMessage.setOption(OPTION_PARAMETER_REQUEST_LIST, OPTION_VALUE_PARAMETER_REQUEST_LIST_DEFAULT)
	DHCPServerConfig := map[OptionInterface]interface{}{
		OPTION_SERVER_ID:     serverIP,
		OPTION_SUBNET_MASK:   subnetMask,
		OPTION_IP_LEASE_TIME: leaseTimeSeconds,
		OPTION_REQUESTED_IP:  intendedIP,
	}
	rule1 := NewRespondToDiscoveryRule(intendedIP, serverIP, DHCPServerConfig, map[FieldInterface]interface{}{}, true)
	rule2 := NewRespondToRequestRule(intendedIP, serverIP, DHCPServerConfig, map[FieldInterface]interface{}{}, true, "", "", true)
	rule2.isFinalHandler = true
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
	if offerType != MESSAGE_TYPE_OFFER {
		return errors.New("type of DHCP repsonse is not offer")
	}
	offerIP := offerPacket.getField(FIELD_YOUR_IP)
	offerIPStr, ok := offerIP.(string)
	if !ok {
		return errors.New("offered IP is not string type")
	}
	if offerIPStr != intendedIP {
		return errors.New("server didn't give us the IP we expected")
	}

	requestMessage.setOption(OPTION_SERVER_ID, offerPacket.getOption(OPTION_SERVER_ID))
	requestMessage.setOption(OPTION_SUBNET_MASK, offerPacket.getOption(OPTION_SUBNET_MASK))
	requestMessage.setOption(OPTION_IP_LEASE_TIME, offerPacket.getOption(OPTION_IP_LEASE_TIME))
	requestMessage.setOption(OPTION_REQUESTED_IP, offerPacket.getOption(OPTION_REQUESTED_IP))
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
	if ackType != MESSAGE_TYPE_ACK {
		return errors.New("type of DHCP response is not acknowledgment")
	}
	ackIP := ackPacket.getField(FIELD_YOUR_IP)
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
