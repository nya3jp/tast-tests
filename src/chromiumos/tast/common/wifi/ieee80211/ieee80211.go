// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package ieee80211 defines some 802.11 constants not defined by
// gopacket/layers/dot11.go
package ieee80211

// WFAOUI is the OUI corresponding to the WiFi Alliance
var WFAOUI = []byte{0x50, 0x6F, 0x9A}

const (
	// OpClass2GHz is the first 2.4GHz operating class
	OpClass2GHz = 0x51
	// OpClass5GHz is the first 5GHz operating class
	OpClass5GHz = 0x73
)
