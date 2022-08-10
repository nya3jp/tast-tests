// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bluez

import (
	"context"

	"github.com/godbus/dbus/v5"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/testing"
)

// Device is the bluez dbus remote device object abstraction.
type Device struct {
	dbus *dbusutil.DBusObject
}

// NewDevice creates a new bluetooth Device from the passed D-Bus object path.
func NewDevice(ctx context.Context, path dbus.ObjectPath) (*Device, error) {
	obj, err := NewBluezDBusObject(ctx, bluezDeviceIface, path)
	if err != nil {
		return nil, err
	}
	a := &Device{
		dbus: obj,
	}
	return a, nil
}

// Devices creates a Device for all bluetooth devices in the system.
func Devices(ctx context.Context) ([]*Device, error) {
	paths, err := collectExistingBluezObjectPaths(ctx, bluezDeviceIface)
	if err != nil {
		return nil, err
	}
	devices := make([]*Device, len(paths))
	for i, path := range paths {
		device, err := NewDevice(ctx, path)
		if err != nil {
			return nil, err
		}
		devices[i] = device
	}
	return devices, nil
}

// DBusObject returns the D-Bus object wrapper for this object.
func (d *Device) DBusObject() *dbusutil.DBusObject {
	return d.dbus
}

// Path gets the D-Bus path this device was created from.
func (d *Device) Path() dbus.ObjectPath {
	return d.dbus.ObjectPath()
}

// Address returns the Address of the bluetooth remote device.
func (d *Device) Address(ctx context.Context) (string, error) {
	return d.dbus.PropertyString(ctx, "Address")
}

// Alias returns the alias of the bluetooth remote device.
func (d *Device) Alias(ctx context.Context) (string, error) {
	return d.dbus.PropertyString(ctx, "Alias")
}

// Connected returns true if the bluetooth remote device is connected.
func (d *Device) Connected(ctx context.Context) (bool, error) {
	return d.dbus.PropertyBool(ctx, "Connected")
}

// Paired returns true if the bluetooth remote device is paired.
func (d *Device) Paired(ctx context.Context) (bool, error) {
	return d.dbus.PropertyBool(ctx, "Paired")
}

// Connect connects to the bluetooth remote device.
func (d *Device) Connect(ctx context.Context) error {
	c := d.dbus.Call(ctx, "Connect")
	if c.Err != nil {
		return errors.Wrap(c.Err, "failed to connect")
	}
	return nil
}

// Disconnect disconnects the bluetooth remote device.
func (d *Device) Disconnect(ctx context.Context) error {
	c := d.dbus.Call(ctx, "Disconnect")
	if c.Err != nil {
		return errors.Wrap(c.Err, "failed to disconnect")
	}
	return nil
}

// Pair pairs the bluetooth remote device.
func (d *Device) Pair(ctx context.Context) error {
	c := d.dbus.Call(ctx, "Pair")
	if c.Err != nil {
		return errors.Wrap(c.Err, "failed to pair")
	}
	return nil
}

// DeviceByAlias creates a new bluetooth device object by the alias name of the device.
func DeviceByAlias(ctx context.Context, alias string) (*Device, error) {
	devices, err := Devices(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get device list")
	}
	for _, d := range devices {
		al, err := d.Alias(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get alias property")
		}
		if al == alias {
			return d, nil
		}
	}
	return nil, errors.Errorf("device with the given alias %q not found", alias)
}

// DeviceByAddress creates a new bluetooth device object by the address of the device.
func DeviceByAddress(ctx context.Context, address string) (*Device, error) {
	devices, err := Devices(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get device list")
	}
	for _, d := range devices {
		ad, err := d.Address(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get address property")
		}
		if ad == address {
			return d, nil
		}
	}
	return nil, errors.Errorf("device with the given address %s not found", address)
}

// DisconnectAllDevices disconnects all remote bluetooth devices.
func DisconnectAllDevices(ctx context.Context) error {
	devices, err := Devices(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get device list")
	}
	var firstErr error
	recordErr := func(d *Device, err error) {
		testing.ContextLogf(ctx, "Failure during disconnecting %q: %v", d.Path(), err)
		if firstErr != nil {
			firstErr = err
		}
	}
	for _, d := range devices {
		connected, err := d.Connected(ctx)
		if err != nil {
			recordErr(d, errors.Wrap(err, "failed to get connected property"))
			continue
		}
		paired, err := d.Paired(ctx)
		if err != nil {
			recordErr(d, errors.Wrap(err, "failed to get paired property"))
			continue
		}

		if paired && connected {
			if err := d.Disconnect(ctx); err != nil {
				recordErr(d, errors.Wrap(err, "failed to disconnect"))
			}
		}
	}
	return firstErr
}
