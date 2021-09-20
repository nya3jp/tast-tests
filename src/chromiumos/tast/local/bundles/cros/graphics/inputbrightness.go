// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package graphics

import (
	"context"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type inputbrightnessParams struct {
	inputType string
}

const (
	brightnessLevels   = 16
	brightnessCmd      = "backlight_tool --get_brightness_percent"
	resetBrightnessCmd = "backlight_tool --set_brightness_percent=100"
)

var (
	brightIncKey = "brightnessup"
	brightDecKey = "brightnessdown"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Inputbrightness,
		Desc:         "Verifies system Brightness increase and decrease through onbaord keyboard and UI",
		Contacts:     []string{"pathan.jilani@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Attr:         []string{"group:mainline", "informational"},
		Params: []testing.Param{{
			Name:    "brightness_keyboard",
			Val:     inputbrightnessParams{inputType: "keyboard"},
			Fixture: "chromePolicyLoggedIn",
		},
			{
				Name:    "brightness_ui",
				Val:     inputbrightnessParams{inputType: "settingsUI"},
				Fixture: "chromePolicyLoggedIn",
			},
		},
	})
}

func Inputbrightness(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*fixtures.FixtData).Chrome
	testOpt := s.Param().(inputbrightnessParams)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	brightnessSetMax := func() {
		if err := testexec.CommandContext(ctx, "bash", "-c", resetBrightnessCmd).Run(); err != nil {
			s.Fatal("Unable to execute brightness reset command: ", err)
		}
	}
	defer func(ctx context.Context) {
		s.Log("Resetting brightness")
		brightnessSetMax()
	}(ctx)
	brightnessSetMax()
	if testOpt.inputType == "keyboard" {
		kb, err := input.Keyboard(ctx)
		if err != nil {
			s.Fatal("Failed to create keyboard object: ", err)
		}
		// Decreasing brightness level with on-board keyboard key press
		if err := performingBrightnessTest(ctx, kb, brightDecKey); err != nil {
			s.Fatal("Failed to decrease system brightness: ", err)
		}
		brightnessDecValue, err := getSystemBrightness(ctx)
		if err != nil {
			s.Fatal("Failed to get system brightness after performing Brightness decrease: ", err)
		}
		if brightnessDecValue != 0.0 {
			s.Fatal("Failed to decrease brightness to minimum value")
		}
		// Increasing brightness level with on-board keyboard key press
		if err := performingBrightnessTest(ctx, kb, brightIncKey); err != nil {
			s.Fatal("Failed to increase system brightness: ", err)
		}
		brightnessIncValue, err := getSystemBrightness(ctx)
		if err != nil {
			s.Fatal("Failed to get system brightness after performing Brightness increase: ", err)
		}
		if brightnessIncValue != 100.0 {
			s.Fatal("Failed to increase brightness to maximum value")
		}
	}
	if testOpt.inputType == "settingsUI" {
		fdms := s.FixtValue().(*fixtures.FixtData).FakeDMS
		for _, param := range []struct {
			name           string
			wantBrightness string
			policy         *policy.ScreenBrightnessPercent
		}{
			{
				name:           "brightnessDecreaseCheck",
				wantBrightness: "0%",
				policy: &policy.ScreenBrightnessPercent{
					Val: &policy.ScreenBrightnessPercentValue{
						BrightnessAC:      0,
						BrightnessBattery: 0,
					},
				},
			},
			{
				name:           "brightnessIncreaseCheck",
				wantBrightness: "100%",
				policy: &policy.ScreenBrightnessPercent{
					Val: &policy.ScreenBrightnessPercentValue{
						BrightnessAC:      100,
						BrightnessBattery: 100,
					},
				},
			},
		} {
			s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
				// Perform cleanup.
				if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
					s.Fatal("Failed to clean up: ", err)
				}
				// Update policies.
				if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{param.policy}); err != nil {
					s.Fatal("Failed to update policies: ", err)
				}
				ui := uiauto.New(tconn)

				// Find the Status tray node and click to open it.
				statusTray := nodewith.ClassName("ash/StatusAreaWidgetDelegate")
				if err := uiauto.Combine("find and click the status tray",
					ui.WaitUntilExists(statusTray),
					ui.LeftClick(statusTray),
				)(ctx); err != nil {
					s.Fatal("Failed to find and click the status try: ", err)
				}

				defer func() {
					// Close the Status tray again, otherwise the next subtest won't find it.
					if err := ui.LeftClick(statusTray)(ctx); err != nil {
						s.Fatal("Failed to close Status tray: ", err)
					}
				}()

				// Get the NodeInfo of the Brightness slider.
				brightnessSlider := nodewith.Name("Brightness").Role(role.Slider)
				sliderInfo, err := ui.Info(ctx, brightnessSlider)
				if err != nil {
					s.Fatal("Failed to find Brightness slider: ", err)
				}

				if sliderInfo.Value != param.wantBrightness {
					s.Fatalf("Failed brightness set: got %s; want %s", sliderInfo.Value, param.wantBrightness)
				}
			})
		}
	}
}

func performingBrightnessTest(ctx context.Context, kb *input.KeyboardEventWriter, brghtKey string) error {
	for level := range [brightnessLevels]int{} {
		if err := kb.Accel(ctx, brghtKey); err != nil {
			return errors.Wrapf(err, "failed to press key: %q", level)
		}
		if err := testing.Sleep(ctx, time.Second); err != nil {
			return errors.Wrap(err, "failed while waiting for key press")
		}
	}
	return nil
}

func getSystemBrightness(ctx context.Context) (float64, error) {
	out, err := testexec.CommandContext(ctx, "bash", "-c", brightnessCmd).Output()
	if err != nil {
		return 0.0, errors.Wrap(err, "Unable to execute brightness command")
	}
	sysBrightness, err := strconv.ParseFloat(strings.Replace(string(out), "\n", "", -1), 64)
	if err != nil {
		return 0.0, errors.Wrap(err, "failed to parse string into float64")
	}
	return sysBrightness, nil
}
