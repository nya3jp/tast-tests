// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// 18 Chromebook USB-C Out is MST Source

// "Precondition
// (Please note: Brand / Model number on test result)
// 1. External displays (Dual- DP port daisy chain)
// 2. Display (MST - support)
// 4. Connection Type (HDMI/DP/VGA/DVI/USB-C on test result)

// Procedure:
// 1) Boot-up and Sign-In to the device
// 2) Connect ext-display to Chromebook
// 3) Daisy chains one display to another display
// 4) Open Chrome Browser: www.youtube.com with Full screen window and play any video

// Verification:
// 4) Make sure both ""Extended"" ext-displays screen show (MST- Multi Stream Transport) and video/audio playback without any issue

// "

/////////////////////////////////////////////////////////////////////////////////////
// automation step
// "Preperation:
// 1. Two DP monitor.
// 2. Chromebook
// 3. Type-C to DP cable
// 4. DP to DP cable
// 5. Power outlet

// Test Step:
// 1. Power the Chrombook On.
// 2. Sign-in account.
// 3. Connect the 1st external monitor to the chromebook via Type-C to DP cable.(switch Type-C fixture)
// 4. Connect the 2nd external monitor to the 1st external monitor via DP cable. (Manual)
// 5. Click and open the Google Chrome browser from the bottom middle of the screen.(open on extend display)
// 6. Click the maximum icon in the upper right corner to display the Chrome browser in full screen.
// 7. Input and navigate the video address ""https://www.youtube.com/watch?v=l4bDVq-nP-0&t=65s""
// 8. Run verification."

// Automation verification
// "1. Check the 1st external monitor display properly by test fixture.
// 2. Check the 2nd external monitor display properly by test fixture.
// 3. Check both displays exist and screen mode is ""Extended"" or Chrome browser bounds on both displays
// 4. Check the 1Khz video/audio playback  by test fixture."

// Package wwcb contains local Tast tests that work with chromebook
package wwcb

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/wwcb/utils"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Dock18Mst,
		Desc:         "Chromebook USB-C Out is MST Source",
		Contacts:     []string{"flin@google.com", "newmanliu19020@allion.corp-partner.google.com"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      10 * time.Minute,
		Pre:          chrome.LoggedIn(),
		Vars:         []string{"WWCBIP", "DockingID", "1stExtDispID", "ExternalDisplayCamera"},
	})
}

func Dock18Mst(ctx context.Context, s *testing.State) {
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

	cleanup, err := utils.InitFixtures(ctx)
	if err != nil {
		s.Fatal("Failed to initialize fixtures: ", err)
	}
	defer cleanup(ctx)

	testing.ContextLog(ctx, "Step 1 - Power the Chrombook On")

	testing.ContextLog(ctx, "Step 2 - Sign-in account")

	// Step 3 - Connect 1st external displays via DP
	if err := dock18MstStep3(ctx, extDispID); err != nil {
		s.Fatal("Failed to execute step3: ", err)
	}

	// Step 4 - Connect 2nd external displays (manual)
	if err := dock18MstStep4(ctx); err != nil {
		s.Fatal("Failed to execute step4: ", err)
	}

	// Step 5 - Connect docking station to Chromebook
	if err := dock18MstStep5(ctx, dockingID); err != nil {
		s.Fatal("Failed to execute step5: ", err)
	}

	// Step 6, 7, 8 - play youtube on first external display
	if err := dock18MstStep6To8(ctx, cr, tconn, kb); err != nil {
		s.Fatal("Failed to execute step6, 7, 8: ", err)
	}

	// Step 9 - verification
	if err := dock18MstStep9(ctx, tconn, extDispID); err != nil {
		s.Fatal("Failed to execute step9: ", err)
	}
}

func dock18MstStep3(ctx context.Context, extDispID string) error {
	testing.ContextLog(ctx, "Step 3 - Connect the 1st external monitor to the chromebook via Type-C to DP cable.(switch Type-C fixture)")
	if err := utils.SwitchFixture(ctx, extDispID, "on", "0"); err != nil {
		return errors.Wrap(err, "failed to connect external display")
	}
	return nil
}

func dock18MstStep4(ctx context.Context) error {
	testing.ContextLog(ctx, "Step 4 - connect 2 Connect the 2nd external monitor to the 1st external monitor via DP cable. (Manual)")
	return nil
}

func dock18MstStep5(ctx context.Context, dockingID string) error {
	testing.ContextLog(ctx, "Step 5 - Connect station to chromebook")
	if err := utils.SwitchFixture(ctx, dockingID, "on", "0"); err != nil {
		return errors.Wrap(err, "failed to connect docking station")
	}
	return nil
}

func dock18MstStep6To8(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn, kb *input.KeyboardEventWriter) error {
	testing.ContextLog(ctx, "Step 6, 7, 8 - Play youtube on external display")

	// Switch window to external display.
	if err := testing.Poll(ctx, func(c context.Context) error {
		browser, err := ash.FindWindow(ctx, tconn, func(window *ash.Window) bool {
			return window.WindowType == ash.WindowTypeBrowser
		})
		if err != nil {
			return errors.Wrap(err, "failed to find browser")
		}

		if err := browser.ActivateWindow(ctx, tconn); err != nil {
			return errors.Wrap(err, "failed to activate browser")
		}

		if err := utils.SwitchWindowToDisplay(ctx, tconn, kb, true)(ctx); err != nil {
			return errors.Wrap(err, "failed to switch window to external display")
		}
		return nil
	}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
		return err
	}
	return nil
}

func dock18MstStep9(ctx context.Context, tconn *chrome.TestConn, extDispID string) error {

	testing.ContextLog(ctx, "Step 9 - Run verification")

	// 1. Check the 1st external monitor display properly by test fixture.
	// 2. Check the 2nd external monitor display properly by test fixture.
	if err := utils.VerifyDisplayCount(ctx, tconn, 3); err != nil {
		return errors.Wrap(err, "failed to verify display properly")
	}

	// get display info
	infos, err := display.GetInfo(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get display info")
	}

	// 3. Check both displays exist and screen mode is ""Extended"" or Chrome browser bounds on both displays
	for _, info := range infos {
		// when display is external
		if info.IsInternal == false {
			// check external display should be in extended mode
			if info.IsPrimary == true {
				return errors.New("External display should not be in primary mode")
			}

			// check external display is not in mirror mode
			if info.MirroringSourceID == infos[0].ID {
				return errors.New("External display should not be in mirror mode")
			}
		}
	}

	// check chrome browser on first external
	if err := utils.EnsureYoutubeOnDisplay(ctx, tconn, &infos[1]); err != nil {
		return errors.Wrapf(err, "failed to ensure youtube on first display - %s: ", infos[1].ID)
	}

	// 4. Check the 1Khz video/audio playback by test fixture."
	// tell wwcb server to record video with camera fixture
	videoPath, err := utils.VideoRecord(ctx, "60", extDispID)
	if err != nil {
		return errors.Wrap(err, "failed to video record")
	}
	// compare video with sample
	if err := utils.DetectVideo(ctx, videoPath); err != nil {
		return errors.Wrap(err, "failed to compare video with sample")
	}

	return nil
}
