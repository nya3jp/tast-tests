// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
)

// TODO: Errors needs to be handles across as per cbb
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
	lockStatus = GetCellularSIMLockStatus(ctx)
	lockEnabled := lockStatus["LockEnabled"].Value()
	return lockEnabled, err
}

// IsSimPinLocked returns true if locktype value is sim-pin
func (d *Device) IsSimPinLocked(ctx context.Context) (bool, error) {
	lockStatus = GetCellularSIMLockStatus(ctx)
	lockType := lockStatus["LockType"].Value()
	return lockType == shillconst.DevicePropertyValueSIMLockTypePIN, err
}

// IsSimPukLocked returns true if locktype value is sim-puk
func (d *Device) IsSimPukLocked(ctx context.Context) (bool, error) {
	lockStatus, err = GetCellularSIMLockStatus(ctx)
	lockType := lockStatus["LockType"].Value()
	return lockType == shillconst.DevicePropertyValueSIMLockTypePUK, err
}

// GetRetriesLeft helps to get modem property UnlockRetries value
func (d *Device) GetRetriesLeft(ctx context.Context) (int, error) {
	lockStatus = GetCellularSIMLockStatus(ctx)
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

// ClearSIMLock clears puk, pin lock if lockenabled
func (d *Device) ClearSIMLock(ctx context.Context, pin string, puk string) error {
	if IsSimLockEnabled(ctx) {
		// clear puk lock
		if d.IsSimPukLocked(ctx) {
			err = d.UnblockPUK(puk, pin)
		}
		// clear pin lock
		if d.IsSimPinLocked(ctx) {
			err = d.EnterPin(pin)
			if err == shillconst.ErrorIncorrectPin {
				// Do max unlock tries and do puk unlock
				err = d.PukLockSim(ctx, pin)
				if err != nil {
					return errors.Wrapf(err, "failed to puklockSIM with pin in ClearSIMLock")
				}
				err = d.UnblockPUK(puk, pin)
				if err != nil {
					return errors.Wrapf(err, "failed to clear with UnblockPUK")
				}
				err = d.EnterPin(pin)
				if err != nil {
					return errors.Wrapf(err, "failed to clear pin lock with EnterPin")
				}
			}
		}
		// disable sim lock
		err = d.RequirePin(ctx, pin, false)
	}
	return err
}

// GetCellularSIMLockStatus dict gets Cellular.SIMLockStatus dictionary
func (d *Device) GetCellularSIMLockStatus(ctx context.Context) ([]map[string]dbus.Variant, error) {
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
func (d *Device) BadPin(ctx context.Context, currentPin string) (string, error) {
	pin := random(1000, 9999)
	if pin == Atoi(currentPin) {
		pin += 1
	}
	return Itoa(pin), nil
}

// BadPuk obtains a puk that does not match the valid sim-puk.
func (d *Device) BadPuk(ctx context.Context, currentPuk string) (string, error) {
	puk := random(10000000, 99999999)
	if puk == Atoi(currentPuk) {
		puk += 1
	}
	return Itoa(puk), nil
}

// PinLockSim is a helper method to pin-lock a SIM, assuming nothing bad happens.
func (d *Device) PinLockSim(ctx context.Context, newPin string) error {
	if err := d.RequirePin(Itoa(newPin), true); err != nil {
		return errors.Wrap(err, "failed to enable with new pin")
	}
	return nil
}

// PukLockSim is a helper method to puk-lock a SIM, assuming nothing bad happens.
func (d *Device) PukLockSim(ctx context.Context, currentPin string) error {
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
func (d *Device) EnterIncorrectPin(ctx context.Context, currentPin string) error {
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
func (d *Device) EnterIncorrectPuk(ctx context.Context, currentPuk string) error {
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
