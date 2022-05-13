// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package shill

import (
	"context"
	"time"

	"github.com/godbus/dbus/v5"

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

// (Cellular only) Enable or disable PIN protection for a cellular modem's SIM
// card. If 'require' is true, then a PIN will need to be supplied (by calling
// EnterPin) before the modem is usable. If 'require' is false, a PIN will not be required.

// RequirePin enables/disables SIM PIN based on required value.
func (d *Device) RequirePin(ctx context.Context, pin string, require bool) error {
	if err := d.Call(ctx, "RequirePin", pin, require).Err; err != nil {
		return errors.Wrapf(err, "failed to enter pin %s", d.String())
	}
	return nil
}

// EnterPin is to lock/unlock a SIM card with given PIN.
func (d *Device) EnterPin(ctx context.Context, pin string) error {
	if err := d.Call(ctx, "EnterPin", pin).Err; err != nil {
		return errors.Wrapf(err, "failed to enter pin %s", d.String())
	}
	return nil
}

// When an incorrect PIN has been entered too many times (three is generally the
// number of tries allowed), the PIN becomes "blocked", and the SIM card can only
// be unlocked by providing a PUK code provided by the carrier. At the same time,
// a new PIN must be specified.

// UnblockPin used to unblock the locked sim with puk code.
func (d *Device) UnblockPin(ctx context.Context, pukCode, newPin string) error {
	if err := d.Call(ctx, "UnblockPin", pukCode, newPin).Err; err != nil {
		return errors.Wrapf(err, "failed to unblock SIM using PUK code due to error %s", d.String())
	}
	return nil
}

// ChangePin changes the PIN used to unlock a SIM card, The existing PIN must be
// provided along with the new PIN.
func (d *Device) ChangePin(ctx context.Context, oldPin, newPin string) error {
	if err := d.Call(ctx, "ChangePin", oldPin, newPin).Err; err != nil {
		return errors.Wrapf(err, "failed to change pin %s", d.String())
	}
	return nil
}

// Reset resets underlying modem.
func (d *Device) Reset(ctx context.Context) error {
	if err := d.Call(ctx, "Reset").Err; err != nil {
		return errors.Wrapf(err, "failed to reset modem %s", d.String())
	}
	return nil
}
