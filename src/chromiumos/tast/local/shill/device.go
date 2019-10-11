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

	// Ethernet device property names.
	DevicePropertyEthernetBusType   DeviceProperty = "Ethernet.DeviceBusType"
	DevicePropertyEthernetLinkUp    DeviceProperty = "Ethernet.LinkUp"
	DevicePropertyEthernetMACSource DeviceProperty = "Ethernet.UsbEthernetMacAddressSource"
)

// Device wraps a Device D-Bus object in shill.
type Device struct {
	conn *dbus.Conn
	obj  dbus.BusObject
	path dbus.ObjectPath
}

// NewDevice connects to shill's Device.
func NewDevice(ctx context.Context, path dbus.ObjectPath) (*Device, error) {
	conn, obj, err := dbusutil.Connect(ctx, dbusService, path)
	if err != nil {
		return nil, err
	}
	m := &Device{conn: conn, obj: obj, path: path}
	return m, nil
}

func (d *Device) String() string {
	return string(d.path)
}

// GetProperties returns a list of properties provided by the device.
func (d *Device) GetProperties(ctx context.Context) (map[DeviceProperty]interface{}, error) {
	props := make(map[DeviceProperty]interface{})
	if err := call(ctx, d.obj, dbusDeviceInterface, "GetProperties").Store(&props); err != nil {
		return nil, errors.Wrap(err, "failed getting properties")
	}
	return props, nil
}

// SetUsbEthernetMacAddressSource sets USB Ethernet MAC address source for the device.
func (d *Device) SetUsbEthernetMacAddressSource(ctx context.Context, source string) error {
	if err := call(ctx, d.obj, dbusDeviceInterface, "SetUsbEthernetMacAddressSource", source).Err; err != nil {
		return errors.Wrap(err, "failed set USB Ethernet MAC address source")
	}
	return nil
}

// WatchPropertyChanged returns a SignalWatcher to observe the
// "PropertyChanged" signal.
func (d *Device) WatchPropertyChanged(ctx context.Context) (*dbusutil.SignalWatcher, error) {
	spec := dbusutil.MatchSpec{
		Type:      "signal",
		Path:      d.path,
		Interface: dbusDeviceInterface,
		Member:    "PropertyChanged",
	}
	return dbusutil.NewSignalWatcher(ctx, d.conn, spec)
}
