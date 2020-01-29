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
)

// getBacklightBrightness returns the current backlight brightness in percent.
func getBacklightBrightness(ctx context.Context) (uint, error) {
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

// setBacklightLux is an Action that sets the screen backlight
// brightness.
type setBacklightLux struct {
	ctx            context.Context
	prevBrightness uint
	lux            uint
}

// Setup sets the screen backlight lux to a.lux.
func (a *setBacklightLux) Setup() error {
	prevBrightness, err := getBacklightBrightness(a.ctx)
	if err != nil {
		return err
	}
	a.prevBrightness = prevBrightness
	brightness, err := getDefaultBacklightBrightness(a.ctx, a.lux)
	if err != nil {
		return err
	}
	return setBacklightBrightness(a.ctx, brightness)
}

// Cleanup restores the previous backlight brightness.
func (a *setBacklightLux) Cleanup() error {
	return setBacklightBrightness(a.ctx, a.prevBrightness)
}

// SetBacklightLux creates a Action that sets the screen backlight to a
// given lux level.
func SetBacklightLux(ctx context.Context, lux uint) Action {
	return &setBacklightLux{
		ctx:            ctx,
		prevBrightness: 0,
		lux:            lux,
	}
}
