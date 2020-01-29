// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"context"
	"strconv"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func getKeyboardBrightness(ctx context.Context) (uint, error) {
	output, err := testexec.CommandContext(ctx, "backlight_tool", "--keyboard", "--get_brightness").Output(testexec.DumpLogOnError)
	if err != nil {
		return 0, errors.Wrap(err, "unable to get current keyboard brightness")
	}
	brightness, err := strconv.ParseUint(strings.TrimSpace(string(output)), 10, 64)
	if err != nil {
		return 0, errors.Wrapf(err, "unable to parse current keyboard brightness from %q", output)
	}
	return uint(brightness), nil
}

func setKeyboardBrightness(ctx context.Context, brightness uint) error {
	brightnessArg := "--set_brightness=" + strconv.FormatUint(uint64(brightness), 10)
	if err := testexec.CommandContext(ctx, "backlight_tool", "--keyboard", brightnessArg).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "unable to set keyboard brightness")
	}
	return nil
}

// SetKeyboardBrightness sets the keyboard brightness.
func SetKeyboardBrightness(ctx context.Context, brightness uint, chain CleanupChain) (CleanupChain, error) {
	setupFailed, guard := SetupFailureGuard(chain)
	defer guard(ctx)

	prevBrightness, err := getKeyboardBrightness(ctx)
	if err != nil {
		return nil, err
	}
	if err := setKeyboardBrightness(ctx, brightness); err != nil {
		return nil, err
	}
	testing.ContextLogf(ctx, "Set keyboard backlight brightness to %d from %d", brightness, prevBrightness)

	return SetupSucceeded(setupFailed, chain, func(ctx context.Context) error {
		if err := setKeyboardBrightness(ctx, prevBrightness); err != nil {
			return err
		}
		testing.ContextLogf(ctx, "Reset keyboard backlight brightness to %d", prevBrightness)
		return nil
	})
}
