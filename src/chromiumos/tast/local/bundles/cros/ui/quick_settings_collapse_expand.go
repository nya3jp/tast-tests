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
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: QuickSettingsCollapseExpand,
		Desc: "Checks that Quick Settings can be collapsed and expanded",
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

// QuickSettingsCollapseExpand tests collapsing and expanding Quick Settings.
func QuickSettingsCollapseExpand(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// TODO(crbug/1099502): replace this with quicksettings.Show when retry is no longer needed.
	if err := quicksettings.ShowWithRetry(ctx, tconn, 10*time.Second); err != nil {
		s.Fatal("Failed to open quick settings: ", err)
	}
	defer quicksettings.Hide(ctx, tconn)

	if err := quicksettings.Collapse(ctx, tconn); err != nil {
		s.Fatal("Failed to collapse quick settings: ", err)
	}
	defer quicksettings.Expand(ctx, tconn)

	// Check that the expected UI elements are shown in the collapsed Quick Settings.
	networkParams, err := quicksettings.PodIconParams(quicksettings.SettingPodNetwork)
	if err != nil {
		s.Fatal("Failed to get params for network pod icon: ", err)
	}
	dndParams, err := quicksettings.PodIconParams(quicksettings.SettingPodDoNotDisturb)
	if err != nil {
		s.Fatal("Failed to get params for Do Not Disturb pod icon: ", err)
	}
	nightLightParams, err := quicksettings.PodIconParams(quicksettings.SettingPodNightLight)
	if err != nil {
		s.Fatal("Failed to get params for Night Light pod icon: ", err)
	}

	// Associate the params with a descriptive name for better error reporting.
	checkNodes := map[string]ui.FindParams{
		"Network pod":        networkParams,
		"Do Not Disturb pod": dndParams,
		"Night Light pod":    nightLightParams,
		"Signout button":     quicksettings.SignoutBtnParams,
		"Shutdown button":    quicksettings.ShutdownBtnParams,
		"Lock button":        quicksettings.LockBtnParams,
		"Settings button":    quicksettings.SettingsBtnParams,
		"Date/time display":  quicksettings.DateViewParams,
	}

	// Only check for the battery display if the DUT has a battery.
	if s.Param().(bool) {
		checkNodes["Battery display"] = quicksettings.BatteryViewParams
	}

	// Only check for the bluetooth pod if the DUT has a bluetooth adapter.
	adapters, err := bluetooth.Adapters(ctx)
	if err != nil {
		s.Fatal("Unable to get Bluetooth adapters: ", err)
	}
	if len(adapters) > 0 {
		bluetoothParams, err := quicksettings.PodIconParams(quicksettings.SettingPodBluetooth)
		if err != nil {
			s.Fatal("Failed to get params for bluetooth pod icon: ", err)
		}
		checkNodes["Bluetooth pod"] = bluetoothParams
	}

	for node, params := range checkNodes {
		if shown, err := ui.Exists(ctx, tconn, params); err != nil {
			s.Errorf("Failed to check existence of %v: %v", node, err)
		} else if !shown {
			s.Errorf("%v was not found in the UI", node)
		}
	}

	// Verify that expanding Quick Settings works as well.
	if err := quicksettings.Expand(ctx, tconn); err != nil {
		s.Fatal("Failed to expand Quick Settings: ", err)
	}
}
