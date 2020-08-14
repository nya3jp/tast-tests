// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/local/bluetooth"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/chrome/ui/quicksettings"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: QuickSettingsLockScreen,
		Desc: "Checks that settings can be opened from the Ubertray",
		Contacts: []string{
			"kyleshima@chromium.org",
			"bhansknecht@chromium.org",
			"dhaddock@chromium.org",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Pre:          chrome.LoggedIn(),
		Params: []testing.Param{{
			ExtraHardwareDeps: hwdep.D(hwdep.Battery()),
			Val:               true,
		}, {
			Name: "no_battery",
			Val:  false,
		}},
	})
}

// QuickSettingsLockScreen tests that the screen can be locked from Quick Settings
// and verifies its contents when the screen is locked.
func QuickSettingsLockScreen(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// Take a screenshot to show a notification. Using the virtual keyboard is required since
	// different physical keyboards can require different key combinations to take a screenshot.
	keyboard, err := input.VirtualKeyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get virtual keyboard: ", err)
	}
	defer keyboard.Close()

	if err := keyboard.Accel(ctx, "Ctrl+F5"); err != nil {
		s.Fatal("Failed to take a screenshot: ", err)
	}

	params := ui.FindParams{
		Role:      ui.RoleTypeWindow,
		ClassName: "ash/message_center/MessagePopup",
	}

	if err := ui.WaitUntilExists(ctx, tconn, params, 10*time.Second); err != nil {
		s.Fatal("Failed to find notification center: ", err)
	}

	if err := quicksettings.LockScreen(ctx, tconn); err != nil {
		s.Fatal("Failed to lock the screen: ", err)
	}

	// Explicitly show Quick Settings on the lock screen, so it will
	// remain open for the UI verification steps.
	if err := quicksettings.Show(ctx, tconn); err != nil {
		s.Fatal("Failed to show quick settings on the lock screen: ", err)
	}
	defer quicksettings.Hide(ctx, tconn)

	// Check if notifications are hidden.
	if hidden, err := quicksettings.AreNotificationsHidden(ctx, tconn); err != nil {
		s.Fatal("Notifications not hidden: ", err)
	} else if !hidden {
		s.Fatal("Notifications were not hidden")
	}

	// Check that the network and bluetooth buttons are present and restricted (cannot be clicked).
	restrictedPods := []quicksettings.SettingPod{quicksettings.SettingPodNetwork}

	// Only check for the bluetooth pod on devices with at least 1 bluetooth adapter.
	adapters, err := bluetooth.Adapters(ctx)
	if err != nil {
		s.Fatal("Unable to get Bluetooth adapters: ", err)
	}
	if len(adapters) > 0 {
		restrictedPods = append(restrictedPods, quicksettings.SettingPodBluetooth)
	}

	for _, setting := range restrictedPods {
		if restricted, err := quicksettings.IsPodRestricted(ctx, tconn, setting); err != nil {
			s.Errorf("Failed to check restricted status of pod setting %v: %v", setting, err)
		} else if !restricted {
			s.Errorf("Pod setting %v not restricted: %v", setting, err)
		}
	}

	// Check that the expected UI elements are shown in Quick Settings.
	accessibilityParams, err := quicksettings.PodIconParams(quicksettings.SettingPodAccessibility)
	if err != nil {
		s.Fatal("Failed to get params for accessibility pod icon: ", err)
	}

	// Associate the params with a descriptive name for better error reporting.
	checkNodes := map[string]ui.FindParams{
		"Accessibility pod": accessibilityParams,
		"Brightness slider": quicksettings.BrightnessSliderParams,
		"Volume slider":     quicksettings.VolumeSliderParams,
		"Signout button":    quicksettings.SignoutBtnParams,
		"Shutdown button":   quicksettings.ShutdownBtnParams,
		"Collapse button":   quicksettings.CollapseBtnParams,
		"Date/time display": quicksettings.DateViewParams,
	}

	// Only check the battery display if the DUT has a battery.
	if s.Param().(bool) {
		checkNodes["Battery display"] = quicksettings.BatteryViewParams
	}

	for node, params := range checkNodes {
		if shown, err := ui.Exists(ctx, tconn, params); err != nil {
			s.Errorf("Failed to check existence of %v: %v", node, err)
		} else if !shown {
			s.Errorf("%v was not found in the UI", node)
		}
	}
}
