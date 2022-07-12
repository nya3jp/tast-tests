// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// 21 Connect a new display

// "Pre-Condition:
// (Please note: Brand / Model number on test result)
// 1. External displays (Single/Dual)
// 2. Docking station / Hub /Dongle
// 3. Connection Type (HDMI/DP/VGA/DVI/USB-C on test result)

// Procedure:
// 1) Boot-up and Sign-In to the device
// 2) Connect ext-display to (Docking station)
// 3) Connect (Docking station) to Chromebook
// 4) Open Chrome Browser: www.youtube.com and play any video

// Verification:
// 1) Make sure video/audio playback without any issue and check for display connection performance.
// 2) Validate each applicable port (example, if checking USB-C ports, check all the USB-C ports, same applies to HDMI, DP etc)

// "

/////////////////////////////////////////////////////////////////////////////////////
// automation step
// "Preperation:
// 1. Monitor (Type-C, DP, HDMI).
// 2. Chromebook
// 3. Docking Station
// 4. Type-C cable
// 4. DP cable
// 5. HDMI cable

// Test Step:
// 1. Power the Chrombook On.
// 2. Sign-in account.
// 3. Connect the external monitor to the docking station via Type-C cable. (Manual)
// 4. Connect the docking station to chromebook via Type-C cable. (switch Type-C & HDMI fixture)
// 5. Click and open the Google Chrome browser from the bottom middle of the screen.
// 6. Input and navigate the video address ""https://www.youtube.com/watch?v=l4bDVq-nP-0&t=65s""
// 7. Run verification.

// 8. Repeat the test on each applicable Type-C port.
// 9. Repeat the test on each applicable DP port.
// 10. Repeat the test on each applicable HDMI port.
// 11. Repeat the test on other applicable video output connect."

// Automation verification
// 1. Check the external monitor display properly by test fixture.
// 2. Check external display exist and screen mode is ""Exetended""
// 3. Check the 1Khz video/audio playback  by test fixture."

// Package wwcb contains local Tast tests that work with chromebook
package wwcb

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/wwcb/utils"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Dock21ConnectDisplay,
		Desc:         "Connect a new display",
		Contacts:     []string{"flin@google.com", "newmanliu19020@allion.corp-partner.google.com"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      10 * time.Minute, // was 15
		Pre:          chrome.LoggedIn(),
		Vars:         []string{"WWCBIP", "DockingID", "1stExtDispID", "ExternalDisplayCamera"},
	})
}

func Dock21ConnectDisplay(ctx context.Context, s *testing.State) {
	extDispID := s.RequiredVar("1stExtDispID")
	dockingID := s.RequiredVar("DockingID")

	cr := s.PreValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to open connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer kb.Close()

	if err := utils.InitFixtures(ctx); err != nil {
		s.Fatal("Failed to initialize fixtures: ", err)
	}

	testing.ContextLog(ctx, "Step 1 - Power the Chrombook On")

	testing.ContextLog(ctx, "Step 2 - Sign-in account")

	// step 3 - connect ext-display to docking
	if err := dock21ConnectDisplayStep3(ctx, extDispID); err != nil {
		s.Fatal("Failed to execute step 3: ", err)
	}

	// step 4 - connect docking to chromebook
	if err := dock21ConnectDisplayStep4(ctx, dockingID); err != nil {
		s.Fatal("Failed to execute step 4: ", err)
	}

	// step 5, 6 - play youtube on ext-display
	if err := dock21ConnectDisplayStep5To6(ctx, cr, tconn, kb); err != nil {
		s.Fatal("Failed to execute step 5, 6: ", err)
	}

	// step 7 - verification
	if err := dock21ConnectDisplayStep7(ctx, s, tconn); err != nil {
		s.Fatal("Failed to execute step 7: ", err)
	}
}

func dock21ConnectDisplayStep3(ctx context.Context, extDispID string) error {
	testing.ContextLog(ctx, "Step 3 - Connect ext-display to the docking station")
	if err := utils.SwitchFixture(ctx, extDispID, "on", "0"); err != nil {
		return errors.Wrap(err, "failed to connect external display")
	}
	return nil
}

func dock21ConnectDisplayStep4(ctx context.Context, dockingID string) error {
	testing.ContextLog(ctx, "Step 4 - Connect the docking station to chromebook")
	if err := utils.SwitchFixture(ctx, dockingID, "on", "0"); err != nil {
		return errors.Wrap(err, "failed to connect docking station")
	}
	return nil
}

func dock21ConnectDisplayStep5To6(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn, kb *input.KeyboardEventWriter) error {
	testing.ContextLog(ctx, "Step 5, 6 - Play youtube on ext-display")
	// call function to play youtube
	if err := utils.PlayYouTube(ctx, cr, tconn); err != nil {
		return errors.Wrap(err, "failed to play youtube")
	}
	// retry in 30s
	if err := testing.Poll(ctx, func(c context.Context) error {
		// (open on extend display)
		// get youtube window
		youtube, err := utils.GetYoutubeWindow(ctx, tconn)
		if err != nil {
			return errors.Wrap(err, "failed to get youtube window")
		}
		if err := youtube.ActivateWindow(ctx, tconn); err != nil {
			return errors.Wrap(err, "failed to activate youtube window")
		}
		// move window form internal to external
		if err := utils.SwitchWindowToDisplay(ctx, tconn, kb, true)(ctx); err != nil {
			return errors.Wrap(err, "failed to switch window to external display")
		}
		return nil
	}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
		return err
	}
	return nil
}

func dock21ConnectDisplayStep7(ctx context.Context, s *testing.State, tconn *chrome.TestConn) error {

	testing.ContextLog(ctx, "Step 7 - Run verification")

	// 1. check the external monitor display properly by test fixture
	if err := utils.VerifyDisplayProperly(ctx, tconn, 2); err != nil {
		return errors.Wrap(err, "failed to verify display properly")
	}

	// get display info
	infos, err := display.GetInfo(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get display info")
	}

	// 2. Check external display exist and screen mode is "Exetended"
	for _, info := range infos {
		// check external
		if info.IsInternal == false {
			// check extended
			if info.IsPrimary == true {
				return errors.New("External display should not be in primary mode")
			}

			// check mirror
			if info.MirroringSourceID == infos[0].ID {
				return errors.New("External display should not be in mirror mode")
			}
		}
	}

	// 3. Check the 1Khz video/audio playback on ext-display by test fixture."
	if err := utils.CameraCheckPlayback(ctx, s, s.RequiredVar("ExternalDisplayCamera")); err != nil {
		return errors.Wrap(err, "failed to check playback on ext-display1 by test fixture")
	}

	return nil
}
