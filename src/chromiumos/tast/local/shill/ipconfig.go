// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package shill

import (
	"context"

	"github.com/godbus/dbus"
)

const (
	dbusIPConfigInterface = "org.chromium.flimflam.IPConfig"
)

// IPConfig property names.
const (
	IPConfigPropertyAddress                   = "Address"
	IPConfigPropertyNameServers               = "NameServers"
	IPConfigPropertyBroadcast                 = "Broadcast"
	IPConfigPropertyDomainName                = "DomainName"
	IPConfigPropertyGateway                   = "Gateway"
	IPConfigPropertyMethos                    = "Method"
	IPConfigPropertyMtu                       = "Mtu"
	IPConfigPropertyPeerAddress               = "PeerAddress"
	IPConfigPropertyPrefixlen                 = "Prefixlen"
	IPConfigPropertyVendorEncapsulatedOptions = "VendorEncapsulatedOptions"
	IPConfigPropertyWebProxyAutoDiscoveryURL  = "WebProxyAutoDiscoveryUrl"
	IPConfigPropertyiSNSOptionData            = "iSNSOptionData"
)

// IPConfig wraps an IPConfig D-Bus object in shill.
type IPConfig struct {
	PropertyHolder
}

// NewIPConfig connects to an IPConfig in Shill.
func NewIPConfig(ctx context.Context, path dbus.ObjectPath) (*IPConfig, error) {
	ph, err := NewPropertyHolder(ctx, dbusIPConfigInterface, path)
	if err != nil {
		return nil, err
	}
	return &IPConfig{PropertyHolder: ph}, nil
}
