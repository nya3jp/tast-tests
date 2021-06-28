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

// TODO: Errors needds to be handles across as per cbb
// Add here SIM pin functions
// Device function names.
// Possible Errors: [service].Error.InvalidArguments
// [service].Error.NotSupported
// [service].Error.PinError

// In the case of PinError, the error message gives
// more detail: [interface].PinRequired
// [interface].PinBlocked
// [interface].IncorrectPin

// (Cellular only) Enable or disable PIN protection for
// a cellular modem's SIM card. If 'require' is true,
// then a PIN will need to be supplied (by calling
// EnterPin) before the modem is usable. If 'require'
// is false, a PIN will not be required.

// RequirePin enables/disables SIM PIN based on require value
func (d *Device) RequirePin(ctx context.Context, pin string, require bool) error {
	if err := d.Call(ctx, "RequirePin", pin).Err; err != nil {
		err = parseSIMLockError(err)
		return errors.Wrapf(err, "failed to enter pin %s", d.String())
	}
	return nil
}

// EnterPin is to lock/unlock a SIM card with given PIN. TODO: can be wrapped in LockSIM & UnlockSIM in cbb
func (d *Device) EnterPin(ctx context.Context, pin string) error {
	if err := d.Call(ctx, "EnterPin", pin).Err; err != nil {
		err = parseSIMLockError(err)
		return errors.Wrapf(err, "failed to enter pin %s", d.String())
	}
	return nil
}

// Provide a PUK code to unblock a PIN.
// When an incorrect PIN has been entered too many times
// (three is generally the number of tries allowed), the
// PIN becomes "blocked", and the SIM card can only be
// unlocked by providing a PUK code provided by the
// carrier. At the same time, a new PIN must be specified.
func (d *Device) UnblockPUK(ctx context.Context, pukCode string, newPin string) error {
	if err := d.Call(ctx, "UnblockPin", pukCode, newPin).Err; err != nil {
		err = parseSIMLockError(err)
		return errors.Wrapf(err, "failed to unblock SIM using PUK code due to error %s", d.String())
	}
	return nil
}

// ChangePin changes the PIN used to unlock a SIM card
// The existing PIN must be provided along with the new PIN.
func (d *Device) ChangePin(ctx context.Context, oldPin string, newPin string) error {
	if err := d.Call(ctx, "ChangePin", oldPin, newPin).Err; err != nil {
		err = parseSIMLockError(err)
		return errors.Wrapf(err, "failed to change pin %s", d.String())
	}
	return nil
}

// IsSimLockEnabled returns lockenabled value
func (d *Device) IsSimLockEnabled(ctx context.Context) (bool, error) {
	lockStatus = GetSimLockStatus(ctx)
	lockEnabled := lockStatus["LockEnabled"].Value()
	return lockEnabled, err
}

// IsSimPinLocked returns true if locktype value is sim-pin
func (d *Device) IsSimPinLocked(ctx context.Context) (bool, error) {
	lockStatus = GetSimLockStatus(ctx)
	lockType := lockStatus["LockType"].Value()
	return lockType == shillconst.DevicePropertyValueSIMLockTypePIN, err
}

// IsSimPukLocked returns true if locktype value is sim-puk
func (d *Device) IsSimPukLocked(ctx context.Context) (bool, error) {
	lockStatus, err = GetSimLockStatus(ctx)
	lockType := lockStatus["LockType"].Value()
	return lockType == shillconst.DevicePropertyValueSIMLockTypePUK, err
}

// GetRetriesLeft helps to get modem property UnlockRetries value
func (d *Device) GetRetriesLeft(ctx context.Context) (int, error) {
	lockStatus = GetSimLockStatus(ctx)
	retriesLeft := lockStatus["RetriesLeft"].Value()
	if retriesLeft == nil {
		return errors.Wrap("failed to get RetriesLeft")
	}
	if retriesLeft == none {
		return nil, errors.Wrap("missing RetriesLeft property")
	}
	if retriesLeft < 0 {
		return nil, errors.Wrapf("malformed RetriesLeft: %d", retries_left)
	}
	return retriesLeft, err
}

