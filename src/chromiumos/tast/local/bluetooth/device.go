// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bluetooth

import (
	"context"

	"github.com/godbus/dbus"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/testing"
)

// Device is the bluz dbus remote device object abstraction.
type Device struct {
	obj  dbus.BusObject
	path dbus.ObjectPath
}

const deviceIface = service + ".Device1"

// Devices creates a Device for each of the remote bluetooth devices in the system.
func Devices(ctx context.Context) ([]*Device, error) {
	var devices []*Device
	_, obj, err := dbusutil.Connect(ctx, service, "/")
	if err != nil {
		return nil, err
	}
	managed, err := dbusutil.ManagedObjects(ctx, obj)
	if err != nil {
		return nil, err
	}
	for _, path := range managed[deviceIface] {
		_, obj, err := dbusutil.Connect(ctx, service, path)
		if err != nil {
			return nil, err
		}
		devices = append(devices, &Device{obj, path})
	}
	return devices, nil
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
		if err == nil {
			return
		}
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

// Path gets the D-Bus path this device was created from.
func (d *Device) Path() dbus.ObjectPath {
	return d.path
}

// Address returns the Address of the bluetooth remote device.
func (d *Device) Address(ctx context.Context) (string, error) {
	const prop = deviceIface + ".Address"
	value, err := dbusutil.Property(ctx, d.obj, prop)
	if err != nil {
		return "", err
	}
	s, ok := value.(string)
	if !ok {
		return "", errors.New("address property not a string")
	}
	return s, nil
}

// Alias returns the alias of the bluetooth remote device.
func (d *Device) Alias(ctx context.Context) (string, error) {
	const prop = deviceIface + ".Alias"
	value, err := dbusutil.Property(ctx, d.obj, prop)
	if err != nil {
		return "", err
	}
	s, ok := value.(string)
	if !ok {
		return "", errors.New("alias property not a string")
	}
	return s, nil
}

// Connected returns true if the bluetooth remote device is connected.
func (d *Device) Connected(ctx context.Context) (bool, error) {
	const prop = deviceIface + ".Connected"
	value, err := dbusutil.Property(ctx, d.obj, prop)
	if err != nil {
		return false, err
	}
	b, ok := value.(bool)
	if !ok {
		return false, errors.New("alias property not a bool")
	}
	return b, nil
}

// Paired returns true if the bluetooth remote device is paired.
func (d *Device) Paired(ctx context.Context) (bool, error) {
	const prop = deviceIface + ".Paired"
	value, err := dbusutil.Property(ctx, d.obj, prop)
	if err != nil {
		return false, err
	}
	b, ok := value.(bool)
	if !ok {
		return false, errors.New("paired property not a bool")
	}
	return b, nil
}

// Connect connects to the bluetooth remote device.
func (d *Device) Connect(ctx context.Context) error {
	const method = deviceIface + ".Connect"
	c := d.obj.CallWithContext(ctx, method, 0)
	if c.Err != nil {
		return errors.Wrap(c.Err, "failed to connect")
	}
	return nil
}

// Disconnect disconnects the bluetooth remote device.
func (d *Device) Disconnect(ctx context.Context) error {
	const method = deviceIface + ".Disconnect"
	c := d.obj.CallWithContext(ctx, method, 0)
	if c.Err != nil {
		return errors.Wrap(c.Err, "failed to disconnect")
	}
	return nil
}

// Pair pairs the bluetooth remote device.
func (d *Device) Pair(ctx context.Context) error {
	const method = deviceIface + ".Pair"
	c := d.obj.CallWithContext(ctx, method, 0)
	if c.Err != nil {
		return errors.Wrap(c.Err, "failed to pair")
	}
	return nil
}
