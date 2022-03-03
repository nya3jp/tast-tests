// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// 22 Physically disconnect a display

// "Pre-Condition:
// (Please note: Brand / Model number on test result)
// 1. External displays (Single/Dual)
// 2. Docking station / Hub /Dongle
// 3. Connection Type (HDMI/DP/VGA/DVI/USB-C on test result)

// Procedure:
// 1) Boot-up and Sign-In to the device
// 2) Connect ext-display to (Docking station)
// 3) Connect (Dock station) to Chromebook
// 4) Open Chrome browser: www.youtube.com and play any video
// 5) While video is playing drag the Chrome browser window onto ""Extended"" ext-display
// 6) Disconnect the (Dock station) from Chromebook while video still playing

// Verification:
// 6) Make sure Chrome browser window bound it back to Chromebook ""Primary"" screen without any issue

// "

/////////////////////////////////////////////////////////////////////////////////////
// automation step
// "Preperation:
// 1. Monitor (Type-C)
// 2. Chromebook
// 3. Docking Station
// 4. Type-C cable

// Test Step:
// 1. Power the Chrombook On.
// 2. Sign-in account.
// 3. Connect the external monitor to the docking station via Type-C cable. (Manual)
// 4. Connect the docking station to chromebook via Type-C cable. (switch Type-C & HDMI fixture)
// 5. Run verification step 1.
// 6. Click and open the Google Chrome browser from the bottom middle of the screen.(open on extend display)
// 7. Input and navigate the video address ""https://www.youtube.com/watch?v=l4bDVq-nP-0&t=65s""
// 8. Run verification step 3
// 9. Disconnect the docking station from chromebook.
// 10. Run verification step 2 & 3.4"

// Automation verification
// 1. Check the external monitor display properly by test fixture.
// 2. Check external display exist and screen mode is ""Exetended""
// 3. Check the 1Khz video/audio playback  by test fixture.
// 4. Check Chrome browser window bounds it back to ""Primary"" screen"

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
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Dock22DisconnectDisplay,
		Desc:         "Physically disconnect a display",
		Contacts:     []string{"allion-sw@allion.com"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      20 * time.Minute,
		Pre:          chrome.LoggedIn(), //Boot-up and Sign-In to the device
		Vars:         []string{"WWCBIP", "InternalDisplayCamera", "ExternalDisplayCamera"},
	})
}

func Dock22DisconnectDisplay(ctx context.Context, s *testing.State) {

	cr := s.PreValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to open connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	if err := utils.InitFixtures(ctx); err != nil {
		s.Fatal("Failed to initialize fixtures: ", err)
	}

	s.Log("Step 1 - Power the Chrombook On")

	s.Log("Step 2 - Sign-in account")

	// step 3 - connect ext-display to docking
	if err := dock22DisconnectDisplayStep3(ctx, s); err != nil {
		s.Fatal("Failed to execute step 3: ", err)
	}

	// step 4 - connect docking station to chromebook
	if err := dock22DisconnectDisplayStep4(ctx, s, tconn); err != nil {
		s.Fatal("Failed to execute step 4: ", err)
	}

	// step 5 - check ext-display
	if err := dock22DisconnectDisplayStep5(ctx, s, tconn); err != nil {
		s.Fatal("Failed to execute step 5: ", err)
	}

	// step 6, 7 - open youtube on ext-display1
	if err := dock22DisconnectDisplayStep6To7(ctx, s, cr, tconn); err != nil {
		s.Fatal("Failed to execute step 6, 7: ", err)
	}

	// step 8 - check playback on ext-display1
	if err := dock22DisconnectDisplayStep8(ctx, s, tconn); err != nil {
		s.Fatal("Failed to execute Step 8: ", err)
	}

	// step 9 - disconnect docking
	if err := dock22DisconnectDisplayStep9(ctx, s, tconn); err != nil {
		s.Fatal("Failed to execute step 9: ", err)
	}

	// step 10 - check external display is not exist
	if err := dock22DisconnectDisplayStep10(ctx, s, tconn); err != nil {
		s.Fatal("Failed to execute step 10: ", err)
	}

	// step 11 - check playback on primary display
	if err := dock22DisconnectDisplayStep11(ctx, s, tconn); err != nil {
		s.Fatal("Failed to execute step 11: ", err)
	}

	// step 12 - check youtube browser on primary display
	if err := dock22DisconnectDisplayStep12(ctx, s, tconn); err != nil {
		s.Fatal("Failed to execute step 12: ", err)
	}

}

func dock22DisconnectDisplayStep3(ctx context.Context, s *testing.State) error {

	s.Log("Step 3 - Connect the external monitor to the docking station via Type-C cable. (Manual)")

	if err := utils.ControlFixture(ctx, s, utils.ExtDisp1Type, utils.ExtDisp1Index, utils.ActionPlugin, false); err != nil {
		return errors.Wrap(err, "failed to connect ext-display to docking station")
	}

	return nil
}

