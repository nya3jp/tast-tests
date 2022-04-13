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
const ChargingStateTimeout = 29 * time.Minute

const (
	// This is a string that appears when the computer is discharging.
	dischargeString = `uint32 [0-9]\s+uint32 2`
)

func setBatteryNormal(ctx context.Context) error {
	if err := testexec.CommandContext(ctx, "ectool", "chargecontrol", "normal").Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "ectool chargecontrol failed")
	}
	return nil
}

// SetFwupdChargingState sets the battery charging state and polls for
// the appropriate change to be registered by powerd via its dbus
// method.
func SetFwupdChargingState(ctx context.Context, charge bool) error {
	if charge {
		if err := setBatteryNormal(ctx); err != nil {
			return err
		}
	} else {
		if _, err := setup.SetBatteryDischarge(ctx, 20.0); err != nil {
			return err
		}
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		cmd := testexec.CommandContext(ctx, "dbus-send", "--print-reply", "--system", "--type=method_call",
			"--dest=org.chromium.PowerManager", "/org/chromium/PowerManager",
			"org.chromium.PowerManager.GetBatteryState")
		output, err := cmd.Output(testexec.DumpLogOnError)
		if err != nil {
			return err
		}
		matched, err := regexp.Match(dischargeString, output)
		if charge == matched {
			cmd := testexec.CommandContext(ctx, "stressapptest", "-s", "30")
			if err := cmd.Run(); err != nil {
				return err
			}
		}
		if charge == matched || err != nil {
			return errors.New("powerd has not registered a battery state change")
		}
		return nil
	}, &testing.PollOptions{Timeout: ChargingStateTimeout}); err != nil {
		return errors.Wrap(err, "battery polling was unsuccessful")
	}

	return nil
}
