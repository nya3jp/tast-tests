// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// 23 Soft-disconnect a display

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
// 5) Press and Hold down (F6 - dimmer) button on top row keyboard until Primary screen turn off
// 6) Now use the Chromebook keyboard and Touchpad to navigate the Chrome browser on ""Ext-display"" extended screen
// 7) Now press and hold down (F7 - dimmer) button to turn Primary screen back ON

// Verification:
// 5) Make sure Chrome browser window bounce it to Ext- display ""Extended"" screen without any issue
// 6) Make sure Keyboard /Mouse/ Touchpad work without any issue
// 7) Make sure Chrome browser bounce back onto ""Primary"" screen without issue"

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
// 5. Click and open the Google Chrome browser from the bottom middle of the screen.
// 6. Input and navigate the video address ""https://www.youtube.com/watch?v=l4bDVq-nP-0&t=65s""
// 7. internal display power off
// 8. Run verification step 1 & 2.
// 9. internal display power on
// 10. Run verification step 3 & 4."

// Automation verification
// 1. Check window bounds on external display
// 2. Check Keyboard /Mouse/ Touchpad work without any issue (use tast Event)
// 3. Check window bounds on Primary display
// 4. Check the 1Khz video/audio playback  by test fixture.

// Package wwcb contains local Tast tests that work with chromebook
package wwcb

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/wwcb/utils"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Dock23DisconnectDisplay,
		Desc:         "Soft-disconnect a display",
		Contacts:     []string{"allion-sw@allion.com"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "arcBooted", //Boot-up and Sign-In to the device
		Timeout:      10 * time.Minute,
		Vars:         []string{"WWCBIP", "InternalDisplayCamera"},
	})
}

func Dock23DisconnectDisplay(ctx context.Context, s *testing.State) {

	p := s.FixtValue().(*arc.PreData)
	cr := p.Chrome
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	if err := utils.InitFixtures(ctx); err != nil {
		s.Fatal("Failed to initialize fixtures: ", err)
	}

	s.Log("Step 1 - Power the Chrombook On")

	s.Log("Step 2 - Sign-in account")

	// step 3 - connect ext-display to docking station
	if err := dock23DisconnectDisplayStep3(ctx, s); err != nil {
		s.Fatal("Failed to execute step 3: ", err)
	}

	// step 4 - connect docking station to chromebook
	if err := dock23DisconnectDisplayStep4(ctx, s); err != nil {
		s.Fatal("Failed to execute step 4: ", err)
	}

	// step 5, 6 - play youtube on internal display
	if err := dock23DisconnectDisplayStep5To6(ctx, s, cr, tconn); err != nil {
		s.Fatal("Failed to execute step 5, 6: ", err)
	}

	// step 7  - turn off internal display
	if err := dock23DisconnectDisplayStep7(ctx, s); err != nil {
		s.Fatal("Failed to execute step 7: ", err)
	}

	// step 8  - check youtube on external display
	if err := dock23DisconnectDisplayStep8(ctx, s, tconn); err != nil {
		s.Fatal("Failed to execute step 8: ", err)
	}

	// step 9  - check keyboard.. etc (using event)
	if err := dock23DisconnectDisplayStep9(ctx, s); err != nil {
		s.Fatal("Failed to execute step 9: ", err)
	}

	// step 10 - turn on internal display
	if err := dock23DisconnectDisplayStep10(ctx, s); err != nil {
		s.Fatal("Failed to execute step 10: ", err)
	}

	// step 11 - check youtube on primary display
	if err := dock23DisconnectDisplayStep11(ctx, s, tconn); err != nil {
		s.Fatal("Failed to execute step 11: ", err)
	}

	// step 12 - check playback
	if err := dock23DisconnectDisplayStep12(ctx, s, tconn); err != nil {
		s.Fatal("Failed to execute step 12: ", err)
	}

	// reset chromebook
	youtube, err := utils.GetYoutubeWindow(ctx, tconn)
	if err != nil {
		s.Log("Failed to get youtube window: ", err)
	}

	if err := youtube.CloseWindow(ctx, tconn); err != nil {
		s.Log("Failed to close youtube: ", err)
	}
}

