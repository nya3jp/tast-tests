// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// // "Pre-Condition:// "Pre-Condition:27 Test DVI & VGA Port

// "Pre-Condition:
// (Please note: Brand / Model number on test result)
// 1. External displays (Single/Dual - DVI/VGA support)
// 2. Docking station / Hub /Dongle (DVI/VGA support)
// 3. Connection Type (HDMI/DP/VGA/DVI/USB-C on test result)

// Procedure:
// 1) Boot-up and Sign-In to the device
// 2) Connect ext-display to (Dock/Hub)
// 3) Connect (Dock/Hub) to Chromebook
// 4) Open Chrome browser: www.youtube.com and play any video
// 5) While video is playing drag the Chrome browser window onto ""Extended"" ext-display

// Verification:
// 4) Make sure bolt ""Primary and Extended"" ext-display show up without any issue and video playback successfully ""Primary"" screen
// 5) Make sure video playback without any issue on ""Extended"" ext-display

// "

/////////////////////////////////////////////////////////////////////////////////////
// automation
// "Preperation:
// 1. Monitor (DVI).
// 2. Chromebook
// 3. Docking Station
// 4. Type-C cable
// 5. DVI cable
// 6. VGA cable

// Test Step:
// 1. Power the Chrombook On.
// 2. Sign-in account.
// 3. Connect the external monitor to the docking station via DVI cable. (Manual)
// 4. Connect the docking station to chromebook via Type-C cable. (switch Type-C & HDMI fixture)
// 5. Click and open the Google Chrome browser from the bottom middle of the screen.(open on primary display)
// 6. Input and navigate the video address ""https://www.youtube.com/watch?v=l4bDVq-nP-0&t=65s""
// 7. Run verification step 1~2.
// 8. Drag the Google Chrome browser from chromebook display to external monitor.
// 9. Run verification step 2

// 10. Repeat the test on applicable VGA port."

// Automation verification
// "1. Check the external monitor display properly by test fixture.
// 2. Check the 1Khz video/audio playback  by test fixture."

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
		Func:         Dock27TestPort,
		Desc:         "Test DVI & VGA Port",
		Contacts:     []string{"flin@google.com", "newmanliu19020@allion.corp-partner.google.com"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      10 * time.Minute,
		Pre:          chrome.LoggedIn(),
		Vars:         []string{"WWCBIP", "DockingID", "1stExtDispID", "ExternalDisplayCamera"},
	})
}

func Dock27TestPort(ctx context.Context, s *testing.State) { // chrome.LoggedIn()
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

	// step 3 - connect ext-display to docking
	if err := dock27TestPortStep3(ctx, extDispID); err != nil {
		s.Fatal("Failed to execute step 3: ", err)
	}
	// step 4 - connect docking to chromebook
	if err := dock27TestPortStep4(ctx, dockingID); err != nil {
		s.Fatal("Failed to execute step 4: ", err)
	}
	// step 5, 6 - play youtube on primary display
	if err := dock27TestPortStep5To6(ctx, cr, tconn); err != nil {
		s.Fatal("Failed to execute step 5, 6: ", err)
	}
	// step 7 - check external display exist & check ext-display properly
	if err := dock27TestPortStep7(ctx, tconn); err != nil {
		s.Fatal("Failed to execute step 7: ", err)
	}
	// step 8 - drag youtube to external
	if err := dock27TestPortStep8(ctx, tconn, kb); err != nil {
		s.Fatal("Failed to execute step 8: ", err)
	}
	// step 9 - check the 1Khz video/audio playback by test fixture.
	if err := dock27TestPortStep9(ctx, extDispID); err != nil {
		s.Fatal("Failed to execute step 9: ", err)
	}
}

func dock27TestPortStep3(ctx context.Context, extDispID string) error {
	testing.ContextLog(ctx, "Step 3 - Connect the external monitor to the docking station via DVI cable (Manually)")
	if err := utils.SwitchFixture(ctx, extDispID, "on", "0"); err != nil {
		return errors.Wrap(err, "failed to connect external display")
	}
	return nil
}

func dock27TestPortStep4(ctx context.Context, dockingID string) error {
	testing.ContextLog(ctx, "Step 4 - Connect the docking station to chromebook")
	if err := utils.SwitchFixture(ctx, dockingID, "on", "0"); err != nil {
		return errors.Wrap(err, "failed to connect docking station")
	}
	return nil
}

func dock27TestPortStep5To6(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn) error {
	testing.ContextLog(ctx, "Step 5, 6 - Play youtube on primary display")
	if err := utils.PlayYouTube(ctx, cr, tconn); err != nil {
		return errors.Wrap(err, "failed to play youtube")
	}
	primaryInfo, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get primary display info")
	}
	if err := utils.EnsureYoutubeOnDisplay(ctx, tconn, primaryInfo); err != nil {
		return errors.Wrap(err, "failed to ensure youtbe on primary display")
	}
	return nil
}

func dock27TestPortStep7(ctx context.Context, tconn *chrome.TestConn) error {
	testing.ContextLog(ctx, "Step 7 - Check the external monitor display properly by test fixture")
	if err := utils.VerifyDisplayCount(ctx, tconn, 2); err != nil {
		return errors.Wrap(err, "failed to verify display properly")
	}
	return nil
}

func dock27TestPortStep8(ctx context.Context, tconn *chrome.TestConn, kb *input.KeyboardEventWriter) error {
	testing.ContextLog(ctx, "Step 8 - Drag the browser from internal display to external monitor")
	if err := testing.Poll(ctx, func(c context.Context) error {
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

func dock27TestPortStep9(ctx context.Context, extDispID string) error {
	testing.ContextLog(ctx, "Step 9 - Check the 1Khz video/audio playback by test fixture")
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
