// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package setup

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

func noKeyboardBrightness(ctx context.Context) bool {
	_, stderr, err := testexec.CommandContext(ctx, "backlight_tool", "--keyboard", "--get_brightness").SeparatedOutput()
	return err != nil && strings.HasPrefix(string(stderr), "No backlight in")
}

func keyboardBrightness(ctx context.Context) (uint, error) {
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
	brightnessArg := fmt.Sprintf("--set_brightness=%d", brightness)
	if err := testexec.CommandContext(ctx, "backlight_tool", "--keyboard", brightnessArg).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "unable to set keyboard brightness")
	}
	return nil
}

// SetKeyboardBrightness sets the keyboard brightness if there is a backlight.
func SetKeyboardBrightness(ctx context.Context, brightness uint) (CleanupCallback, error) {
	if noKeyboardBrightness(ctx) {
		return nil, nil
	}
	prevBrightness, err := keyboardBrightness(ctx)
	if err != nil {
		return nil, err
	}
	testing.ContextLogf(ctx, "Setting keyboard backlight brightness to %d from %d", brightness, prevBrightness)
	if err := setKeyboardBrightness(ctx, brightness); err != nil {
		return nil, err
	}

	return func(ctx context.Context) error {
		testing.ContextLogf(ctx, "Resetting keyboard backlight brightness to %d", prevBrightness)
		return setKeyboardBrightness(ctx, prevBrightness)
	}, nil
}
