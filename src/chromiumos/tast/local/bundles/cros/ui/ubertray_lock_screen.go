// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/chrome/ui/ubertray"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: UbertrayLockScreen,
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

// UbertrayLockScreen tests that the screen can be locked from the ubertray
// and verifies the ubertray's contents when the screen is locked.
func UbertrayLockScreen(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// Take a screenshot to show a notification. Using the virtual keyboard is required
	// since different physical keyboards can require different key combinations to take a screenshot.
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

	if err := ubertray.Show(ctx, tconn); err != nil {
		s.Fatal("Failed to open the ubertray: ", err)
	}

	if err := ubertray.LockScreen(ctx, tconn); err != nil {
		s.Fatal("Failed to lock the screen: ", err)
	}

	// Locking the screen hides the ubertray, so show it again.
	if err := ubertray.Show(ctx, tconn); err != nil {
		s.Fatal("Failed to open the ubertray: ", err)
	}

	// Check if notifications are hidden.
	if hidden, err := ubertray.AreNotificationsHidden(ctx, tconn); err != nil {
		s.Fatal("Notifications not hidden: ", err)
	} else if !hidden {
		s.Fatal("Notifications were not hidden")
	}

	// Check that the network and bluetooth buttons are present and restricted (cannot be clicked).
	for _, setting := range []ubertray.QuickSetting{ubertray.QuickSettingBluetooth, ubertray.QuickSettingNetwork} {
		if restricted, err := ubertray.IsPodRestricted(ctx, tconn, setting); err != nil {
			s.Errorf("Failed to check restricted status of pod setting %v: %v", setting, err)
		} else if !restricted {
			s.Errorf("Pod setting %v not restricted: %v", setting, err)
		}
	}

	// Check that the expected UI elements are shown in the Ubertray.
	accessParams, err := ubertray.PodIconParams(ubertray.QuickSettingAccessibility)
	if err != nil {
		s.Fatal("Failed to get params for accessibility pod icon: ", err)
	}

	// Associate the params with a descriptive name for better error reporting.
	checkNodes := map[string]ui.FindParams{
		"Accessibility pod": accessParams,
		"Brightness slider": ubertray.BrightnessSliderParams,
		"Volume slider":     ubertray.VolumeSliderParams,
		"Signout button":    ubertray.SignoutBtnParams,
		"Shutdown button":   ubertray.ShutdownBtnParams,
		"Collapse button":   ubertray.CollapseBtnParams,
		"Date/time display": ubertray.DateViewParams,
	}

	if s.Param().(bool) {
		checkNodes["Battery display"] = ubertray.BatteryViewParams
	}

	for node, params := range checkNodes {
		if shown, err := ui.Exists(ctx, tconn, params); err != nil {
			s.Errorf("Failed to check existence of %v: %v", node, err)
		} else if !shown {
			s.Errorf("%v was not found in the UI", node)
		}
	}
}