func dock22DisconnectDisplayStep4(ctx context.Context, s *testing.State, tconn *chrome.TestConn) error {

	s.Log("Step 4 - Connect the docking station to chromebook via Type-C cable. (switch Type-C & HDMI fixture) ")

	if err := utils.ControlFixture(ctx, s, utils.StationType, utils.StationIndex, utils.ActionPlugin, false); err != nil {
		return errors.Wrap(err, "failed to connect docking station to chromebook")
	}

	return nil
}

func dock22DisconnectDisplayStep5(ctx context.Context, s *testing.State, tconn *chrome.TestConn) error {

	s.Log("Step 5 - Check external monitor display properly by test fixture")

	if err := utils.VerifyDisplayProperly(ctx, tconn, 2); err != nil {
		return errors.Wrap(err, "failed to verify display properly")
	}

	return nil
}

func dock22DisconnectDisplayStep6To7(ctx context.Context, s *testing.State, cr *chrome.Chrome, tconn *chrome.TestConn) error {

	s.Log("Step 6, 7 - Play youtube on ext-display 1")

	// call function to play youtube
	if err := utils.PlayYouTube(ctx, cr, tconn); err != nil {
		return errors.Wrap(err, "failed to play youtube")
	}

	// retry in 30s
	if err := testing.Poll(ctx, func(c context.Context) error {

		testing.Sleep(ctx, time.Second)

		infos, err := display.GetInfo(ctx, tconn)
		if err != nil {
			return errors.Wrap(err, "failed to get display info")
		}

		// (open on extend display)
		// get youtube window
		youtube, err := utils.GetYoutubeWindow(ctx, tconn)
		if err != nil {
			return errors.Wrap(err, "failed to get youtube window")
		}

		// move window form internal to external
		if err := utils.MoveWindowToDisplay(ctx, tconn, youtube, &infos[1]); err != nil {
			return errors.Wrap(err, "failed to move window ext-display")
		}

		return nil
	}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
		return err
	}

	return nil
}

func dock22DisconnectDisplayStep8(ctx context.Context, s *testing.State, tconn *chrome.TestConn) error {

	s.Log("Step 8 - Check the 1Khz video/audio playback on ext-display 1 by test fixture")

	// check playback
	if err := utils.CameraCheckPlayback(ctx, s, s.RequiredVar("ExternalDisplayCamera")); err != nil {
		return errors.Wrap(err, "failed to check playback")
	}

	return nil
}

func dock22DisconnectDisplayStep9(ctx context.Context, s *testing.State, tconn *chrome.TestConn) error {

	s.Log("Step 9 - Disconnect the docking station from chromebook. ")

	if err := utils.ControlFixture(ctx, s, utils.StationType, utils.StationIndex, utils.ActionUnplug, false); err != nil {
		return errors.Wrap(err, "failed to disconnect docking station from chromebook")
	}

	return nil
}

func dock22DisconnectDisplayStep10(ctx context.Context, s *testing.State, tconn *chrome.TestConn) error {

	s.Log("Step 10 - Check external display is not exist and check internal display becomes to be primary")

	// get display info
	infos, err := display.GetInfo(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get display info")
	}

	// 10. Check external display exist and screen mode is ""Exetended""
	if len(infos) != 1 {
		return errors.Errorf("Length of display is not enough, got %d, want 1", len(infos))
	}

	for _, info := range infos {
		// check external
		if info.IsInternal == false {
			// check extended
			if info.IsPrimary == true {
				return errors.New("External display should not be in primary mode")
			}
		}
	}

	return nil
}

func dock22DisconnectDisplayStep11(ctx context.Context, s *testing.State, tconn *chrome.TestConn) error {

	s.Log("Step 11 - Check the 1Khz video/audio playback on internal display by test fixture ")

	// 11. Check the 1Khz video/audio playback  by test fixture.
	if err := utils.CameraCheckPlayback(ctx, s, s.RequiredVar("InternalDisplayCamera")); err != nil {
		return errors.Wrap(err, "failed to check playback by fixture")
	}

	return nil
}

func dock22DisconnectDisplayStep12(ctx context.Context, s *testing.State, tconn *chrome.TestConn) error {

	s.Log("Step 12 - Check Chrome browser window bounds it back to primary screen")

	// get primary info
	primaryInfo, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get the primary display info")
	}

	// 12. Check Chrome browser window bounds it back to ""Primary"" screen"
	if err := utils.EnsureYoutubeOnDisplay(ctx, s, tconn, primaryInfo); err != nil {
		return errors.Wrap(err, "failed to ensure youtube on primary display")
	}

	return nil
}
