// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package iw contains utility functions to wrap around the iw program.
package iw

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestGetAllLinkKeys(t *testing.T) {
	const testStr = `Connected to 74:e5:43:10:4f:c0 (on wlan0)
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
	if diff := cmp.Diff(linkMap, cmpMap); diff != "" {
		t.Error("getAllLinkKeys returned unexpected map: diff:\n", diff)
	}
}

func TestParseScanResults(t *testing.T) {
	const testStr = `BSS 00:11:22:33:44:55(on wlan0)
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
	l, err := parseScanResults(testStr)
	if err != nil {
		t.Fatal("parseScanResults failed: ", err)
	}
	cmpBSS := []*BSSData{
		&BSSData{
			BSS:       "00:11:22:33:44:55",
			Frequency: 2447,
			SSID:      "my_open_network",
			Security:  "RSN",
			HT:        "HT20",
			Signal:    -46,
		},
	}
	if diff := cmp.Diff(l, cmpBSS); diff != "" {
		t.Error("parseScanResults returned unexpected result; diff:\n", diff)
	}
}

func TestNewNetDev(t *testing.T) {
	const testStr = `Interface wlan0
                ifindex 3
                wdev 0x1
                addr 98:2c:bc:c0:ee:32
                type managed
                txpower 0.00 dBm`
	cmpNetDev := &NetDev{Phy: 0, IfName: "wlan0", IfType: "managed"}
	res, err := newNetDev("phy#0", testStr)
	if err != nil {
		t.Error("newNetDev returned error:", err)
	}
	if diff := cmp.Diff(res, cmpNetDev); diff != "" {
		t.Error("newNetDev returned unexpected result; diff:\n", diff)
	}
}
