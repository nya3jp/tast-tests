// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package shill

import (
	"context"

	"github.com/godbus/dbus"

	"chromiumos/tast/errors"
)

const (
	dbusDeviceInterface = "org.chromium.flimflam.Device"
)

// Device property names defined in dbus-constants.h .
const (
	// Device property names.
	DevicePropertyAddress   = "Address"
	DevicePropertyInterface = "Interface"
	DevicePropertyType      = "Type"

	// Ethernet device property names.
	DevicePropertyEthernetBusType   = "Ethernet.DeviceBusType"
	DevicePropertyEthernetLinkUp    = "Ethernet.LinkUp"
	DevicePropertyEthernetMACSource = "Ethernet.UsbEthernetMacAddressSource"
)

// Device wraps a Device D-Bus object in shill.
// It also caches device properties when GetProperties() is called.
type Device struct {
	PropertyHolder
}

// NewDevice connects to shill's Device.
// It also obtains properties after device creation.
func NewDevice(ctx context.Context, path dbus.ObjectPath) (*Device, error) {
	ph, err := NewPropertyHolder(ctx, dbusDeviceInterface, path)
	if err != nil {
		return nil, err
	}
	return &Device{PropertyHolder: ph}, nil
}

// SetUsbEthernetMacAddressSource sets USB Ethernet MAC address source for the device.
func (d *Device) SetUsbEthernetMacAddressSource(ctx context.Context, source string) error {
	if err := d.dbusObject.Call(ctx, "SetUsbEthernetMacAddressSource", source).Err; err != nil {
		return errors.Wrap(err, "failed set USB Ethernet MAC address source")
	}
	return nil
}
