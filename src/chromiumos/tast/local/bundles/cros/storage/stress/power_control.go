// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package stress

import (
	"context"
	"strconv"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// PowerCommandType defines a type for a power command to perform.
type PowerCommandType int

const (
	// Suspend suspends the device for a pre-defined time and then resumes it.
	Suspend PowerCommandType = iota
	// Reboot performs device reboot and waits for it to load back.
	// Note:
	Reboot
)

const (
	defaultSuspendTimeout    = 20 * time.Second
	defaultSuspendWakeupTime = 30 * time.Second
	defaultRebootTimeout     = 2 * time.Minute
)

func suspend(ctx context.Context, timeout, wakeup time.Duration) (err error) {
	testing.ContextLog(ctx, "Performing DUT suspend and wakeup after: ", durationAsString(wakeup))
	err = testexec.CommandContext(ctx, "powerd_dbus_suspend",
		"--timeout="+durationAsString(timeout),
		"--wakeup_timeout="+durationAsString(wakeup)).Run()
	if err != nil {
		testing.ContextLog(ctx, "Failed suspending DUT: ", err)

		// Device might still be trying to suspend, so need to restart.
		restartPowerd(ctx)

		return errors.Wrap(err, "failed to suspend device")
	}

	return nil
}

func restartPowerd(ctx context.Context) {
	testing.ContextLog(ctx, "Restarting powerd")
	err := testexec.CommandContext(ctx, "restart", "powerd").Run()
	if err != nil {
		testing.ContextLog(ctx, "Failed restarting DUT: ", err)
		// If we fail to restart powerd, just reboot.
		reboot(ctx)
	}
}

// reboot performs client-side DUT reboot
// Client-side reboot should be discouraged, this method is here for DUT recovery purposes only.
func reboot(ctx context.Context) (err error) {
	testing.ContextLog(ctx, "Rebooting")
	testexec.CommandContext(ctx, "reboot").Run()
	if err != nil {
		testing.ContextLog(ctx, "Failed restarting DUT: ", err)
	}

	return nil
}

func durationAsString(dur time.Duration) string {
	return strconv.Itoa(int(dur / time.Second))
}

// PerformPowerCommand performs a DUT power control command such as suspend/resume or reboot.
func PerformPowerCommand(ctx context.Context, command PowerCommandType) (err error) {
	switch command {
	case Suspend:
		err = suspend(ctx, defaultSuspendTimeout, defaultSuspendWakeupTime)
	case Reboot:
		err = reboot(ctx)
	default:
		return errors.Errorf("Power command %v is not supported", command)
	}
	return err
}
