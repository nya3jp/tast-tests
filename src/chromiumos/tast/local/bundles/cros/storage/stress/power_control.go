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

const (
	defaultSuspendTimeout    = 20 * time.Second
	defaultSuspendWakeupTime = 30 * time.Second
	defaultRebootTimeout     = 2 * time.Minute
)

// Suspend suspends the device for a pre-defined time and then resumes it.
func Suspend(ctx context.Context) error {
	return suspend(ctx, defaultSuspendTimeout, defaultSuspendWakeupTime)
}

func suspend(ctx context.Context, timeout, wakeup time.Duration) error {
	inSec := func(dur time.Duration) string {
		return strconv.Itoa(int(dur / time.Second))
	}

	testing.ContextLog(ctx, "Suspending DUT, wakeup = ", wakeup)
	err := testexec.CommandContext(ctx, "powerd_dbus_suspend",
		"--timeout="+inSec(timeout),
		"--wakeup_timeout="+inSec(wakeup)).Run(testexec.DumpLogOnError)
	if err != nil {
		// Device might still be trying to suspend, so need to restart.
		restartPowerd(ctx)

		return errors.Wrap(err, "failed to suspend device")
	}

	return nil
}

func restartPowerd(ctx context.Context) {
	testing.ContextLog(ctx, "Restarting powerd")
	err := testexec.CommandContext(ctx, "restart", "powerd").Run(testexec.DumpLogOnError)
	if err != nil {
		testing.ContextLog(ctx, "Failed restarting DUT powerd, rebooting: ", err)
		// If we fail to restart powerd, just reboot.
		testexec.CommandContext(ctx, "reboot").Run(testexec.DumpLogOnError)
		if err != nil {
			testing.ContextLog(ctx, "Failed restarting DUT: ", err)
		}
	}
}
