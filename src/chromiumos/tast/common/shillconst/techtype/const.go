// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package techtype defines the technology types in shill.
package techtype

// Type values defined in dbus-constants.h
// The values are used both for Service type and Technology type.
const (
	Ethernet = "ethernet"
	Wifi     = "wifi"
	Cellular = "cellular"
	VPN      = "vpn"
	PPPoE    = "pppoe"
)
