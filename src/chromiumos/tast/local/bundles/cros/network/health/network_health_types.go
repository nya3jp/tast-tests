// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package health

// A simplified version of the types in network_health.mojom,
// network_types.mojom and url.mojom to be used in tests. The JSON marshalling
// comments are required for passing structs to javascript. Note that only
// fields relevant to the tast tests are included.

// There are fields that should not be included in the json object at all (not
// even as an empty object). Setting the optional fields that are of the type
// struct as a pointer allows them to be nullable and to not appear in the json
// object if not provided.

// network_types.mojom types

// NetworkType describes the network technology type. NOTE: 'All' and
// 'Wireless' are only used by FilterType for requesting groups of networks.
type NetworkType int

const (
	// AllNT : All the network types.
	AllNT NetworkType = iota
	// CellularNT : Cellular network type.
	CellularNT
	// EthernetNT : Ethernet network type.
	EthernetNT
	// MobileNT : Mobile network type. Mobile includes Cellular, and
	// Tether.
	MobileNT
	// TetherNT : Tether network type.
	TetherNT
	// VPNNT : VPN network type.
	VPNNT
	// WirelessNT : Wireless network type.  Wireles includes Cellular,
	// Tether, and WiFi.
	WirelessNT
	// WiFiNT : WiFi network type.
	WiFiNT
)

// PortalState describes the captive portal state. Provides additional details
// when the connection state is Portal.
type PortalState int

const (
	// UnknownPS : The network is not connected or the portal state is not
	// available.
	UnknownPS PortalState = iota
	// OnlinePS : The network is connected and no portal is detected.
	OnlinePS
	// PortalSuspectedPS : A portal is suspected but no redirect was provided.
	PortalSuspectedPS
	// PortalPS : The network is in a portal state with a redirect URL.
	PortalPS
	// ProxyAuthRequiredPS : A proxy requiring authentication is detected.
	ProxyAuthRequiredPS
	// NoInternetPS : The network is connected but no internet is available
	// and no proxy was detected.
	NoInternetPS
)

// url.mojom types

// URL contains a string describing a URL.
type URL struct {
	URL string `json:"url"`
}

// network_health.mojom types

// NetworkState is the current state of the network.
type NetworkState int

const (
	// UninitializedNS : The network type is available but not yet
	// initialized.
	UninitializedNS NetworkState = iota
	// DisabledNS : The network type is available but disabled or
	// disabling.
	DisabledNS
	// ProhibitedNS : The network type is prohibited by policy.
	ProhibitedNS
	// NotConnectedNS : The network type is available and enabled or
	// enabling, but no network connection has been established.
	NotConnectedNS
	// ConnectingNS : The network type is available and enabled, and a
	// network connection is in progress.
	ConnectingNS
	// PortalNS : The network is in a portal state.
	PortalNS
	// ConnectedNS : The network is in a connected state, but connectivity
	// is limited.
	ConnectedNS
	// OnlineNS : The network is connected and online.
	OnlineNS
)

// Network is returned by GetNetworkList
type Network struct {
	Type           NetworkType  `json:"type"`
	State          NetworkState `json:"state"`
	GUID           string       `json:"guid,omitempty"`
	Name           string       `json:"name,omitempty"`
	PortalState    PortalState  `json:"portalState"`
	PortalProbeURL URL          `json:"portalProbeUrl,omitempty"`
}