// GetSimLockStatus dict gets Cellular.SIMLockStatus dictionary
func (d *Device) GetSimLockStatus(ctx context.Context) ([]map[string]dbus.Variant, error) {
	// Gather Shill Device properties
	deviceProps, err := d.GetShillProperties(ctx)
	if err != nil {
		s.Fatal("Failed to get Device properties: ", err)
	}

	// Verify Device.SimSlots.
	info, err := deviceProps.Get(shillconst.DevicePropertyCellularSIMLockStatus)
	if err != nil {
		err = parseSIMLockError(err)
		s.Fatal("Failed to get Device.CellularSIMLockStatus property: ", err)
	}
	simLockStatus, ok := info.([]map[string]dbus.Variant)
	if !ok {
		s.Fatal("Invalid format for Device.CellularSIMLockStatus")
	}
	return simLockStatus, err
}

// Helper functions for SIM lock/unlock

// random generates a random integer
func random(min, max int) int {
	return rand.Intn(max-min) + min
}

// BadPin obtains a pin that does not match the valid sim-pin.
func (d *Device) BadPin(ctx context.Context, currentPin int) (int, error) {
	pin := random(1000, 9999)
	if pin == currentPin {
		pin += 1
	}
	return pin, nil
}

// BadPuk obtains a puk that does not match the valid sim-puk.
func (d *Device) BadPuk(ctx context.Context, currentPuk int) (int, error) {
	puk := random(10000000, 99999999)
	if puk == currentPuk {
		puk += 1
	}
	return puk, nil
}

// PinLockSim is a helper method to pin-lock a SIM, assuming nothing bad happens.
func (d *Device) PinLockSim(ctx context.Context, newPin int) error {
	if err := d.RequirePin(newPin, true); err != nil {
		return errors.Wrap(err, "failed to enable with new pin")
	}
	return nil
}

// PukLockSim is a helper method to puk-lock a SIM, assuming nothing bad happens.
func (d *Device) PukLockSim(ctx context.Context, currentPin int) error {
	if err := PinLockSim(ctx, currentPin); err != nil {
		return errors.Wrap(err, "failed at puklocksim")
	}
	locked := false
	for !locked {
		locked, err = d.IsSimPukLocked(ctx)
		if locked == true {
			break
		}
		err := EnterIncorrectPin(ctx, currentPin)
		if err == "PIN Blocked Error" {
			return errors.Wrapf("sim could not get blocked", err)
		}
	}
	if !d.IsSimPukLocked(ctx) {
		return errors.Wrap("expected SIM to be puk-locked")
	}
}

// EnterIncorrectPin checks expected error for bad pin given
func (d *Device) EnterIncorrectPin(ctx context.Context, currentPin int) error {
	badPin, err := d.BadPin(ctx, currentPin)
	if err != nil {
		return errors.Wrap(err, "failed to generate bad pin")
	}
	if err = d.EnterPin(badPin); err == nil {
		return errors.Wrap(err, "failed to send bad pin")
	}
	// errorIncorrectPin used to do graceful exit for expected bad pin error
	// TODO: ERROR_INCORRECT_PIN = 'org.freedesktop.ModemManager1.Sim.Error.IncorrectPin'
	errorIncorrectPin := errors.New("org.freedesktop.ModemManager1.Sim.Error.IncorrectPin")

	if err == errorIncorrectPin {
		return nil
	}
	return errors.Wrap(err, "unusual pin error")
}

// EnterIncorrectPuk checks expected error for bad puk given
func (d *Device) EnterIncorrectPuk(ctx context.Context, currentPuk int) error {
	badPuk, err := d.BadPuk(ctx, currentPuk)
	if err != nil {
		return errors.Wrap(err, "failed to generate bad puk")
	}
	if err = d.UnblockPUK(badPuk); err == nil {
		return errors.Wrap(err, "failed to send bad puk")
	}
	// errorIncorrectPuk used to do graceful exit for expected bad puk error
	// TODO: ERROR_INCORRECT_PUK = 'org.freedesktop.ModemManager1.Sim.Error.IncorrectPuk'
	errorIncorrectPuk := errors.New("org.freedesktop.ModemManager1.Sim.Error.IncorrectPuk")
	if err == errorIncorrectPuk {
		return nil
	}
	return errors.Wrap(err, "unusual puk error")
}
