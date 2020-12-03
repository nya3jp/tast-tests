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
	"chromiumos/tast/local/chrome/ui/quicksettings"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: QuickSettingsSignIn,
		Desc: "Checks the Quick Settings from SignIn screen",
		Contacts: []string{
			"chromeos-sw-engprod@google.com",
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

// QuickSettingsSignIn verifies Quick Settings and its contents from the signin(noLogin) screen.
func QuickSettingsSignIn(ctx context.Context, s *testing.State) {

	// NoLogin is used to land in signin screen.
	cr, err := chrome.New(
		ctx,
		chrome.NoLogin(),
		chrome.LoadSigninProfileExtension(s.RequiredVar("ui.signinProfileTestExtensionManifestKey")),
	)

	if err != nil {
		s.Fatal("Chrome startup failed: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.SigninProfileTestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating login test API connection failed: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)
	defer tconn.Close()

	// Skip OOBE before login screen
	oobeerr := cr.SkipOOBEBeforeLogin(ctx)
	if oobeerr != nil {
		s.Fatal("Failed to skip OOBE before login: ", oobeerr)
	}

	// Explicitly show Quick Settings ShowWithRetry on the signin screen, so it will
	// remain open for the UI verification steps.
	if err := quicksettings.ShowWithRetry(ctx, tconn, 10*time.Second); err != nil {
		s.Fatal("Failed to show quick settings : ", err)
	}
	defer quicksettings.Hide(ctx, tconn)

	// Get the common Quick Settings Nodes.
	checkNodes := quicksettings.GetQuickSettingsNodeParams(s.Param().(bool))

	// Add the MicGain Slider to the map for verification.
	checkNodes["MicGain Slider"] = quicksettings.MicGainSliderParams

	// Featured Pods specific to SignIn(noLogin) screen are added in the map for verification.
	podParams := map[string]quicksettings.SettingPod{
		"Accessibility pod": quicksettings.SettingPodAccessibility,
		"NetWork pod":       quicksettings.SettingPodNetwork,
		"Bluetooth pod":     quicksettings.SettingPodBluetooth,
		"Keyboard pod":      quicksettings.SettingPodKeyboard,
	}

	// Loop through all the Settingspod. Check whether the podParam exists in the Quick Settings UI.
	// If the pod exists,get the params value and add the pod and its podparams value to the checkNodes map.
	for pod, settingPod := range podParams {
		podParams, err := quicksettings.PodIconParams(settingPod)
		if err != nil {
			s.Fatal("Failed to get params for accessibility pod icon: ", err)
		} else {
			checkNodes[pod] = podParams
		}
	}

	// Loop through all the Quick Settings nodes and verify if they exist.
	for node, params := range checkNodes {
		if shown, err := ui.Exists(ctx, tconn, params); err != nil {
			s.Fatalf("Failed to check existence of %v: %v", node, err)
		} else if !shown {
			s.Errorf("%v was not found in the UI", node)
		}
	}
}
