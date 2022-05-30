// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package reporters

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// Now reports the output of the `date` command as a Go Time.
func (r *Reporter) Now(ctx context.Context) (time.Time, error) {
	const (
		bashFormat = "%Y-%m-%d %H:%M:%S"
		goFormat   = "2006-01-02 15:04:05"
	)
	res, err := r.CommandOutput(ctx, "date", fmt.Sprintf("+%s", bashFormat))
	if err != nil {
		return time.Time{}, err
	}
	return time.Parse(goFormat, res)
}

// DoAllPathsExist reports whether all given paths exist on the DUT.
func (r *Reporter) DoAllPathsExist(ctx context.Context, paths []string) (bool, error) {
	out, err := r.CombinedOutput(ctx, "file", append([]string{"-E"}, paths...)...)
	if err == nil {
		return true, nil
	}
	if strings.Contains(out, "No such file or directory") {
		return false, nil
	}
	return false, err
}

// BootID reports the device's Boot ID, which changes upon reboot.
func (r *Reporter) BootID(ctx context.Context) (string, error) {
	return r.CatFile(ctx, "/proc/sys/kernel/random/boot_id")
}

// BatteryStatus reports the device's battery status.
func (r *Reporter) BatteryStatus(ctx context.Context) (string, error) {
	return r.CatFile(ctx, "/sys/class/power_supply/BAT0/status")
}
