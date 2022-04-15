// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/power"
	"chromiumos/tast/local/power/setup"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ScreenBrightness,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Test metrics collection from lacros-Chrome",
		Contacts:     []string{"xliu@cienet.com"},
		SoftwareDeps: []string{"chrome", "lacros"},
		Fixture:      "chromeLoggedIn",
		Timeout:      5 * time.Minute,
	})
}

func ScreenBrightness(ctx context.Context, s *testing.State) {
	targetBrightnessByPM := 100
	powerdRestartReadyWait := 5 * time.Second
	screenBrightnessChangeWait := 20 * time.Second

	cr := s.FixtValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	// Restart powerd
	if err := restartPowerd(ctx); err != nil {
		s.Fatal("Failed to restart powerd: ", err)
	}
	testing.Sleep(ctx, powerdRestartReadyWait) // Wait for powerd to fully work.
	s.Log("=========1. Test Power Manager Brightness Setting")
	func() {
		pm, err := power.NewPowerManager(ctx)
		if err != nil {
			s.Fatal("Failed to create a PowerManager object: ", err)
		}
		initBrightness, err := pm.GetScreenBrightnessPercent(ctx)
		if err != nil {
			s.Fatal("Failed to get screen brightness: ", err)
		}
		s.Log("Before test - screen brightness from PowerManager: ", initBrightness)

		s.Log("Set screen brightness by PowerManager to: ", targetBrightnessByPM)
		if err := pm.SetScreenBrightness(ctx, 100); err != nil {
			s.Fatal("Failed to set the screen brightness: ", err)
		}

		testing.Sleep(ctx, screenBrightnessChangeWait)
		brightness, err := pm.GetScreenBrightnessPercent(ctx)
		if err != nil {
			s.Fatal("Failed to get screen brightness: ", err)
		}
		s.Log("During test - screen brightness from PowerManager: ", brightness)
		testing.Sleep(ctx, 5*time.Second)

		s.Log("Restore screen brightness by PowerManager to: ", initBrightness)
		if err := pm.SetScreenBrightness(ctx, initBrightness); err != nil {
			s.Fatal("Failed to restore the screen brightness: ", err)
		}

		testing.Sleep(ctx, 5*time.Second)

		brightness, err = pm.GetScreenBrightnessPercent(ctx)
		if err != nil {
			s.Fatal("Failed to get screen brightness: ", err)
		}
		s.Log("After test - screen brightness from PowerManager: ", brightness)
	}()

	// Restart powerd
	if err := restartPowerd(ctx); err != nil {
		s.Fatal("Failed to restart powerd: ", err)
	}
	testing.Sleep(ctx, powerdRestartReadyWait) // Wait for powerd to fully work.
	s.Log("=========2. Test backlight_tool Brightness Setting")
	func() {
		initBrightness, err := getBacklightBrightnessPercent(ctx)
		if err != nil {
			s.Fatal("Failed to get screen brightness: ", err)
		}
		s.Log("Before test - screen brightness from backlight_tool: ", initBrightness)

		s.Log("Set screen brightness by backlight_tool to: ", targetBrightnessByPM)
		if err := setBacklightBrightnessPercent(ctx, 100); err != nil {
			s.Fatal("Failed to set the screen brightness: ", err)
		}

		testing.Sleep(ctx, screenBrightnessChangeWait)
		brightness, err := getBacklightBrightnessPercent(ctx)
		if err != nil {
			s.Fatal("Failed to get screen brightness: ", err)
		}
		s.Log("During test - screen brightness from backlight_tool: ", brightness)
		testing.Sleep(ctx, 5*time.Second)

		s.Log("Restore screen brightness by backlight_tool to: ", initBrightness)
		if err := setBacklightBrightnessPercent(ctx, initBrightness); err != nil {
			s.Fatal("Failed to restore the screen brightness: ", err)
		}

		testing.Sleep(ctx, 5*time.Second)

		brightness, err = getBacklightBrightnessPercent(ctx)
		if err != nil {
			s.Fatal("Failed to get screen brightness: ", err)
		}
		s.Log("During test - screen brightness from backlight_tool: ", brightness)
	}()

	s.Log("=========3. Test backlight_tool Brightness Setting with powerd disabled")
	func() {
		s.Log("Disable powerd service")
		restoreFunc, err := setup.DisableService(ctx, "powerd")
		if err != nil {
			s.Fatal("Failed to disable powerd: ", err)
		}
		defer func() {
			s.Log("Restore powerd service")
			if err := restoreFunc(ctx); err != nil {
				s.Fatal("Failed to enable powerd: ", err)
			}
		}()
		shortCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		if _, err := power.NewPowerManager(shortCtx); err != nil {
			// This is expected. Log the error only.
			s.Log("Cannot get power manager instance: ", err)
		}

		initBrightness, err := getBacklightBrightnessPercent(ctx)
		if err != nil {
			s.Fatal("Failed to get screen brightness: ", err)
		}
		s.Log("Before test - screen brightness from backlight_tool: ", initBrightness)

		s.Log("Set screen brightness by backlight_tool to: ", targetBrightnessByPM)
		if err := setBacklightBrightnessPercent(ctx, 100); err != nil {
			s.Fatal("Failed to set the screen brightness: ", err)
		}

		testing.Sleep(ctx, screenBrightnessChangeWait)
		brightness, err := getBacklightBrightnessPercent(ctx)
		if err != nil {
			s.Fatal("Failed to get screen brightness: ", err)
		}
		s.Log("During test - screen brightness from backlight_tool: ", brightness)
		testing.Sleep(ctx, 5*time.Second)

		s.Log("Restore screen brightness by backlight_tool to: ", initBrightness)
		if err := setBacklightBrightnessPercent(ctx, initBrightness); err != nil {
			s.Fatal("Failed to restore the screen brightness: ", err)
		}

		testing.Sleep(ctx, 5*time.Second)

		brightness, err = getBacklightBrightnessPercent(ctx)
		if err != nil {
			s.Fatal("Failed to get screen brightness: ", err)
		}
		s.Log("After test - screen brightness from backlight_tool: ", brightness)
	}()

	s.Log("=========4. Test PowerTest Setup")
	func() {
		shortCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		if _, err := power.NewPowerManager(shortCtx); err != nil {
			// This is expected. Log the error only.
			s.Log("Cannot get power manager instance: ", err)
		}

		initBrightness, err := getBacklightBrightnessPercent(ctx)
		if err != nil {
			s.Fatal("Failed to get screen brightness: ", err)
		}
		s.Log("Before test - screen brightness from backlight_tool: ", initBrightness)

		var batteryDischargeErr error
		powerSetupCleanup, err := setup.PowerTest(ctx, tconn, setup.PowerTestOptions{
			Wifi:       setup.DoNotChangeWifiInterfaces,
			Battery:    setup.TryBatteryDischarge(&batteryDischargeErr),
			NightLight: setup.DisableNightLight,
		})
		restored := false
		defer func() {
			if !restored {
				if err := powerSetupCleanup(ctx); err != nil {
					testing.ContextLog(ctx, "Failed to clean up power setup: ", err)
				}
			}
		}()

		testing.Sleep(ctx, screenBrightnessChangeWait)
		brightness, err := getBacklightBrightnessPercent(ctx)
		if err != nil {
			s.Fatal("Failed to get screen brightness: ", err)
		}
		s.Log("During test - screen brightness from backlight_tool: ", brightness)
		testing.Sleep(ctx, 5*time.Second)

		s.Log("Restore PowerTest setup")
		restored = true
		if err := powerSetupCleanup(ctx); err != nil {
			s.Fatal("Failed to clean up power setup: ", err)
		}
		testing.Sleep(ctx, 5*time.Second)

		brightness, err = getBacklightBrightnessPercent(ctx)
		if err != nil {
			s.Fatal("Failed to get screen brightness: ", err)
		}
		s.Log("After test - screen brightness from backlight_tool: ", brightness)
	}()
}

