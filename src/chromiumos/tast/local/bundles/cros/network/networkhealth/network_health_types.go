// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package networkhealth

// A simplified version of the types in network_health.mojom,
// network_types.mojom and url.mojom to be used in tests. The JSON marshalling
// comments are required for passing structs to javascript. Note that only
// fields relevant to the tast tests are included.

// There are fields that should not be included in the json object at all (not
// even as an empty object). Setting the optional fields that are of the type
// struct as a pointer allows them to be nullable and to not appear in the json
// object if not provided.

// network_types.mojom types

// PortalState describes the captive portal state. Provides additional details
// when the connection state is Portal.
type PortalState int

const (
	// Unknown : The network is not connected or the portal state is not available.
	Unknown
	// Online : The network is connected and no portal is detected.
	Online
	// PortalSuspected : A portal is suspected but no redirect was provided.
	PortalSuspected
	// Portal : The network is in a portal state with a redirect URL.
	Portal
	// ProxyAuthRequired : A proxy requiring authentication is detected.
	ProxyAuthRequired
	// NoInternet : The network is connected but no internet is available and no proxy
	// was detected.
	NoInternet
)

// url.mojom types

// URL contains a string describing a URL.
type URL struct {
	URL string `json:"url"`
}

// network_health.mojom types

// Network is returned by GetNetworkList
type Network struct {
	Type           bool        `json:"type"`
	State          bool        `json:"state"`
	GUID           string      `json:"guid,omitempty"`
	Name           string      `json:"name,omitempty"`
	PortalState    PortalState `json:"portalState"`
	PortalProbeURL URL         `json:"portalProbeUrl,omitempty"`
}
