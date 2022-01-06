// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package netconfig

// A simplified version of the types in cros_network_config.mojom and
// network_types.mojom to be used in tests. The JSON marshalling comments are
// required for passing structs to javascript.

// Types from network_types.mojom

// NetworkType is the network technology type.
type NetworkType int

// Types of networks. Note that All and Wireless are only used for filtering.
const (
	All NetworkType = iota
	Cellular
	Ethernet
	Mobile
	Tether
	VPN
	Wireless
	WiFi
)

// Types from cros_network_config.mojom

// SecurityType is the security for WiFi and Ethernet.
type SecurityType int

// Security types.
const (
	None SecurityType = iota
	Wep8021x
	WepPsk
	WpaEap
	WpaPsk
)

// HiddenSsidMode is the tri-state status of hidden SSID.
type HiddenSsidMode int

// Whether SSID is hidden.
const (
	Automatic HiddenSsidMode = iota
	Disabled
	Enabled
)

// ManagedString contains active value, if required one may add policy value
// and source.
type ManagedString struct {
	ActiveValue string `json:"activeValue"`
}

// ManagedWiFiProperties contain managed properties of a wifi connection.
type ManagedWiFiProperties struct {
	Passphrase ManagedString `json:"passphrase,omitempty"`
	Ssid       ManagedString `json:"ssid"`
	Security   SecurityType  `json:"security"`
}

// NetworkTypeManagedProperties contains managed properties for one of the
// network types. Only WiFi is implemented so far.
type NetworkTypeManagedProperties struct {
	Wifi ManagedWiFiProperties `json:"wifi"`
}

// ManagedProperties are provided by GetManagedProperties, see onc_spec.md for
// details.
type ManagedProperties struct {
	Type           NetworkType                  `json:"type"`
	TypeProperties NetworkTypeManagedProperties `json:"typeProperties"`
}

// WiFiConfigProperties is used to create new configurations or augment
// existing ones.
type WiFiConfigProperties struct {
	Passphrase string         `json:"passphrase,omitempty"`
	Ssid       string         `json:"ssid,omitempty"`
	Security   SecurityType   `json:"security"`
	HiddenSsid HiddenSsidMode `json:"hiddenSsid"`
}

// NetworkTypeConfigProperties contains properties for one type of network.
// Currently only WiFi is supported.
type NetworkTypeConfigProperties struct {
	Wifi WiFiConfigProperties `json:"wifi"`
}

// ConfigProperties is passed to SetProperties or ConfigureNetwork to configure
// a new network or augment an existing one.
type ConfigProperties struct {
	TypeConfig NetworkTypeConfigProperties `json:"typeConfig"`
}
