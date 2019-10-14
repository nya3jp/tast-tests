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
	dbusObject *DBusObject
	props      *Properties
}

// NewDevice connects to shill's Device.
// It also obtains properties after device creation.
func NewDevice(ctx context.Context, path dbus.ObjectPath) (*Device, error) {
	conn, obj, err := dbusutil.Connect(ctx, dbusService, path)
	if err != nil {
		return nil, err
	}

	dbusObject := &DBusObject{Interface: dbusDeviceInterface, Object: obj, Conn: conn}
	props, err := NewProperties(ctx, dbusObject)
	if err != nil {
		return nil, err
	}
	return &Device{dbusObject: dbusObject, props: props}, nil
}

// Properties returns existing properties.
func (d *Device) Properties() *Properties {
	return d.props
}

// String returns the path of the device.
// It is so named to conforms the Stringer interface.
func (d *Device) String() string {
	return string(d.dbusObject.Object.Path())
}

// GetProperties refreshes and returns properties.
func (d *Device) GetProperties(ctx context.Context) (*Properties, error) {
	props, err := NewProperties(ctx, d.dbusObject)
	if err != nil {
		return nil, err
	}
	d.props = props
	return props, nil
}

// SetUsbEthernetMacAddressSource sets USB Ethernet MAC address source for the device.
func (d *Device) SetUsbEthernetMacAddressSource(ctx context.Context, source string) error {
	if err := call(ctx, d.dbusObject.Object, d.dbusObject.Interface, "SetUsbEthernetMacAddressSource", source).Err; err != nil {
		return errors.Wrap(err, "failed set USB Ethernet MAC address source")
	}
	return nil
}
