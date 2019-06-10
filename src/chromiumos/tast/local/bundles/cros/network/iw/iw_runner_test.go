// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package network contains local Tast tests that exercise the Chrome OS network stack.
package iw

import (
	"fmt"
	"reflect"
	"testing"
)

//Test all parsing primitives in iw_runner.go
func TestParsers(t *testing.T) {
	testGetAllLinkKeys(t)
	testExtractBssid(t)
	testParseScanResults(t)
	testParseScanTime(t)
}

func testGetAllLinkKeys(t *testing.T) {
	testStr := `Connected to 74:e5:43:10:4f:c0 (on wlan0)
      SSID: PMKSACaching_4m9p5_ch1
      freq: 5220
      RX: 5370 bytes (37 packets)
      TX: 3604 bytes (15 packets)
      signal: -59 dBm
      tx bitrate: 13.0 MBit/s MCS 1

      bss flags:      short-slot-time
      dtim period:    5
      beacon int:     100`
	cmpMap := map[string]string{
		"SSID":        "PMKSACaching_4m9p5_ch1",
		"freq":        "5220",
		"TX":          "3604 bytes (15 packets)",
		"signal":      "-59 dBm",
		"bss flags":   "short-slot-time",
		"dtim period": "5",
		"beacon int":  "100",
		"RX":          "5370 bytes (37 packets)",
		"tx bitrate":  "13.0 MBit/s MCS 1",
	}
	linkMap := getAllLinkKeys(testStr)
	if !reflect.DeepEqual(linkMap, cmpMap) {
		t.Error(fmt.Println("getAllLinkKeys: Map behavior differs from expected: Got ", linkMap, " but expected ", cmpMap))
	}
}

func testExtractBssid(t *testing.T) {
	testStr1 := `Connected to 04:f0:21:03:7d:bb (on wlan0)`
	testStr2 := `Station 04:f0:21:03:7d:bb (on mesh-5000mhz)`
	cmpStr := `04:f0:21:03:7d:bb`
	res := extractBssid(testStr1, "wlan0", false)
	if res != cmpStr {
		t.Error(fmt.Sprintf("extractBssid: Failed on %s. Got %s, expected %s", testStr1, res, cmpStr))
	}
	res = extractBssid(testStr2, "mesh-5000mhz", true)
	if res != cmpStr {
		t.Error(fmt.Sprintf("extractBssid: Failed on %s. Got %s, expected %s", testStr2, res, cmpStr))
	}
}

func testParseScanResults(t *testing.T) {
	testStr := `BSS 00:11:22:33:44:55(on wlan0)
          freq: 2447
          beacon interval: 100 TUs
          signal: -46.00 dBm
          Information elements from Probe Response frame:
          SSID: my_open_network
          Extended supported rates: 24.0 36.0 48.0 54.0
          HT capabilities:
          Capabilities: 0x0c
          HT20
          HT operation:
          * primary channel: 8
          * secondary channel offset: no secondary
          * STA channel width: 20 MHz
          RSN: * Version: 1
          * Group cipher: CCMP
          * Pairwise ciphers: CCMP
          * Authentication suites: PSK
          * Capabilities: 1-PTKSA-RC 1-GTKSA-RC (0x0000)`
	iwr := NewIwRunner(nil, nil)
	var l []*IwBss = iwr.parseScanResults([]byte(testStr))
	var cmpBss IwBss = IwBss{Bss: "00:11:22:33:44:55", Frequency: 2447, Ssid: "my_open_network", Security: "WPA2", Ht: "HT20", Signal: -46}
	if len(l) > 1 {
		t.Error(fmt.Sprintf("parseScanResults: Too many Bss entries. Got %d, expected 1", len(l)))
	}
	if *l[0] != cmpBss {
		t.Error(fmt.Println("parseScanResults: Bss Entries differ. Got: ", *l[0], " Expected: ", cmpBss))
	}
}

func testParseScanTime(t *testing.T) {
	testStr := "Command Executed\nreal     3.01\nuser     2.01\nsys      1.00"
	iwr := NewIwRunner(nil, nil)
	cmpFloat := 3.01
	var res float64 = iwr.parseScanTime([]byte(testStr))
	if res != cmpFloat {
		t.Error(fmt.Sprintf("parseScanTime: Results differ. Got: %f Expected: %f", res, cmpFloat))
	}
}
