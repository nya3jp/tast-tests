// Copyright 2022 The Chromium OS Authors. All rights reserved.
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

// CCADimBacklight dim backlight to avoid chart reflect DUT backlight for CCA tast test
func CCADimBacklight(ctx context.Context) (str string) {
	originalVal, err := exec.CommandContext(ctx, "backlight_tool", "--get_brightness_percent").Output()
	if err != nil {
		testing.ContextLog(ctx, "Failed to get brightness_percent")
	}
	brightnessVal := string(originalVal)
	testing.ContextLog(ctx, "DUT_Backlight Save original brightness level of % :", brightnessVal)
	brightnessArg := fmt.Sprintf("--set_brightness_percent=%d", brightness)
	err = exec.CommandContext(ctx, "backlight_tool", brightnessArg).Run()
	if err != nil {
		testing.ContextLog(ctx, "Failed to set brightness")
	}
	return brightnessVal

}

// CCARestoreBacklight restore backlight to original value for CCA usage
func CCARestoreBacklight(ctx context.Context, OriginalBrightness string) {
	testing.ContextLog(ctx, "DUT_Backlight RestoreBacklight to original %:", OriginalBrightness)
	brightnessArg := fmt.Sprintf("--set_brightness_percent=%s", OriginalBrightness)
	err := exec.CommandContext(ctx, "backlight_tool", brightnessArg).Run()
	if err != nil {
		testing.ContextLog(ctx, "Failed to store back brightness: ", err)
	}
}

// DimBacklight dim backlight to avoid chart reflect DUT backlight for tast test
func DimBacklight(ctx context.Context, d *dut.DUT) (str string) {
	originalVal, err := d.Conn().CommandContext(ctx, "backlight_tool", "--get_brightness_percent").Output()
	if err != nil {
		testing.ContextLog(ctx, "Failed to get brightness_percent")
	}
	brightnessVal := string(originalVal)
	testing.ContextLog(ctx, "DUT_Backlight Save original brightness level of % :", brightnessVal)
	brightnessArg := fmt.Sprintf("--set_brightness_percent=%d", brightness)
	err = d.Conn().CommandContext(ctx, "backlight_tool", brightnessArg).Run()
	if err != nil {
		testing.ContextLog(ctx, "Failed to set brightness")
	}
	return brightnessVal
}

// RestoreBacklight restore backlight to original value
func RestoreBacklight(ctx context.Context, d *dut.DUT, OriginalBrightness string) {
	testing.ContextLog(ctx, "DUT_Backlight RestoreBacklight to original %:", OriginalBrightness)
	brightnessArg := fmt.Sprintf("--set_brightness_percent=%s", OriginalBrightness)
	err := d.Conn().CommandContext(ctx, "backlight_tool", brightnessArg).Run()
	if err != nil {
		testing.ContextLog(ctx, "Failed to store back brightness: ", err)
	}
}
