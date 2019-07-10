// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package iw contains utility functions to wrap around the iw program.
package iw

import (
	"reflect"
	"testing"
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
	l := getAllLinkKeys(testStr)

	if !reflect.DeepEqual(l, cmpMap) {
		t.Errorf("unexpected result in getAllLinkKeys: got %v, want %v", l, cmpMap)
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
	if !reflect.DeepEqual(l, cmpBSS) {
		t.Errorf("unexpected result in parseScanResults: got %v, want %v", l, cmpBSS)
	}
}

func TestNewPhy(t *testing.T) {
	const phyString = `Wiphy 3`
	const testStr = `	max # scan SSIDs: 20
	max scan IEs length: 425 bytes
	max # sched scan SSIDs: 20
	max # match sets: 11
	Retry short limit: 7
	Retry long limit: 4
	Coverage class: 0 (up to 0m)
	Device supports RSN-IBSS.
	Device supports AP-side u-APSD.
	Device supports T-DLS.
	Supported Ciphers:
		* WEP40 (00-0f-ac:1)
		* WEP104 (00-0f-ac:5)
		* TKIP (00-0f-ac:2)
		* CCMP-128 (00-0f-ac:4)
		* CMAC (00-0f-ac:6)
	Available Antennas: TX 0 RX 0
	Supported interface modes:
		 * IBSS
		 * managed
		 * monitor
	Band 1:
		Capabilities: 0x11ef
			RX LDPC
			HT20/HT40
			SM Power Save disabled
			RX HT20 SGI
			RX HT40 SGI
			TX STBC
			RX STBC 1-stream
			Max AMSDU length: 3839 bytes
			DSSS/CCK HT40
		Maximum RX AMPDU length 65535 bytes (exponent: 0x003)
		Minimum RX AMPDU time spacing: 4 usec (0x05)
		HT Max RX data rate: 300 Mbps
		HT TX/RX MCS rate indexes supported: 0-15
		Bitrates (non-HT):
			* 1.0 Mbps
		Frequencies:
			* 2412 MHz [1] (22.0 dBm)
	Supported commands:
		 * connect
		 * disconnect`
	l, err := newPhy(phyString, testStr)
	if err != nil {
		t.Fatal("newPhy failed: ", err)
	}
	cmpPhy := &Phy{
		Name: "3",
		Bands: []Band{
			{
				Num:            1,
				FrequencyFlags: map[int][]string{},
				McsIndicies: []int{
					0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15,
				},
			},
		},
		Modes: []string{
			"IBSS",
			"managed",
			"monitor",
		},
		Commands: []string{
			"connect",
			"disconnect",
		},
		Features: []string{
			"RSN-IBSS",
			"AP-side u-APSD",
			"T-DLS",
		},
		RxAntenna:      0,
		TxAntenna:      0,
		MaxScanSSIDs:   20,
		SupportVHT:     false,
		SupportHT2040:  true,
		SupportHT40SGI: true,
	}
	if !reflect.DeepEqual(l, cmpPhy) {
		t.Errorf("unexpected result in newPhy: got %v, want %v", l, cmpPhy)
	}
}
