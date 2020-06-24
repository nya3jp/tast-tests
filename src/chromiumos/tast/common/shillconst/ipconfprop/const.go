// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package ipconfprop defines the constant keys of the IP config of a service in shill.
package ipconfprop

// IPConfig property names.
const (
	Address                   = "Address"
	NameServers               = "NameServers"
	Broadcast                 = "Broadcast"
	DomainName                = "DomainName"
	Gateway                   = "Gateway"
	Methos                    = "Method"
	Mtu                       = "Mtu"
	PeerAddress               = "PeerAddress"
	Prefixlen                 = "Prefixlen"
	VendorEncapsulatedOptions = "VendorEncapsulatedOptions"
	WebProxyAutoDiscoveryURL  = "WebProxyAutoDiscoveryUrl"
	iSNSOptionData            = "iSNSOptionData"
)
