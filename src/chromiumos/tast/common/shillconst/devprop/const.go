// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package devprop defines the constant keys of Device's properties in shill.
package devprop

// Device property names defined in dbus-constants.h .
const (
	// Common device property names.
	Address         = "Address"
	Interface       = "Interface"
	Type            = "Type"
	SelectedService = "SelectedService"

	// Ethernet device property names.
	EthernetBusType   = "Ethernet.DeviceBusType"
	EthernetLinkUp    = "Ethernet.LinkUp"
	EthernetMACSource = "Ethernet.UsbEthernetMacAddressSource"
	EapDetected       = "EapAuthenticatorDetected"
	EapCompleted      = "EapAuthenticationCompleted"

	// WiFi device property names.
	WiFiBgscanMethod       = "BgscanMethod"
	MACAddrRandomEnabled   = "MACAddressRandomizationEnabled"
	MACAddrRandomSupported = "MACAddressRandomizationSupported"
	Scanning               = "Scanning" // Also for cellular.
)
