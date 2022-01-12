// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package mojo

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

/*
 The following data structure are defined in cros_bluetooth_config.mojom
 https://chromium.googlesource.com/chromium/src/+/refs/heads/main/chromeos/services/bluetooth_config/public/mojom/cros_bluetooth_config.mojom
*/

// BluetoothSystemState represents the state of Bluetooth on the device.
type BluetoothSystemState int32

// BluetoothModificationState represents the state of whether Bluetooth can be modified.
type BluetoothModificationState int32

const (
	// Unavailable means device does not have access to Bluetooth.
	Unavailable BluetoothSystemState = iota
	// Disabled means Bluetooth is turned off.
	Disabled
	// Disabling means Bluetooth is in the process of turning off.
	Disabling
	// Enabled means Bluetooth is turned on.
	Enabled BluetoothSystemState = 3
	// Enabling means Bluetooth is in the process of turning on.
	Enabling
)

const ( // CannotModifyBluetooth means Bluetooth cannot be turned on/off, and devices cannot be connected. E.g.,
	// the current session may belong to a secondary user, or the screen is locked.
	CannotModifyBluetooth BluetoothModificationState = iota

	// CanModifyBluetooth means Bluetooth settings can be modified as part of the current session.
	CanModifyBluetooth
)

// BluetoothSystemProperties describes the high-level status of system Bluetooth.
type BluetoothSystemProperties struct {
	SystemState       BluetoothSystemState
	ModificationState BluetoothModificationState
}

/*
   Helper functions to get javascript values
*/

func getSystemProperties(ctx context.Context, btmojo chrome.JSObject) (BluetoothSystemProperties, error) {
	var r BluetoothSystemProperties
	js := `function() {return this.systemProperties_;}`
	if err := btmojo.Call(ctx, &r, js); err != nil {
		return r, errors.Wrap(err, "getSystemProperties call failed")
	}
	return r, nil
}

/*
   Wrapper functions around Bluetooth mojo JS calls
*/

// SetBluetoothEnabledState sets Bluetooth state to on/off
func SetBluetoothEnabledState(ctx context.Context, btmojo chrome.JSObject, enabled bool) error {
	if err := btmojo.Call(ctx, nil, `function(enabled) {this.bluetoothConfig.setBluetoothEnabledState(enabled)}`, enabled); err != nil {
		return errors.Wrap(err, "setBluetoothEnabledState call failed")
	}
	return nil
}

// PollForBluetoothSystemState polls BluetoothSystemProperties until expected SystemState is received or  timeout occurs.
func PollForBluetoothSystemState(ctx context.Context, btmojo chrome.JSObject, exp BluetoothSystemState) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		prop, err := getSystemProperties(ctx, btmojo)
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to get Bluetooth system properties"))
		}
		if prop.SystemState != exp {
			return errors.Errorf("failed to verify Bluetooth system state, got %+v, want %d", prop, exp)
		}

		return nil

	}, &testing.PollOptions{Timeout: 10 * time.Second, Interval: time.Second})
}
