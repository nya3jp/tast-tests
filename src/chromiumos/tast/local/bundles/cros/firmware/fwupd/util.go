// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package fwupd

import (
	"context"
	"regexp"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/power/setup"
	"chromiumos/tast/testing"
)

// ReleaseURI contains the release URI of the test webcam device in the system.
const ReleaseURI = "https://storage.googleapis.com/chromeos-localmirror/lvfs/test/3fab34cfa1ef97238fb24c5e40a979bc544bb2b0967b863e43e7d58e0d9a923f-fakedevice124.cab"

// ChargingStateTimeout has the time needed for polling battery charging state changes.
const ChargingStateTimeout = 2 * time.Minute

const (
	// This is a string that appears when the computer is discharging.
	dischargeString = `uint32 [0-9]\s+uint32 2`
)

// SetFwupdChargingState sets the battery charging state and polls for
// the appropriate change to be registered by powerd via its dbus
// method.
func SetFwupdChargingState(ctx context.Context, charge bool) (setup.CleanupCallback, error) {
	var localCleanup setup.CleanupCallback

	if charge {
		setup.AllowBatteryCharging(ctx)
		// Return a no-op function to avoid a `cleanup != nil` check for the callers.
		localCleanup = func(ctx context.Context) error {
			return nil
		}
	} else {
		var err error
		if localCleanup, err = setup.SetBatteryDischarge(ctx, 20.0); err != nil {
			return nil, err
		}

		// Local cleanup function in case polling fails below
		defer func() {
			if localCleanup == nil {
				return
			}

			if err := localCleanup(ctx); err != nil {
				testing.ContextLog(ctx, "WARNING Failed to re-enable AC power: ", err)
			}
		}()
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		cmd := testexec.CommandContext(ctx, "dbus-send", "--print-reply", "--system", "--type=method_call",
			"--dest=org.chromium.PowerManager", "/org/chromium/PowerManager",
			"org.chromium.PowerManager.GetBatteryState")
		output, err := cmd.Output(testexec.DumpLogOnError)
		if err != nil {
			return err
		}
		discharging, err := regexp.Match(dischargeString, output)
		if charge == discharging {
			cmd := testexec.CommandContext(ctx, "stressapptest", "-s", "30")
			if err := cmd.Run(); err != nil {
				return err
			}
		}
		if charge == discharging || err != nil {
			return errors.New("powerd has not registered a battery state change")
		}
		return nil
	}, &testing.PollOptions{Timeout: ChargingStateTimeout}); err != nil {
		return nil, errors.Wrap(err, "battery polling was unsuccessful")
	}

	retCleanup := localCleanup
	// Disable the local cleanup function above
	localCleanup = nil

	return retCleanup, nil
}
