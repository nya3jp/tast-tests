// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package shill

import (
	"context"

	"github.com/godbus/dbus"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/dbusutil"
)

const (
	dbusDeviceInterface = "org.chromium.flimflam.Device"
)

// DeviceProperty is the type for device property names.
type DeviceProperty string

// Device property names defined in dbus-constants.h .
const (
	// Device property names.
	DevicePropertyInterface DeviceProperty = "Interface"
	DevicePropertyType      DeviceProperty = "Type"

	// Ethernet device property names.
	DevicePropertyEthernetBusType   DeviceProperty = "Ethernet.DeviceBusType"
	DevicePropertyEthernetLinkUp    DeviceProperty = "Ethernet.LinkUp"
	DevicePropertyEthernetMACSource DeviceProperty = "Ethernet.UsbEthernetMacAddressSource"
)

// Device wraps a Device D-Bus object in shill.
type Device struct {
	obj  dbus.BusObject
	path dbus.ObjectPath
}

// NewDevice connects to shill's Device.
func NewDevice(ctx context.Context, path dbus.ObjectPath) (*Device, error) {
	_, obj, err := dbusutil.Connect(ctx, dbusService, path)
	if err != nil {
		return nil, err
	}
	m := &Device{obj: obj, path: path}
	return m, nil
}

func (d *Device) String() string {
	return string(d.path)
}

// GetProps returns a list of properties provided by the device.
func (d *Device) GetProps(ctx context.Context) (map[DeviceProperty]interface{}, error) {
	props := make(map[DeviceProperty]interface{})
	if err := call(ctx, d.obj, dbusDeviceInterface, "GetProperties").Store(&props); err != nil {
		return nil, errors.Wrap(err, "failed getting properties")
	}
	return props, nil
}

// SetMACSource sets USB Ethernet MAC address source for the device.
func (d *Device) SetMACSource(ctx context.Context, source string) error {
	if err := call(ctx, d.obj, dbusDeviceInterface, "SetUsbEthernetMacAddressSource", source).Err; err != nil {
		return errors.Wrap(err, "failed set USB Ethernet MAC address source")
	}
	return nil
}