func dock23DisconnectDisplayStep3(ctx context.Context, s *testing.State) error {

	s.Log("Step 3 - Connect the ext-display to docking station")

	if err := utils.ControlFixture(ctx, s, utils.ExtDisp1Type, utils.ExtDisp1Index, utils.ActionPlugin, false); err != nil {
		return errors.Wrap(err, "failed to connect ext-display to docking station")
	}

	return nil
}

func dock23DisconnectDisplayStep4(ctx context.Context, s *testing.State) error {

	s.Log("Step 4 - Connect the docking station to chromebook")

	if err := utils.ControlFixture(ctx, s, utils.ExtDisp1Type, utils.ExtDisp1Index, utils.ActionPlugin, false); err != nil {
		return errors.Wrap(err, "failed to connect docking station to chromebook")
	}

	return nil
}

func dock23DisconnectDisplayStep5To6(ctx context.Context, s *testing.State, cr *chrome.Chrome, tconn *chrome.TestConn) error {

	s.Log("Step 5, 6 - Play youtube")

	if err := utils.PlayYouTube(ctx, cr, tconn); err != nil {
		return errors.Wrap(err, "failed to play youtube")
	}

	return nil
}

func dock23DisconnectDisplayStep7(ctx context.Context, s *testing.State) error {

	s.Log("Step 7 - Internal display off ")

	// 7. internal display power off
	if err := utils.SetDisplayPower(ctx, utils.DisplayPowerInternalOffExternalOn); err != nil {
		return errors.Wrap(err, "failed to set internal display power off")
	}

	return nil
}

func dock23DisconnectDisplayStep8(ctx context.Context, s *testing.State, tconn *chrome.TestConn) error {

	s.Log("Step 8 - Check window bounds on external display ")

	// get display info
	infos, err := display.GetInfo(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get display info")
	}

	// check num of infos
	if len(infos) != 1 {
		return errors.Errorf("failed to get num of display , got %d, at least 1", len(infos))
	}

	// ensure youtube on ext-display 1
	if err := utils.EnsureYoutubeOnDisplay(ctx, s, tconn, &infos[0]); err != nil {
		return errors.Wrap(err, "failed to ensure youtube on ext-display1")
	}

	return nil
}

func dock23DisconnectDisplayStep9(ctx context.Context, s *testing.State) error {

	s.Log("Step 9 - Check Keyboard /Mouse/ Touchpad work ")

	// TODO-verify keyboard

	// TODO-verify mouse

	return nil
}

func dock23DisconnectDisplayStep10(ctx context.Context, s *testing.State) error {

	s.Log("Step 10 - Internal display power on ")

	// 10. internal display power on
	if err := utils.SetDisplayPower(ctx, utils.DisplayPowerAllOn); err != nil {
		return errors.Wrap(err, "failed to set internal display power on")
	}

	return nil
}

func dock23DisconnectDisplayStep11(ctx context.Context, s *testing.State, tconn *chrome.TestConn) error {

	s.Log("Step 11 - Check window bounds on Primary display ")

	// get primary display info
	primaryInfo, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get the primary display info")
	}

	// ensure youtbe on primary display
	if err := utils.EnsureYoutubeOnDisplay(ctx, s, tconn, primaryInfo); err != nil {
		return errors.Wrap(err, "failed to  ensure youtube on primary display")
	}

	return nil
}

func dock23DisconnectDisplayStep12(ctx context.Context, s *testing.State, tconn *chrome.TestConn) error {

	s.Log("Step 12 - Check playback ")

	if err := utils.CameraCheckPlayback(ctx, s, s.RequiredVar("InternalDisplayCamera")); err != nil {
		return errors.Wrap(err, "failed to check playback")
	}

	return nil
}
