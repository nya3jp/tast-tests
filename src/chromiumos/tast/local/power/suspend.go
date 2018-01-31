// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

const (
	wakeupCountPath = "/sys/power/wakeup_count" // sysfs file containing wake event count
)

// SuspendOptions is passed to Suspend to configure how the system is suspended.
type SuspendOptions struct {
	// Duration specifies an RTC alarm timeout used to ensure that the
	// device wakes after being suspended. Rounded to the nearest second.
	Duration time.Duration
}

// Suspend suspends the system.
func Suspend(opt SuspendOptions) error {
	// If powerd receives a wakeup count from the caller that requested suspending,
	// it gives up if the attempt fails. We want this to happen to make sure that a
	// retry doesn't succeed after the RTC alarm fires (leaving the system in a suspended
	// state), so always pass the count.
	wc, err := readWakeupCount()
	if err != nil {
		return err
	}
	args := []string{fmt.Sprintf("--wakeup_count=%v", wc)}

	if opt.Duration > 0 {
		args = append(args, fmt.Sprintf("--wakeup_timeout=%d", opt.Duration.Round(time.Second)/time.Second))
	}
	if out, err := exec.Command("powerd_dbus_suspend", args...).CombinedOutput(); err != nil {
		return fmt.Errorf("powerd_dbus_suspend failed: %v (%s)",
			err, strings.Split(string(out), "\n")[0])
	}
	return nil
}

// readWakeupCount reads and returns the wakeup count from sysfs.
func readWakeupCount() (uint64, error) {
	b, err := ioutil.ReadFile(wakeupCountPath)
	if err != nil {
		return 0, err
	}
	c, err := strconv.ParseUint(string(bytes.TrimSpace(b)), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse %v: %v", wakeupCountPath, err)
	}
	return c, nil
}
