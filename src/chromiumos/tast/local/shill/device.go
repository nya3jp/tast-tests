// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package shill

import (
	"context"
	"math/rand"
	"time"

	"github.com/godbus/dbus"

	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

const (
	dbusDeviceInterface = "org.chromium.flimflam.Device"
)

// Device wraps a Device D-Bus object in shill.
type Device struct {
	*PropertyHolder
}

// NewDevice connects to shill's Device.
// It also obtains properties after device creation.
func NewDevice(ctx context.Context, path dbus.ObjectPath) (*Device, error) {
	ph, err := NewPropertyHolder(ctx, dbusService, dbusDeviceInterface, path)
	if err != nil {
		return nil, err
	}
	return &Device{PropertyHolder: ph}, nil
}

// SetUsbEthernetMacAddressSource sets USB Ethernet MAC address source for the device.
func (d *Device) SetUsbEthernetMacAddressSource(ctx context.Context, source string) error {
	if err := d.Call(ctx, "SetUsbEthernetMacAddressSource", source).Err; err != nil {
		return errors.Wrap(err, "failed set USB Ethernet MAC address source")
	}
	return nil
}

// Enable enables the device.
func (d *Device) Enable(ctx context.Context) error {
	if err := d.Call(ctx, "Enable").Err; err != nil {
		return errors.Wrapf(err, "failed to enable device %s", d.String())
	}
	return nil
}

// Disable disables the device.
func (d *Device) Disable(ctx context.Context) error {
	if err := d.Call(ctx, "Disable").Err; err != nil {
		return errors.Wrapf(err, "failed to disable device %s", d.String())
	}
	return nil
}

// RequestRoam requests that we roam to the specified BSSID.
// Note: this operation assumes that:
// 1- We are connected to an SSID for which |bssid| is a member.
// 2- There is a BSS with an appropriate ID in our scan results.
func (d *Device) RequestRoam(ctx context.Context, bssid string) error {
	if err := d.Call(ctx, "RequestRoam", bssid).Err; err != nil {
		return errors.Wrapf(err, "failed to roam %s", d.String())
	}
	return nil
}

// WaitForSelectedService returns the first valid value (i.e., not "/") of the
// "SelectedService" property.
func (d *Device) WaitForSelectedService(ctx context.Context, timeout time.Duration) (dbus.ObjectPath, error) {
	var servicePath dbus.ObjectPath
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		deviceProp, err := d.GetProperties(ctx)
		if err != nil {
			return testing.PollBreak(errors.Wrapf(err, "failed to get properties of device %v", d))
		}
		servicePath, err = deviceProp.GetObjectPath(shillconst.DevicePropertySelectedService)
		if err != nil {
			return testing.PollBreak(errors.Wrapf(err, "failed to get the DBus object path for the property %s", shillconst.DevicePropertySelectedService))
		}
		if servicePath == "/" {
			return errors.Wrapf(err, "%s is invalid", shillconst.DevicePropertySelectedService)
		}
		return nil
	}, &testing.PollOptions{Timeout: timeout}); err != nil {
		return "/", err
	}
	return servicePath, nil
}
