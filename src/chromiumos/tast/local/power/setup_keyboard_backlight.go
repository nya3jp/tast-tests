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

// getKeyboardBrightness gets the keyboard brightness.
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

// setKeyboardBrightness sets the keyboard brightness.
func setKeyboardBrightness(ctx context.Context, brightness uint) error {
	brightnessArg := "--set_brightness=" + strconv.FormatUint(uint64(brightness), 10)
	if err := testexec.CommandContext(ctx, "backlight_tool", "--keyboard", brightnessArg).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "unable to set keyboard brightness")
	}
	return nil
}

// setKBBrightness is an Action that sets the keyboard backlight
// brightness.
type setKBBrightness struct {
	ctx            context.Context
	brightness     uint
	prevBrightness uint
}

// Setup sets the keyboard backlight brightness.
func (a *setKBBrightness) Setup() error {
	prevBrightness, err := getKeyboardBrightness(a.ctx)
	if err != nil {
		return err
	}
	a.prevBrightness = prevBrightness
	return setKeyboardBrightness(a.ctx, a.brightness)
}

// Cleanup restores the previous keyboard backlight brightness.
func (a *setKBBrightness) Cleanup() error {
	return setKeyboardBrightness(a.ctx, a.prevBrightness)
}

// SetKeyboardBrightness creates an Action that sets the keyboard
// backlight brightness.
func SetKeyboardBrightness(ctx context.Context, brightness uint) Action {
	return &setKBBrightness{
		ctx:            ctx,
		brightness:     brightness,
		prevBrightness: 0,
	}
}