// getBacklightBrightnessPercent returns the current backlight brightness in percent.
func getBacklightBrightnessPercent(ctx context.Context) (float64, error) {
	output, err := testexec.CommandContext(ctx, "backlight_tool", "--get_brightness_percent").Output(testexec.DumpLogOnError)
	if err != nil {
		return 0, errors.Wrap(err, "unable to get current backlight brightness percent")
	}
	brightness, err := strconv.ParseFloat(strings.TrimSpace(string(output)), 64)
	if err != nil {
		return 0, errors.Wrapf(err, "unable to parse current backlight brightness from %q", output)
	}
	return brightness, nil
}

// setBacklightBrightnessPercent returns the current backlight brightness in percent.
func setBacklightBrightnessPercent(ctx context.Context, percent float64) error {
	if err := testexec.CommandContext(ctx, "backlight_tool",
		fmt.Sprintf("--set_brightness_percent=%v", percent)).Run(); err != nil {
		return errors.Wrap(err, "unable to set current backlight brightness percent")
	}
	return nil
}

func restartPowerd(ctx context.Context) error {
	restoreFunc, err := setup.DisableService(ctx, "powerd")
	if err != nil {
		return errors.Wrap(err, "failed to disable powerd")
	}
	if err := restoreFunc(ctx); err != nil {
		return errors.Wrap(err, "failed to enable powerd")
	}
	return nil
}
