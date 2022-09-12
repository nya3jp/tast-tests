// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package floss provides a Floss implementation of the Bluetooth interface.
package floss

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// TODO(chharry): The default adapter is hci0 until we support multiple adapters.
const defaultAdapter = 0

// Enable starts the default adapter.
func Enable(ctx context.Context) error {
	mgr, err := newManagerDBusObject(ctx)
	if err != nil {
		return err
	}

	c := mgr.Call(ctx, "Start", defaultAdapter)
	if c.Err != nil {
		return errors.Wrapf(c.Err, "failed to start hci%d", defaultAdapter)
	}
	return nil
}

// IsEnabled returns the power state of the default adapter.
func IsEnabled(ctx context.Context) (bool, error) {
	mgr, err := newManagerDBusObject(ctx)
	if err != nil {
		return false, err
	}

	c := mgr.Call(ctx, "GetAdapterEnabled", defaultAdapter)
	if c.Err != nil {
		return false, errors.Wrapf(c.Err, "failed to get enabled of hci%d", defaultAdapter)
	}

	var enabled bool
	if err := c.Store(&enabled); err != nil {
		return false, errors.Wrap(err, "failed to store GetAdapterEnabled response")
	}

	return enabled, nil
}

// PollForAdapterState polls the bluetooth adapter state until expected state is received or a timeout occurs.
func PollForAdapterState(ctx context.Context, exp bool) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		status, err := IsEnabled(ctx)
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to check Bluetooth status"))
		}
		if status != exp {
			return errors.Errorf("failed to verify Bluetooth status, got %t, want %t", status, exp)
		}

		return nil

	}, &testing.PollOptions{Timeout: 10 * time.Second, Interval: time.Second})
}

// PollForEnabled polls the bluetooth adapter state until the adapter is enabled.
func PollForEnabled(ctx context.Context) error {
	return PollForAdapterState(ctx, true)
}
