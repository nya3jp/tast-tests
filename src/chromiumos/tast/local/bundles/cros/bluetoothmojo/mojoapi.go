// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bluetoothmojo

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

// Data structure defined in cros_bluetooth_config.mojom

type BluetoothSystemState int32
type BluetoothModificationState int32

const (
	KUnavailable BluetoothSystemState = 0
	KDisabled    BluetoothSystemState = 1
	KDisabling   BluetoothSystemState = 2
	KEnabled     BluetoothSystemState = 3
	KEnabling    BluetoothSystemState = 4

	KCannotModifyBluetooth BluetoothModificationState = 0
	KCanModifyBluetooth    BluetoothModificationState = 1
)

type BluetoothSystemProperties struct {
	SystemState        BluetoothSystemState
	Modification_state BluetoothModificationState
}

// Helper functions to get javascript values

func getSystemProperties(ctx context.Context, s *testing.State, btmojo chrome.JSObject) (BluetoothSystemProperties, error) {
	var r BluetoothSystemProperties
	js := fmt.Sprintf(`function() {return this.systemProperties_;}`)
	if err := btmojo.Call(ctx, &r, js); err != nil {
		return r, errors.Wrap(err, "getSystemProperties call failed")
	}
	return r, nil
}

// Wrapper functions around Bluetooth mojo JS calls

//SetBluetoothEnabledState sets Bluetooth state to on/off
func SetBluetoothEnabledState(ctx context.Context, s *testing.State, btmojo chrome.JSObject, enabled bool) error {
	js := fmt.Sprintf(`function() {this.bluetoothConfig.setBluetoothEnabledState(%t)}`, enabled)
	if err := btmojo.Call(ctx, nil, js); err != nil {
		return errors.Wrap(err, "setBluetoothEnabledState call failed")
	}
	return nil
}

//PollForAdapterState polls bluetooth adapter state until expected state is received or  timeout occurs.
func PollForBluetoothSystemState(ctx context.Context, s *testing.State, m chrome.JSObject, exp BluetoothSystemState) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		prop, err := getSystemProperties(ctx, s, m)
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to get Bluetooth system properties"))
		}
		if prop.SystemState != exp {
			return errors.Errorf("failed to verify Bluetooth system state, got %+v, want %d", prop, exp)
		}

		return nil

	}, &testing.PollOptions{Timeout: 10 * time.Second, Interval: time.Second})
}
