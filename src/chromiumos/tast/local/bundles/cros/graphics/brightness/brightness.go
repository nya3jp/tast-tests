// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package brightness

import (
	"context"
	"strconv"
	"strings"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
)

// SystemBrightness get the current brightness of the system.
func SystemBrightness(ctx context.Context) (float64, error) {
	out, err := testexec.CommandContext(ctx, "backlight_tool", "--get_brightness_percent").Output()
	if err != nil {
		return 0.0, errors.Wrap(err, "failed to execute brightness command")
	}
	sysBrightness, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
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
