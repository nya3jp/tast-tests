// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bluetooth

import (
	"context"
	"strconv"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bluetooth"
	"chromiumos/tast/testing"
)

// Enable enables Bluetooth.
func Enable(ctx context.Context) error {
	testing.ContextLog(ctx, "enables bluetooth")
	return setBluetoothPowered(ctx, true)
}

// Disable disables bluetooth.
func Disable(ctx context.Context) error {
	testing.ContextLog(ctx, "disables bluetooth")
	return setBluetoothPowered(ctx, false)
}

// IsEnabled checks bluetooth is enabled.
func IsEnabled(ctx context.Context) (bool, error) {
	adapters, err := bluetooth.Adapters(ctx)
	if err != nil {
		return false, errors.Wrap(err, "unable to get Bluetooth adapters")
	}
	if len(adapters) != 1 {
		return false, errors.Errorf("got %d adapters, expected 1 adapter", len(adapters))
	}
	return adapters[0].Powered(ctx)
}

// ConnectDevice connects to a Bluetooth device by the given name.
func ConnectDevice(ctx context.Context, deviceName string) error {
	if err := setBluetoothPowered(ctx, true); err != nil {
		return err
	}
	return setDevice(ctx, deviceName, true)
}

// DisconnectDevice disconnects the Bluetooth device by the given name.
func DisconnectDevice(ctx context.Context, deviceName string) error {
	if isEnabled, err := IsEnabled(ctx); err != nil {
		return errors.Wrap(err, "failed to get bluetooth status")
	} else if !isEnabled {
		return nil
	}
	return setDevice(ctx, deviceName, false)
}

func setDevice(ctx context.Context, deviceName string, connect bool) error {
	device, err := bluetooth.DeviceByAlias(ctx, deviceName)
	if err != nil {
		return errors.Wrapf(err, "failed to get Bluetooth device(%q)", deviceName)
	}

	if connected, err := device.Connected(ctx); err != nil {
		return errors.Wrap(err, "could not get Bluetooth connection state")
	} else if connected == connect {
		return nil
	}
	if connect {
		if err := device.Connect(ctx); err != nil {
			return errors.Wrapf(err, "failed to connect to bluetooth device(%q)", deviceName)
		}
	} else {
		if err := device.Disconnect(ctx); err != nil {
			return errors.Wrapf(err, "failed to disconnect to bluetooth device(%q)", deviceName)
		}
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if connected, err := device.Connected(ctx); err != nil {
			return errors.Wrap(err, "could not get Bluetooth connection state")
		} else if connected != connect {
			return errors.Errorf("Bluetooth device connection state not changed to %s after setting", strconv.FormatBool(connect))
		}
		return nil
	}, &testing.PollOptions{
		Timeout:  10 * time.Second,
		Interval: 100 * time.Millisecond,
	}); err != nil {
		return err
	}
	return nil
}

func setBluetoothPowered(ctx context.Context, powered bool) error {
	adapters, err := bluetooth.Adapters(ctx)
	if err != nil {
		return errors.Wrap(err, "unable to get Bluetooth adapters")
	}

	if len(adapters) != 1 {
		return errors.Errorf("got %d adapters, expected 1 adapter", len(adapters))
	}
	if err := adapters[0].SetPowered(ctx, powered); err != nil {
		return errors.Wrap(err, "could not set Bluetooth power state")
	}

	// Poll until the adapter state has been changed to the correct value.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if res, err := adapters[0].Powered(ctx); err != nil {
			return errors.Wrap(err, "could not get Bluetooth power state")
		} else if res != powered {
			return errors.Errorf("Bluetooth adapter state not changed to %s after toggle", strconv.FormatBool(powered))
		}
		return nil
	}, &testing.PollOptions{
		Timeout:  10 * time.Second,
		Interval: time.Second,
	}); err != nil {
		return err
	}
	return nil
}
