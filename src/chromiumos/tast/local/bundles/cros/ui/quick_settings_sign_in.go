// Copyright 2021 The Chromium OS Authors. All rights reserved.
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
		Func: QuickSettingsSignIn,
		Desc: "Checks the Quick Settings from SignIn screen",
		Contacts: []string{
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
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

// QuickSettingsSignIn verifies Quick Settings contents from the signin screen.
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

	// Show Quicksettings.
	if err := quicksettings.ShowWithRetry(ctx, tconn, 10*time.Second); err != nil {
		s.Fatal("Failed to show quick settings : ", err)
	}
	defer quicksettings.Hide(ctx, tconn)

	// Get the common Quick Settings elements.
	checkNodes := quicksettings.CommonElementsExist(s.Param().(bool))

	// Featured Pods specific to SignIn screen are added in the map to get the ui.FindParams.
	signinscreenPods := map[string]quicksettings.SettingPod{
		"Accessibility pod": quicksettings.SettingPodAccessibility,
		"NetWork pod":       quicksettings.SettingPodNetwork,
		"Keyboard pod":      quicksettings.SettingPodKeyboard,
	}

	// Add the bluetooth pod to the signinscreenPods map, only if they have min of 1 adapter.
	if adapters, err := bluetooth.Adapters(ctx); err != nil {
		s.Fatal("Unable to get Bluetooth adapters: ", err)
	} else if len(adapters) > 0 {
		signinscreenPods["Bluetooth pod"] = quicksettings.SettingPodBluetooth
	}

	// Loop through all the Settingspod and generate the ui.FindParams for the specified quick settings pod.
	// Add the podParams to the checkNodes map.
	for pod, settingPod := range signinscreenPods {
		podParams, err := quicksettings.PodIconParams(settingPod)
		if err != nil {
			s.Fatalf("Failed to get params for %v pod icon: %v", podParams, err)
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

	// The mic gain slider is on the audio settings subpage of Quick Settings. First navigate to Audio Settings subpage.
	// Later check that the MicGainSlider UI node is shown and display the slider level.
	sliderLevel, err := quicksettings.SliderValue(ctx, tconn, quicksettings.SliderTypeMicGain)
	if err != nil {
		s.Fatal("Failed to get the MicGain Slider value from Audio Settings subpage : ", err)
	}
	s.Log("The Microphone gain slider level in the Audio Settings subpage : ", sliderLevel)

}
