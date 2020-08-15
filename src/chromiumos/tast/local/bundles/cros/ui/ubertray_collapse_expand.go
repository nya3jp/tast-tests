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
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: UbertrayCollapseExpand,
		Desc: "Checks that the Ubertray can be collapsed and expanded",
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

// UbertrayCollapseExpand tests collapsing and expanding the ubertray.
func UbertrayCollapseExpand(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// TODO(crbug/1099502): replace this with ubertray.Show when retry is no longer needed.
	if err := ubertray.ShowWithRetry(ctx, tconn, 10*time.Second); err != nil {
		s.Fatal("Failed to open the ubertray: ", err)
	}
	defer ubertray.Hide(ctx, tconn)

	if err := ubertray.ToggleCollapsed(ctx, tconn); err != nil {
		s.Fatal("Failed to collapse the ubertray: ", err)
	}

	// Check that the expected UI elements are shown in the collapsed Ubertray.
	networkParams, err := ubertray.PodIconParams(ubertray.QuickSettingNetwork)
	if err != nil {
		s.Fatal("Failed to get params for network pod icon: ", err)
	}
	bluetoothParams, err := ubertray.PodIconParams(ubertray.QuickSettingBluetooth)
	if err != nil {
		s.Fatal("Failed to get params for accessibility pod icon: ", err)
	}
	dndParams, err := ubertray.PodIconParams(ubertray.QuickSettingDoNotDisturb)
	if err != nil {
		s.Fatal("Failed to get params for accessibility pod icon: ", err)
	}
	nightLightParams, err := ubertray.PodIconParams(ubertray.QuickSettingNightLight)
	if err != nil {
		s.Fatal("Failed to get params for accessibility pod icon: ", err)
	}

	// Associate the params with a descriptive name for better error reporting.
	checkNodes := map[string]ui.FindParams{
		"Network pod":        networkParams,
		"Bluetooth pod":      bluetoothParams,
		"Do Not Disturb pod": dndParams,
		"Night Light pod":    nightLightParams,
		"Signout button":     ubertray.SignoutBtnParams,
		"Shutdown button":    ubertray.ShutdownBtnParams,
		"Lock button":        ubertray.LockBtnParams,
		"Settings button":    ubertray.SettingsBtnParams,
		"Date/time display":  ubertray.DateViewParams,
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

	if err := ubertray.ToggleCollapsed(ctx, tconn); err != nil {
		s.Fatal("Failed to expand the ubertray: ", err)
	}
}
