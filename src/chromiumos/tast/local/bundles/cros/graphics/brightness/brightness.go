// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package brightness

import (
	"context"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

// KeyboardBrightnessTest performs increase and decrease of display brightness.
func KeyboardBrightnessTest(ctx context.Context, kb *input.KeyboardEventWriter, brghtKey string) error {
	const brigtnessLevel = 16
	for level := 0; level < brigtnessLevel; level++ {
		preBrightness, err := GetSystemBrightness(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get system brightness")
		}
		if err := kb.Accel(ctx, brghtKey); err != nil {
			return errors.Wrapf(err, "failed to press key: %q", level)
		}
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			brightness, err := GetSystemBrightness(ctx)
			if err != nil {
				return testing.PollBreak(errors.Wrap(err, "failed to get system brightness"))
			}
			if brightness == preBrightness {
				return errors.New("brightness not changed")
			}
			return nil
		}, &testing.PollOptions{Timeout: 1 * time.Second}); err != nil {
			return errors.Wrap(err, "failed to wait for brightness change")
		}
	}
	return nil
}

// GetSystemBrightness get the current brightness of the system.
func GetSystemBrightness(ctx context.Context) (float64, error) {
	out, err := testexec.CommandContext(ctx, "backlight_tool", "--get_brightness_percent").Output()
	if err != nil {
		return 0.0, errors.Wrap(err, "failed to execute brightness command")
	}
	sysBrightness, err := strconv.ParseFloat(strings.Replace(string(out), "\n", "", -1), 64)
	if err != nil {
		return 0.0, errors.Wrap(err, "failed to parse string into float64")
	}
	return sysBrightness, nil
}

// SetBrightnessMax set maximum brightness of the system.
func SetBrightnessMax(ctx context.Context) error {
	if err := testexec.CommandContext(ctx, "backlight_tool", "--set_brightness_percent=100").Run(); err != nil {
		return errors.Wrap(err, "failed to set max brightness")
	}
	return nil
}
