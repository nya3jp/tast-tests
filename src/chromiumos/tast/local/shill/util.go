// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// This file provides small helpers, packaging shill APIs in ways to ease their
// use by others.

package shill

import (
	"context"
	"time"

	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// Turn device offline will also disrupt ssh heartbeat.
// As a consequence, this function needs to return within certain time duration.
const maxOfflineDurationS = 60 * time.Second

// WaitForOnline waits for Internet connectivity, a shorthand which is useful so external packages don't have to worry
// about Shill details (e.g., Service, Manager). Tests that require Internet connectivity (e.g., for a real GAIA login)
// need to ensure that before trying to perform Internet requests. This function is one way to do that.
// Returns an error if we don't come back online within a reasonable amount of time.
func WaitForOnline(ctx context.Context) error {
	m, err := NewManager(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to connect to shill's Manager")
	}

	expectProps := map[string]interface{}{
		shillconst.ServicePropertyState: shillconst.ServiceStateOnline,
	}
	if _, err := m.WaitForServiceProperties(ctx, expectProps, 15*time.Second); err != nil {
		return errors.Wrap(err, "network did not come back online")
	}

	return nil
}

// ExecFuncOffline disables all powered devices to turn the device to completely offline,
// runs given function and then reverts back to the original state.
func ExecFuncOffline(ctx context.Context, f func(offlineCtx context.Context) error) error {
	ctx, cancel := context.WithTimeout(ctx, maxOfflineDurationS)
	defer cancel()

	m, err := NewManager(ctx)
	if err != nil {
		return errors.Wrap(err, "failed creating shill manager proxy")
	}

	devices, _ := m.Devices(ctx)
	for _, device := range devices {
		properties, err := device.GetProperties(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get device properties")
		}

		isDevicePowered, err := properties.GetBool(shillconst.DevicePropertyPowered)
		if err != nil {
			return errors.Wrapf(err, "failed to get power property of device: %s", device)
		}

		if isDevicePowered {
			testing.ContextLogf(ctx, "Disabling device: %s", device)
			if err := device.Disable(ctx); err != nil {
				return errors.Wrapf(err, "failed to disable device: %s", device)
			}
			defer func() error {
				testing.ContextLogf(ctx, "Re-enabling device: %s", device)
				if err := device.Enable(ctx); err != nil {
					return err
				}
				return nil
			}()
		}
	}

	return f(ctx)
}
