// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package network provides general CrOS network goodies.
package network

import (
	"io/ioutil"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const (
	dataPathPrefix             = "testdata/"
	classlessStaticRouteData   = "\x12\x0a\x09\xc0\xac\x1f\x9b\x0a\x00\xc0\xa8\x00\xfe"
	domainSearchListCompressed = "\x03eng\x06google\x03com\x00\x09marketing\xC0\x04"
	domainSearchListExpected   = "\x03eng\x06google\x03com\x00\x09marketing\x06google\x03com\x00"
	domainSearchList1          = "w\x10\x03eng\x06google\x03com\x00"
	domainSearchList2          = "w\x16\x09marketing\x06google\x03com\x00"
)

var (
	classlessStaticRouteListParsed = []staticRoute{
		{uint8(18), "10.9.192.0", "172.31.155.10"},
		{uint8(0), "0.0.0.0", "192.168.0.254"},
	}

	domainSearchListParsed = []string{
		"eng.google.com",
		"marketing.google.com",
	}
)

func TestPacketSerialization(t *testing.T) {
	data, err := ioutil.ReadFile(filepath.Join(dataPathPrefix, "dhcp_discovery.log"))
	if err != nil {
		t.Fatalf("unable to read log file: %v", err)
	}
	discoveryPacket, err := newDHCPPacket(data)
	if err != nil {
		t.Fatalf("Unable to create DHCP packet: %v", err)
	}
	if !discoveryPacket.isValid() {
		t.Fatal("Invalid DHCP Packet")
	}
	generatedBytes, err := discoveryPacket.toBinary()
	if err != nil {
		t.Fatalf("Failed to generate string from packet object: %v", err)
	}
	if string(generatedBytes) != string(data) {
		t.Fatalf("Packets didn't match: \n"+
			"Generated: \n%x\n"+
			"Expected: \n%x\n", generatedBytes, data)
	}
}

func TestClasslessStaticRouteParsing(t *testing.T) {
	var opt classlessStaticRoutesOption
	parsedRoutes, err := opt.unpack([]byte(classlessStaticRouteData))
	if err != nil {
		t.Fatalf("Failed to unpack test data: %v", err)
	}
	if !reflect.DeepEqual(parsedRoutes, classlessStaticRouteListParsed) {
		t.Fatalf("Parsed binary domain list and got %v but expected %v", parsedRoutes, classlessStaticRouteListParsed)
	}
}

func TestClasslessStaticRouteSerialization(t *testing.T) {
	var opt classlessStaticRoutesOption
	bytes, err := opt.pack(classlessStaticRouteListParsed)
	if err != nil {
		t.Fatalf("Failed to pack test data: %v", err)
	}
	if string(bytes) != classlessStaticRouteData {
		t.Fatalf("Serialized %v to %x but expected %x.", classlessStaticRouteListParsed, bytes, classlessStaticRouteData)
	}
}

func TestDomainSearchListParsing(t *testing.T) {
	var opt domainListOption
	parsedDomains, err := opt.unpack([]byte(domainSearchListCompressed))
	if err != nil {
		t.Fatalf("Failed to unpack test data: %v", err)
	}
	if !reflect.DeepEqual(parsedDomains, domainSearchListParsed) {
		t.Fatalf("Parsed binary domain list and got %v but expected %v", parsedDomains, domainSearchListExpected)
	}
}

func TestDomainSearchListSerialization(t *testing.T) {
	var opt domainListOption
	bytes, err := opt.pack(domainSearchListParsed)
	if err != nil {
		t.Fatalf("Failed to pack test data: %v", err)
	}
	if string(bytes) != domainSearchListExpected {
		t.Fatalf("Serialized %v to %x but expected %x.", domainSearchListParsed, bytes, domainSearchListExpected)
	}
}

func TestBrokenDomainSearchListParsing(t *testing.T) {
	byteStr := strings.Repeat("\x00", 240) + domainSearchList1 + domainSearchList2 + "\xff"
	packet, err := newDHCPPacket([]byte(byteStr))
	if err != nil {
		t.Fatalf("Unable to create DHCP packet: %v", err)
	}
	if len(packet.options) != 1 {
		t.Fatalf("Domain list of length %d, expected length 1", len(packet.options))
	}
	for _, v := range packet.options {
		if !reflect.DeepEqual(v, domainSearchListParsed) {
			t.Fatalf("Expected binary domain list and got %v but expected %v", v, domainSearchListParsed)
		}
	}
}
