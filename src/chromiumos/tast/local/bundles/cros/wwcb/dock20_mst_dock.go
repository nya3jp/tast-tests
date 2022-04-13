// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// 20 Dock is MST Source

// "Precondition
// (Please note: Brand / Model number on test result)
// 1. External displays (Dual- DP port daisy chain)
// 2. Docking station
// 3. Connection Type (HDMI/DP/VGA/DVI/USB-C on test result)

// Procedure:
// 1) Boot-up and Sign-In to the device
// 2) Connect ext-display to Chromebook Docking station.
// 3) Connect (Docking station) to Chromebook
// 4) Daisy chains one display to another display
// 5) Open Chrome Browser: www.youtube.com with Full screen window and play any video

// Verification:
// 5) Make sure both ""Extended"" ext-displays screen show (MST- Multi Stream Transport) and video/audio playback without any issue

// "

/////////////////////////////////////////////////////////////////////////////////////
// automation step
// "Preperation:
// 1. Two DP monitor.
// 2. Chromebook
// 3. Docking Station
// 4. Type-C to DP cable
// 5. DP to DP cable

// Test Step:
// 1. Power the Chrombook On.
// 2. Sign-in account.
// 3. Connect the docking station to chromebook via Type-C cable. (switch Type-C & HDMI fixture)
// 4. Connect the 1st external monitor to the docking station via DP cable. (Manual)
// 5. Connect the 2nd external monitor to the 1st external monitor via DP cable. (Manual)
// 6. Click and open the Google Chrome browser from the bottom middle of the screen.(open on extend display)
// 7. Click the maximum icon in the upper right corner to display the Chrome browser in full screen.
// 8. Input and navigate the video address ""https://www.youtube.com/watch?v=l4bDVq-nP-0&t=65s""
// 9. Run verification."

// Automation verification
// 1. Check the 1st external monitor display properly by test fixture.
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
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Dock20MstDock,
		Desc:         "Dock is MST Source",
		Contacts:     []string{"allion-sw@allion.com"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      10 * time.Minute,
		Pre:          chrome.LoggedIn(), //Boot-up and Sign-In to the device
		Vars:         utils.InputArguments,
	})
}

func Dock20MstDock(ctx context.Context, s *testing.State) {

	// set up
	cr := s.PreValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to open connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	s.Log("Step 1 - Power the Chrombook On")

	s.Log("Step 2 - Sign-in account")

	// step 3 - connect docking station to chromebook
	if err := dock20MstDockStep3(ctx, s); err != nil {
		s.Fatal("Failed to execute step 3: ", err)
	}

	if err := dock20MstDockStep4(ctx, s); err != nil {
		s.Fatal("Failed to execute step4 : ", err)
	}

	s.Log("Step 5 - Connect the 2nd external monitor to the 1st external monitor via DP cable. (Manual) ")

	// step 6, 7, 8 - play youtube on ext-display1
	if err := dock20MstDockStep6To8(ctx, s, cr, tconn); err != nil {
		s.Fatal("Failed to execute step 6, 7, 8: ", err)
	}
	// step 9 - verification
	if err := dock20MstDockStep9(ctx, s, tconn); err != nil {
		s.Fatal("Failed to execute step 9: ", err)
	}
}

func dock20MstDockStep3(ctx context.Context, s *testing.State) error {

	s.Log("Step 3 - Connect docking station to chromebook")

	if err := utils.ControlFixture(ctx, s, utils.StationType, utils.StationIndex, utils.ActionPlugin, false); err != nil {
		return errors.Wrap(err, "failed to connect docking station to chromebook")
	}

	return nil
}

func dock20MstDockStep4(ctx context.Context, s *testing.State) error {

	s.Log("Step 4 - Connect the 1st external monitor to the docking station via DP cable")

	if err := utils.ControlFixture(ctx, s, utils.ExtDisp1Type, utils.ExtDisp1Index, utils.ActionPlugin, false); err != nil {
		return errors.Wrap(err, "failed to connect ext-display to docking station")
	}

	return nil
}

func dock20MstDockStep6To8(ctx context.Context, s *testing.State, cr *chrome.Chrome, tconn *chrome.TestConn) error {

	s.Log("Step 6, 7, 8 - Play youtube on ext-display")

	// call function to play youtube
	if err := utils.PlayYouTube(ctx, cr, tconn); err != nil {
		return errors.Wrap(err, "failed to play youtube")
	}

	infos, err := display.GetInfo(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get display info")
	}

	if err := testing.Poll(ctx, func(c context.Context) error {

		// (open on extend display)
		// get youtube window
		youtube, err := utils.GetYoutubeWindow(ctx, tconn)
		if err != nil {
			return errors.Wrap(err, "failed to get youtube window")
		}

		// move window form internal to external
		if err := utils.MoveWindowToDisplay(ctx, s, tconn, youtube, &infos[1]); err != nil {
			return errors.Wrap(err, "failed to move window between display")
		}

		return nil

	}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
		return err
	}

	return nil
}

func dock20MstDockStep9(ctx context.Context, s *testing.State, tconn *chrome.TestConn) error {

	s.Log("Step 9 - Run verification")

	// 1. Check the 1st external monitor display properly by test fixture.
	// 2. Check the 2nd external monitor display properly by test fixture.
	if err := utils.VerifyDisplayProperly(ctx, s, tconn, 3); err != nil {
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
	if err := utils.EnsureYoutubeOnDisplay(ctx, s, tconn, &infos[1]); err != nil {
		return errors.Wrapf(err, "failed to ensure youtube on first display - %s: ", infos[1].ID)
	}

	// 4. Check the 1Khz video/audio playback  by test fixture."
	if err := utils.CheckPlaybackByFixture(ctx, s, utils.ExternalDisplay1); err != nil {
		return errors.Wrap(err, "failed to check playback")
	}

	return nil
}
