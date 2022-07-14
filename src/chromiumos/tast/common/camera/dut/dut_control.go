// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package dutcontrol provides utilities control for DUT.
package dutcontrol

import (
	"context"
	"fmt"
	"os/exec"

	"chromiumos/tast/dut"
	"chromiumos/tast/testing"
)

var brightness = 1

// DoDimBacklight Execution logic of dimming backlight
func DoDimBacklight(ctx context.Context, executeCommand func(string, ...string) (string, error)) (string, error) {
	originalVal, err := executeCommand("backlight_tool", "--get_brightness_percent")
	if err != nil {
		return "", err
	}
	testing.ContextLog(ctx, "DUT_Backlight Save original brightness level of % :", originalVal)
	brightnessArg := fmt.Sprintf("--set_brightness_percent=%d", brightness)
	if _, err := executeCommand("backlight_tool", brightnessArg); err != nil {
		return "", err
	}
	return originalVal, nil
}

// CCADimBacklight dim backlight to avoid chart reflect DUT backlight for CCA tast test
func CCADimBacklight(ctx context.Context) (string, error) {
	return DoDimBacklight(ctx, func(name string, args ...string) (string, error) {
		originalVal, err := exec.CommandContext(ctx, name, args...).Output()
		if err != nil {
			return "", err
		}
		brightnessVal := string(originalVal)
		return brightnessVal, nil
	})
}

// DUTDimBacklight dim backlight to avoid chart reflect DUT backlight for tast test
func DUTDimBacklight(ctx context.Context, d *dut.DUT) (string, error) {
	return DoDimBacklight(ctx, func(name string, args ...string) (string, error) {
		originalVal, err := d.Conn().CommandContext(ctx, name, args...).Output()
		if err != nil {
			return "", err
		}
		brightnessVal := string(originalVal)
		return brightnessVal, nil
	})
}

// DoRestoreBacklight Execution logic of Restore to original brightness
func DoRestoreBacklight(ctx context.Context, originalBrightness string, executeCommand func(string, ...string) error) error {
	testing.ContextLog(ctx, "DUT_Backlight RestoreBacklight to original %:", originalBrightness)
	brightnessArg := fmt.Sprintf("--set_brightness_percent=%s", originalBrightness)
	err := executeCommand("backlight_tool", brightnessArg)
	if err != nil {
		return err
	}
	return nil
}

// CCARestoreBacklight restore backlight to original value for CCA usage
func CCARestoreBacklight(ctx context.Context, originalBrightness string) error {
	return DoRestoreBacklight(ctx, originalBrightness, func(name string, args ...string) error {
		err := exec.CommandContext(ctx, name, args...).Run()
		if err != nil {
			return err
		}
		return nil
	})
}

// DUTRestoreBacklight restore backlight to original value
func DUTRestoreBacklight(ctx context.Context, d *dut.DUT, originalBrightness string) error {
	return DoRestoreBacklight(ctx, originalBrightness, func(name string, args ...string) error {
		err := d.Conn().CommandContext(ctx, name, args...).Run()
		if err != nil {
			return err
		}
		return nil
	})
}
