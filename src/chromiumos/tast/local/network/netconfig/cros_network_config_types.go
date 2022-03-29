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

// ManagedStringList contains active value, if required one may add policy value
// and source.
type ManagedStringList struct {
	ActiveValue []string `json:"activeValue"`
}

// ManagedBoolean contains active value, if required one may add policy value
// and source.
type ManagedBoolean struct {
	ActiveValue bool `json:"activeValue"`
}

// ManagedSubjectAltNameMatchList contains active value, if required one may add
// policy value and source.
type ManagedSubjectAltNameMatchList struct {
	ActiveValue []SubjectAltName `json:"activeValue"`
}

// ManagedEAPProperties contains properties for EAP networks.
// Currently only PEAP networks without CA certificates are supported.
// We include the same fields as EAPConfigProperties.
type ManagedEAPProperties struct {
	AnonymousIdentity   ManagedString                  `json:"anonymousIdentity,omitempty"`
	Identity            ManagedString                  `json:"identity,omitempty"`
	Inner               ManagedString                  `json:"inner,omitempty"`
	Outer               ManagedString                  `json:"outer,omitempty"`
	Password            ManagedString                  `json:"password,omitempty"`
	SaveCredentials     ManagedBoolean                 `json:"saveCredentials,omitempty"`
	ClientCertType      ManagedString                  `json:"clientCertType,omitempty"`
	DomainSuffixMatch   ManagedStringList              `json:"domainSuffixMatch,omitempty"`
	SubjectAltNameMatch ManagedSubjectAltNameMatchList `json:"subjectAltNameMatch,omitempty"`
	UseSystemCAs        ManagedBoolean                 `json:"useSystemCas,omitempty"`
}

// ManagedWiFiProperties contain managed properties of a wifi connection.
type ManagedWiFiProperties struct {
	// Passphrase is only used for PSK networks and Eap is only used for EAP.
	// These fields should not be included in the json object at all otherwise
	// (not even as an empty object). Setting the optional field as a pointer
	// allows it to be nullable and to not appear in the json object if not
	// provided.
	Eap        *ManagedEAPProperties `json:"eap,omitempty"`
	Passphrase *ManagedString        `json:"passphrase,omitempty"`
	Ssid       ManagedString         `json:"ssid"`
	Security   SecurityType          `json:"security"`
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

// SubjectAltNameType is the type for SubjectAltName.
type SubjectAltNameType int

// Allowed types for the alternative subject name.
const (
	Email SubjectAltNameType = iota
	DNS
	URI
)

// SubjectAltName contains the information of an alternative subject name.
type SubjectAltName struct {
	Type  SubjectAltNameType `json:"type"`
	Value string             `json:"value"`
}

// EAPConfigProperties contains properties for EAP networks.
// Currently only PEAP networks without CA certificates are supported, so the
// fields related to certificates are not included: ServerCAPEMs, ServerCARefs,
// ServerCARef (deprecated), SubjectMatch, TLSVersionMax, UseProactiveKeyCaching.
type EAPConfigProperties struct {
	AnonymousIdentity   string           `json:"anonymousIdentity,omitempty"`
	Identity            string           `json:"identity,omitempty"`
	Inner               string           `json:"inner,omitempty"` // "Automatic"
	Outer               string           `json:"outer"`           // "PEAP"
	Password            string           `json:"password,omitempty"`
	SaveCredentials     bool             `json:"saveCredentials,omitempty"` // true.
	ClientCertType      string           `json:"clientCertType,omitempty"`  // "None".
	DomainSuffixMatch   []string         `json:"domainSuffixMatch"`         // Empty in manual example but not optional in mojo.
	SubjectAltNameMatch []SubjectAltName `json:"subjectAltNameMatch"`       // Empty in manual example but not optional in mojo.
	UseSystemCAs        bool             `json:"useSystemCAs,omitempty"`    // false. Defaults to true.
}

// WiFiConfigProperties is used to create new configurations or augment
// existing ones.
type WiFiConfigProperties struct {
	// Eap configuration is only used if the wifi security is WpaEap and it should
	// not be included in the json object at all otherwise (not even as an empty
	// object). Setting the field Eap as a pointer allows it to be nullable and
	// to not appear in the json object if not provided.
	Eap        *EAPConfigProperties `json:"eap,omitempty"`
	Passphrase string               `json:"passphrase,omitempty"`
	Ssid       string               `json:"ssid,omitempty"`
	Security   SecurityType         `json:"security"`
	HiddenSsid HiddenSsidMode       `json:"hiddenSsid"`
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
