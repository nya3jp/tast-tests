// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package utils

import (
	"context"
	"fmt"
	"io/ioutil"
	"strings"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

// SuspendAndResume calls powerd_dbus_suspend command to suspend the system and lets it
// stay sleep for the given duration and then wake up.
func SuspendAndResume(ctx context.Context, cr *chrome.Chrome, sleepTime int) error {
	// The actual time used to suspend and weekup the system is:
	// 		(time to suspend the system) + (sleep time) + (time to wakeup the system)
	// Tast runner might time out if DUT is inaccessible for more than 60 seconds.
	// We allow 30-second maximum sleep time, trying to keep the total suspend/wakeup time
	// under 1 minute.
	const maxSleepTime = 30
	if sleepTime > maxSleepTime {
		return errors.Errorf("suspend time should less than %d seconds", maxSleepTime)
	}

	// timeout, according to powerd_dbus_suspend help page, defines how long to wait for
	// a resume signal in seconds. We add 20 seconds to maxSleepTime to ensure the command
	// will exit if the whole suspend/wakeup procedure couldn't trigger a resume signal for
	// any reason within this time.
	timeout := maxSleepTime + 20

	// Read wakeup count here to prevent suspend retries, which happens without user input.
	wakeupCount, err := ioutil.ReadFile("/sys/power/wakeup_count")
	if err != nil {
		return errors.Wrap(err, "failed to read wakeup count before suspend")
	}

	cmd := testexec.CommandContext(
		ctx,
		"powerd_dbus_suspend",
		"--disable_dark_resume=true",
		fmt.Sprintf("--timeout=%d", timeout),
		fmt.Sprintf("--wakeup_count=%s", strings.Trim(string(wakeupCount), "\n")),
		fmt.Sprintf("--suspend_for_sec=%d", sleepTime),
	)
	testing.ContextLogf(ctx, "Start a DUT suspend of %d seconds: %s", sleepTime, cmd.Args)

	if err := cmd.Run(); err != nil {
		return errors.Wrap(err, "powerd_dbus_suspend failed to properly suspend")
	}

	testing.ContextLog(ctx, "DUT resumes from suspend")
	return cr.Reconnect(ctx)
}
