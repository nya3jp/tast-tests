// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package setup

import (
	"context"
	"io/ioutil"
	"os"
	"path"
	"strconv"
	"strings"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
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

// defaultBacklightBrightness returns the backlight brightness at a given lux
// level. We use backlight_tool instead of sysfs directly because the conversion
// between lux and brightness is complicated and hard to extract.
func defaultBacklightBrightness(ctx context.Context, lux uint) (uint, error) {
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

// listBacklightPaths lists paths of backlights in sysfs
func listBacklightPaths() ([]string, error) {
	const sysfsBacklightPath = "/sys/class/backlight"
	files, err := ioutil.ReadDir(sysfsBacklightPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Ignore NotExist error since /sys/class/backlight may not exist
			return nil, nil
		}
		return nil, errors.Wrap(err, "failed to read sysfs dir")
	}
	var backlightPaths []string
	for _, file := range files {
		devPath := path.Join(sysfsBacklightPath, file.Name())
		backlightPaths = append(backlightPaths, devPath)
	}
	return backlightPaths, nil
}

// SetBacklightLux sets the screen backlight to a brightness in lux.
func SetBacklightLux(ctx context.Context, lux uint) (CleanupCallback, error) {
	backlightPaths, err := listBacklightPaths()
	if err != nil {
		return nil, err
	}
	if len(backlightPaths) == 0 {
		testing.ContextLog(ctx, "Skipping setting screen backlight brightness since there are no backlights")
		return nil, nil
	}
	// TODO(hikarun): Remove listing backlights after checking this logic works on all platforms
	testing.ContextLogf(ctx, "%v backlights found:", len(backlightPaths))
	for _, path := range backlightPaths {
		testing.ContextLog(ctx, path)
	}
	prevBrightness, err := backlightBrightness(ctx)
	if err != nil {
		return nil, err
	}
	brightness, err := defaultBacklightBrightness(ctx, lux)
	if err != nil {
		return nil, err
	}
	testing.ContextLogf(ctx, "Setting screen backlight brightness to %d (%d lux) from %d", brightness, lux, prevBrightness)
	if err := setBacklightBrightness(ctx, brightness); err != nil {
		return nil, err
	}

	return func(ctx context.Context) error {
		testing.ContextLogf(ctx, "Reseting screen backlight brightness to %d", prevBrightness)
		return setBacklightBrightness(ctx, prevBrightness)
	}, nil
}
