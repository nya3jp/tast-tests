// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package quicksettings

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/quicksettings"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

// testParameters contains all the data needed to run a single test iteration.
type testParameters struct {
	Battery     bool
	AudioRecord bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         SignInScreen,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks the Quick Settings from SignIn screen",
		Contacts:     []string{"chromeos-sw-engprod@google.com", "cros-system-ui-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		Vars:         []string{"ui.signinProfileTestExtensionManifestKey"},
		Params: []testing.Param{
			{
				Name: "battery",
				Val: testParameters{
					Battery:     true,
					AudioRecord: false,
				},
				ExtraHardwareDeps: hwdep.D(hwdep.Battery()),
			},
			{
				Name: "audio",
				Val: testParameters{
					Battery:     false,
					AudioRecord: true,
				},
				ExtraHardwareDeps: hwdep.D(hwdep.Microphone()),
			},
			{
				Name: "noaudio_nobattery",
				Val: testParameters{
					Battery:     false,
					AudioRecord: false,
				},
			},
		},
	})
}

// SignInScreen verifies Quick Settings contents from the signin screen.
func SignInScreen(ctx context.Context, s *testing.State) {

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

	// Skip OOBE before Login screen.
	oobeConn, err := cr.WaitForOOBEConnection(ctx)
	if err != nil {
		s.Fatal("Failed to connect OOBE connection: ", err)
	}
	defer oobeConn.Close()

	if err := oobeConn.Eval(ctx, "Oobe.skipToLoginForTesting()", nil); err != nil {
		s.Fatal("Failed to skip to login: ", err)
	}

	if err := quicksettings.ShowWithRetry(ctx, tconn, 10*time.Second); err != nil {
		s.Fatal("Failed to show quick settings: ", err)
	}
	defer quicksettings.Hide(ctx, tconn)

	param := s.Param().(testParameters)

	// Get the common Quick Settings elements to verify.
	checkNodes, err := quicksettings.CommonElements(ctx, tconn, param.Battery, false /* isLockedScreen */)
	if err != nil {
		s.Fatal("Failed to get the params in SignInScreen: ", err)
	}

	// Loop through all the Quick Settings nodes and verify if they exist.
	for node, finder := range checkNodes {
		ui := uiauto.New(tconn)
		if err := ui.WaitUntilExists(finder)(ctx); err != nil {
			s.Fatalf("Failed to wait for %v node to exist: %v", node, err)
		}
		shown, err := ui.IsNodeFound(ctx, finder)
		if err != nil {
			s.Fatalf("Failed to find %v node: %v", node, err)
		}
		if !shown {
			s.Errorf("%v node was not shown in the UI", node)
		}
	}

	// The mic gain slider is on the audio settings subpage of Quick Settings. First navigate to Audio Settings subpage.
	// Later check that the MicGainSlider UI node is shown and display the slider level.
	if param.AudioRecord {
		_, err := quicksettings.SliderValue(ctx, tconn, quicksettings.SliderTypeMicGain)
		if err != nil {
			s.Fatal("Failed to get the MicGain Slider value from Audio Settings subpage: ", err)
		}
	}
}
