// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package shill

import (
	"context"

	"github.com/godbus/dbus/v5"

	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/errors"
)

const (
	dbusIPConfigInterface = "org.chromium.flimflam.IPConfig"
)

// IPConfig wraps an IPConfig D-Bus object in shill.
type IPConfig struct {
	*PropertyHolder
}

// IPProperties wraps all the properties in an IPConfig D-Bus object. See
// ipconfig-api.txt in shill for their definitions.
type IPProperties struct {
	Address                   string
	Broadcast                 string
	DomainName                string
	Gateway                   string
	Method                    string
	MTU                       int32
	NameServers               []string
	PeerAddress               string
	PrefixLen                 int32
	VendorEncapsulatedOptions []uint8
	WebProxyAutoDiscoveryURL  string
	ISNSOptionData            []uint8
}

// NewIPConfig connects to an IPConfig in Shill.
func NewIPConfig(ctx context.Context, path dbus.ObjectPath) (*IPConfig, error) {
	ph, err := NewPropertyHolder(ctx, dbusService, dbusIPConfigInterface, path)
	if err != nil {
		return nil, err
	}
	return &IPConfig{PropertyHolder: ph}, nil
}

// GetIPProperties calls GetProperties() on the D-Bus object and returns the
// current property values.
func (ph *IPConfig) GetIPProperties(ctx context.Context) (IPProperties, error) {
	var ipProps IPProperties

	dbusProps, err := ph.GetProperties(ctx)
	if err != nil {
		return ipProps, errors.Wrapf(err, "failed to call GetProperties on IPConfig object %v", ph.ObjectPath())
	}

	for _, fn := range []struct {
		field *string
		name  string
	}{{&ipProps.Address, shillconst.IPConfigPropertyAddress},
		{&ipProps.Broadcast, shillconst.IPConfigPropertyBroadcast},
		{&ipProps.DomainName, shillconst.IPConfigPropertyDomainName},
		{&ipProps.Gateway, shillconst.IPConfigPropertyGateway},
		{&ipProps.Method, shillconst.IPConfigPropertyMethod},
		{&ipProps.PeerAddress, shillconst.IPConfigPropertyPeerAddress},
		{&ipProps.WebProxyAutoDiscoveryURL, shillconst.IPConfigPropertyWebProxyAutoDiscoveryURL},
	} {
		*fn.field, err = dbusProps.GetString(fn.name)
		if err != nil {
			return ipProps, errors.Wrapf(err, "failed to get property %s", fn.name)
		}
	}

	for _, fn := range []struct {
		field *int32
		name  string
	}{{&ipProps.MTU, shillconst.IPConfigPropertyMtu},
		{&ipProps.PrefixLen, shillconst.IPConfigPropertyPrefixlen},
	} {
		*fn.field, err = dbusProps.GetInt32(fn.name)
		if err != nil {
			return ipProps, errors.Wrapf(err, "failed to get property %s", fn.name)
		}
	}

	for _, fn := range []struct {
		field *[]uint8
		name  string
	}{{&ipProps.VendorEncapsulatedOptions, shillconst.IPConfigPropertyVendorEncapsulatedOptions},
		{&ipProps.ISNSOptionData, shillconst.IPConfigPropertyiSNSOptionData},
	} {
		*fn.field, err = dbusProps.GetUint8s(fn.name)
		if err != nil {
			return ipProps, errors.Wrapf(err, "failed to get property %s", fn.name)
		}
	}

	ipProps.NameServers, err = dbusProps.GetStrings(shillconst.IPConfigPropertyNameServers)
	if err != nil {
		return ipProps, errors.Wrapf(err, "failed to get property %s", shillconst.IPConfigPropertyNameServers)
	}

	return ipProps, nil
}
