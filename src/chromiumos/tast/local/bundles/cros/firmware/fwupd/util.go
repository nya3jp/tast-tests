// Copyright 2021 The ChromiumOS Authors
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
// It takes Brya about 3 minutes for the state to change from fully charged to discharging.
const ChargingStateTimeout = 10 * time.Minute

const (
	// This is a string that appears when the computer is discharging.
	dischargeString = `uint32 [0-9]\s+uint32 2`
)

// SetFwupdChargingState sets the battery charging state and polls for
// the appropriate change to be registered by powerd via its dbus
// method.
func SetFwupdChargingState(ctx context.Context, charge bool) (setup.CleanupCallback, error) {
	var localCleanup setup.CleanupCallback

	// Local cleanup function in case polling fails below
	defer func() {
		if localCleanup == nil {
			return
		}

		if err := localCleanup(ctx); err != nil {
			testing.ContextLog(ctx, "WARNING Failed to re-enable AC power: ", err)
		}
	}()

	if charge {
		if err := setup.AllowBatteryCharging(ctx); err != nil {
			return nil, err
		}

		// Return a no-op function to avoid a `cleanup != nil` check for the callers.
		localCleanup = func(ctx context.Context) error {
			return nil
		}
	} else {
		var err error
		if localCleanup, err = setup.SetBatteryDischarge(ctx, 20.0); err != nil {
			return nil, err
		}
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		// fwupd is checking for the battery state to signal `discharging` instead
		// of checking if the AC power is disconnected. Some batteries won't
		// immediately change their state to `discharging` once the AC is
		// disconnected. Instead they will remain in the `fully charged` state
		// until the battery has discharged past some unknown threshold.
		// This call is here to force the battery to discharge enough so the
		// battery state changes. Ideally fwupd would use the presence of AC
		// instead of the battery state. If it did, we could them remove this
		// workaround.
		if !charge {
			cmd := testexec.CommandContext(ctx, "stressapptest", "-s", "5")
			testing.ContextLog(ctx, "Draining battery using: ", cmd)
			if err := cmd.Run(); err != nil {
				return err
			}
		}

		cmd := testexec.CommandContext(ctx, "dbus-send", "--print-reply", "--system", "--type=method_call",
			"--dest=org.chromium.PowerManager", "/org/chromium/PowerManager",
			"org.chromium.PowerManager.GetBatteryState")
		output, err := cmd.Output(testexec.DumpLogOnError)
		if err != nil {
			return err
		}

		if discharging, err := regexp.Match(dischargeString, output); err != nil {
			return err
		} else if (charge && !discharging) || (!charge && discharging) {
			return nil
		}

		return errors.New("powerd has not registered a battery state change")
	}, &testing.PollOptions{Timeout: ChargingStateTimeout}); err != nil {
		return nil, errors.Wrap(err, "battery polling was unsuccessful")
	}

	retCleanup := localCleanup
	// Disable the local cleanup function above
	localCleanup = nil

	return retCleanup, nil
}
