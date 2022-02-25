// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package netconfig

// A simplified version of the types in cros_network_config.mojom and
// network_types.mojom to be used in tests. The JSON marshalling comments are
// required for passing structs to javascript.

// Types from ip_address.mojom
type IPAddress struct {
	AddressBytes []uint8
}

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

// DeviceStateType  Device / Technology state for devices.
type DeviceStateType int

const (
	UninitializedDST DeviceStateType = iota
	DisabledDST
	DisablingDST
	EnablingDST
	EnabledDST
	ProhibitedDST
	UnavailableDST
)

type ConnectionStateType int

const (
	OnlineCST ConnectionStateType = iota
	ConnectedCST
	PortalCST
	ConnectingCST
	NotConnectedCST
)

type PortalState int

const (
	UnknownPS PortalState = iota
	OnlinePS
	PortalSuspectedPS
	PortalPS
	ProxyAuthRequiredPS
	NoInternetPS
)

type ProxyMode int

const (
	DirectPM ProxyMode = iota
	AutoDetectPM
	PacScriptPM
	FixedServersPM
	SystemPM
)

// Types from cros_network_config.mojom

type OncSource int

const (
	NoneOS OncSource = iota
	UserOS
	DeviceOS
	UserPolicyOS
	DevicePolicyOS
)

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

// Activation state for Cellular networks.
type ActivationStateType int

const (
	UnknownAST ActivationStateType = iota
	NotActivatedAST
	ActivatingAST
	PartiallyActivatedAST
	ActivatedAST
	NoServiceAST
)

type CellularStateProperties struct {
	Iccid             string              `json:"iccid"`
	Eid               string              `json:"eid"`
	ActivationState   ActivationStateType `json:"activationSstate"`
	NetworkTechnology string              `json:"networkTechnology"`
	Roaming           bool                `json:"roaming"`
	SignalStrength    int32               `json:"signalStrength"`
	SimLocked         bool                `json:"simLocked"`
}

type WiFiStateProperties struct {
	Bssid          string       `json:"bssid"`
	Frequency      int32        `json:"frequency"`
	HexSsid        string       `json:"hexSsid"`
	Security       SecurityType `json:"security"`
	SignalStrength int32        `json:"signalStrength"`
	Ssid           string       `json:"ssid"`
	HiddenSsid     bool         `json:"hiddenSsid"`
}

type NetworkTypeStateProperties struct {
	Cellular CellularStateProperties `json:"cellular,omitempty"`
	//	Ethernet EthernetStateProperties `json:"ethernet,omitempty"`
	//	Tether   TetherStateProperties   `json:"tether,omitempty"`
	//	VPN      VPNStateProperties      `json:"vpn,omitempty"`
	WiFi WiFiStateProperties `json:"wifi,omitempty"`
}

type NetworkStateProperties struct {
	Connectable        bool                       `json:"connnectable"`
	ConnectRequested   bool                       `json:"connectRequested"`
	ConnectionState    ConnectionStateType        `json:"connectionState"`
	ErrorState         string                     `json:"errorState,omitempty"`
	Guid               string                     `json:"guid"`
	Name               string                     `json:"name"`
	PortalState        PortalState                `json:"portalState"`
	Priority           int32                      `json:"priority"`
	ProxyMode          ProxyMode                  `json:"proxyMode"`
	ProhibitedByPolicy bool                       `json:"prohibitedByPolicy"`
	Source             OncSource                  `json:"source"`
	Type               NetworkType                `json:"type"`
	TypeState          NetworkTypeStateProperties `json:"typeState"`
}

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

// FilterType is used for requesting lists of network states.
type FilterType int

const (
	ActiveFT FilterType = iota
	VisibleFT
	ConfiguredFT
	AllFT
)

// NetworkFilter is passed to GetNetworkStateList to filter the list of networks returned
type NetworkFilter struct {
	Filter      FilterType  `json:"filter`
	NetworkType NetworkType `json:"networktype`
	Limit       int32       `json:"limit`
}

// The SIM card lock status for Cellular networks.
type SIMLockStatus struct {
	LockType    string `json:"locktype"`
	LockEnabled bool   `json:"lockenabled"`
	RetriesLeft int32  `json:"retriesleft"`
}

// Details about a sim slot available on the device.
type SIMInfo struct {
	SlotId    int32  `json:"slotId"`
	Eid       string `json:"eid"`
	Iccid     string `json:"iccid"`
	IsPrimary bool   `json:"isPrimary"`
}

// Reasons why the Cellular Device may have its scanning inhibited (i.e. temporarily stopped).
type InhibitReason int

const (
	NotInhibited InhibitReason = iota
	InstallingProfile
	RenamingProfile
	RemovingProfile
	ConnectingToProfile
	RefreshingProfileList
	ResettingEuiccMemory
	DisablingProfile
)

type DeviceStateProperties struct {
	Ipv4Address             IPAddress       `json:"ipv4address,omitempty"`
	Ipv6Address             IPAddress       `json:"ipv6address,omitempty"`
	MacAddress              string          `json:"macaddress,omitempty"`
	scanning                bool            `json:"scanning"`
	SimLockStatus           SIMLockStatus   `json:"simlockstatus"`
	SimInfos                []SIMInfo       `json:"siminfos,omitempty"`
	InhibitReason           InhibitReason   `json:"inhibitreason"`
	SimAbsent               bool            `json:"simabsent"`
	DeviceState             DeviceStateType `json:"devicestate"`
	Type                    NetworkType     `json:"type"`
	ManagedNetworkAvailable bool            `json:"managednetworkavailable"`
}
