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

// backlightBrightness returns the current backlight brightness in percent.
func backlightBrightness(ctx context.Context) (uint, error) {
	output, err := testexec.CommandContext(ctx, "backlight_tool", "--get_brightness").Output(testexec.DumpLogOnError)
	if err != nil {
		return 0, errors.Wrap(err, "unable to get current backlight brightness")
	}
	brightness, err := strconv.ParseUint(strings.TrimSpace(string(output)), 10, 64)
	if err != nil {
		return 0, errors.Wrapf(err, "unable to parse current backlight brightness from %q", output)
	}
	return uint(brightness), nil
}

// getDefaultBacklightBrightness returns the backlight brightness at a given
// lux level.
func getDefaultBacklightBrightness(ctx context.Context, lux uint) (uint, error) {
	luxArg := "--lux=" + strconv.FormatUint(uint64(lux), 10)
	output, err := testexec.CommandContext(ctx, "backlight_tool", "--get_initial_brightness", luxArg).Output(testexec.DumpLogOnError)
	if err != nil {
		return 0, errors.Wrap(err, "unable to get default backlight brightness")
	}
	brightness, err := strconv.ParseUint(strings.TrimSpace(string(output)), 10, 64)
	if err != nil {
		return 0, errors.Wrap(err, "unable to parse default backlight brightness")
	}
	return uint(brightness), nil
}

// setBacklightBrightness sets the backlight brightness.
func setBacklightBrightness(ctx context.Context, brightness uint) error {
	brightnessArg := "--set_brightness=" + strconv.FormatUint(uint64(brightness), 10)
	if err := testexec.CommandContext(ctx, "backlight_tool", brightnessArg).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "unable to set backlight brightness")
	}
	return nil
}

// SetBacklightLux sets the screen backlight to a brightness in lux.
func SetBacklightLux(ctx context.Context, lux uint, chain CleanupChain) (CleanupChain, error) {
	setupFailed, guard := SetupFailureGuard(chain)
	defer guard(ctx)

	prevBrightness, err := backlightBrightness(ctx)
	if err != nil {
		return nil, err
	}
	brightness, err := getDefaultBacklightBrightness(ctx, lux)
	if err != nil {
		return nil, err
	}
	if err := setBacklightBrightness(ctx, brightness); err != nil {
		return nil, err
	}
	testing.ContextLogf(ctx, "Set screen backlight brightness to %d (%d lux) from %d", brightness, lux, prevBrightness)

	return SetupSucceeded(setupFailed, chain, func(ctx context.Context) error {
		if err := setBacklightBrightness(ctx, prevBrightness); err != nil {
			return err
		}
		testing.ContextLogf(ctx, "Reset screen backlight brightness to %d", prevBrightness)
		return err
	})
}
