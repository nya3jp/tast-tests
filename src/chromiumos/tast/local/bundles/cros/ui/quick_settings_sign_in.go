// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/chrome/ui/quicksettings"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: QuickSettingsSignIn,
		Desc: "Checks the Quick Settings from SignIn screen",
		Contacts: []string{
			"kyleshima@chromium.org",
			"bhansknecht@chromium.org",
			"dhaddock@chromium.org",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			ExtraHardwareDeps: hwdep.D(hwdep.Battery()),
			Val:               true,
		}, {
			Name: "no_battery",
			Val:  false,
		}},
		Vars: []string{
			"ui.signinProfileTestExtensionManifestKey",
		},
	})
}

// QuickSettingsSignIn verifies Quick Settings and its contents from the signin screen.
func QuickSettingsSignIn(ctx context.Context, s *testing.State) {

	// NoLogin is used to land in signin screen.
	cr, err := chrome.New(
		ctx,
		chrome.NoLogin(),
		chrome.LoadSigninProfileExtension(s.RequiredVar("ui.signinProfileTestExtensionManifestKey")),
	)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.SigninProfileTestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating login test API connection failed: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)
	defer tconn.Close()

	//Skipping the OOBE and navigating directly to the Login screen.
	oobeConn, err := cr.WaitForOOBEConnection(ctx)
	if err != nil {
		s.Fatal("Failed to create OOBE connection: ", err)
	}

	if err := oobeConn.Eval(ctx, "Oobe.skipToLoginForTesting()", nil); err != nil {
		s.Fatal("Failed to skip to login: ", err)
	}

	// Explicitly show Quick Settings on the signin screen, so it will
	// remain open for the UI verification steps.
	if err := quicksettings.Show(ctx, tconn); err != nil {
		s.Fatal("Failed to show quick settings : ", err)
	}
	defer quicksettings.Hide(ctx, tconn)

	// Check the network UI elements are shown in Quick Settings.
	networkParams, err := quicksettings.PodIconParams(quicksettings.SettingPodNetwork)
	if err != nil {
		s.Fatal("Failed to get params for network pod icon: ", err)
	}

	// Check the bluetooth UI elements are shown in Quick Settings.
	bluetoothParams, err := quicksettings.PodIconParams(quicksettings.SettingPodBluetooth)
	if err != nil {
		s.Fatal("Failed to get params for bluetoothParams pod icon: ", err)
	}

	// Associate the params with a descriptive name for better error reporting.
	checkNodes := map[string]ui.FindParams{
		"NetWork pod":       networkParams,
		"Bluetooth pod":     bluetoothParams,
		"Brightness slider": quicksettings.BrightnessSliderParams,
		"Volume slider":     quicksettings.VolumeSliderParams,
		"Shutdown button":   quicksettings.ShutdownBtnParams,
		"Collapse button":   quicksettings.CollapseBtnParams,
		"Date/time display": quicksettings.DateViewParams,
	}

	// Only check the battery display if the DUT has a battery.
	if s.Param().(bool) {
		checkNodes["Battery display"] = quicksettings.BatteryViewParams
	}

	//Loop through all the Quick Settings nodes and verify if they exist.
	for node, params := range checkNodes {
		if shown, err := ui.Exists(ctx, tconn, params); err != nil {
			s.Fatalf("Failed to check existence of %v: %v", node, err)
		} else if !shown {
			s.Errorf("%v was not found in the UI", node)
		}
	}
}
