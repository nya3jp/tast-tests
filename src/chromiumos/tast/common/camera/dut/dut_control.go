// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package dutcontrol provides utilities control for DUT.
package dutcontrol

import (
	"context"
	"fmt"
	"os/exec"
)

const brightness = 1

// doDimBacklight is the execution logic of dimming backlight.
func doDimBacklight(ctx context.Context, executeCommand func(string, ...string) (string, error)) (string, error) {
	originalVal, err := executeCommand("backlight_tool", "--get_brightness_percent")
	if err != nil {
		return "", err
	}
	brightnessArg := fmt.Sprintf("--set_brightness_percent=%d", brightness)
	if _, err := executeCommand("backlight_tool", brightnessArg); err != nil {
		return "", err
	}
	return originalVal, nil
}

// CCADimBacklight is dim backlight to avoid chart reflect DUT backlight on CCA tast test.
func CCADimBacklight(ctx context.Context) (string, error) {
	return doDimBacklight(ctx, func(name string, args ...string) (string, error) {
		originalVal, err := exec.CommandContext(ctx, name, args...).Output()
		if err != nil {
			return "", err
		}
		brightnessVal := string(originalVal)
		return brightnessVal, nil
	})
}

// doRestoreBacklight is execution logic of Restore to original brightness.
func doRestoreBacklight(ctx context.Context, originalBrightness string, executeCommand func(string, ...string) error) error {
	brightnessArg := fmt.Sprintf("--set_brightness_percent=%s", originalBrightness)
	err := executeCommand("backlight_tool", brightnessArg)
	if err != nil {
		return err
	}
	return nil
}

// CCARestoreBacklight is restore backlight to original value for CCA usage.
func CCARestoreBacklight(ctx context.Context, originalBrightness string) error {
	return doRestoreBacklight(ctx, originalBrightness, func(name string, args ...string) error {
		err := exec.CommandContext(ctx, name, args...).Run()
		if err != nil {
			return err
		}
		return nil
	})
}
