// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

// A simplified version of the types in cros_network_config.mojom and
// network_types.mojom to be used in tests. The JSON marshalling comments are
// required for passing structs to javascript.

// network_types.mojom

type NetworkType int

const (
	All      NetworkType = 0
	Cellular NetworkType = 1
	Ethernet NetworkType = 2
	Mobile   NetworkType = 3
	Tether   NetworkType = 4
	VPN      NetworkType = 5
	Wireless NetworkType = 6
	WiFi     NetworkType = 7
)

// cros_network_config.mojom

type SecurityType int

const (
	None     SecurityType = 0
	Wep8021x SecurityType = 1
	WepPsk   SecurityType = 2
	WpaEap   SecurityType = 3
	WpaPsk   SecurityType = 4
)

type HiddenSsidMode int

const (
	Automatic HiddenSsidMode = 0
	Disabled  HiddenSsidMode = 1
	Enabled   HiddenSsidMode = 2
)

type ManagedString struct {
	ActiveValue string `json:"activeValue"`
}

type ManagedWiFiProperties struct {
	Passphrase ManagedString `json:"passphrase", omitempty`
	Ssid       ManagedString `json:"ssid"`
	Security   SecurityType  `json:"security"`
}

type NetworkTypeManagedProperties struct {
	Wifi ManagedWiFiProperties `json:"wifi"`
}

type ManagedProperties struct {
	Type           NetworkType                  `json:"type"`
	TypeProperties NetworkTypeManagedProperties `json:"typeProperties"`
}

type WiFiConfigProperties struct {
	Passphrase string         `json:"passphrase", omitempty`
	Ssid       string         `json:"ssid", omitempty`
	Security   SecurityType   `json:"security"`
	HiddenSsid HiddenSsidMode `json:"hiddenSsid"`
}

type NetworkTypeConfigProperties struct {
	Wifi WiFiConfigProperties `json:"wifi"`
}

type ConfigProperties struct {
	TypeConfig NetworkTypeConfigProperties `json:"typeConfig"`
}
