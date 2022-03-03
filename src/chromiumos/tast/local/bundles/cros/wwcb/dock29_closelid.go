// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// 29 Close Chromebook Lid with Display connected.

// "Pre-Condition:
// (Please note: Brand / Model number on test result)
// 1. External displays (Single/Dual)
// 2. Docking station / Hub /Dongle /Adapter
// 3. Connection Type (HDMI/DP/VGA/DVI/USB-C on test result)

// Procedure:
// 1) Boot-up and Sign-In to the device
// 2) Connect ext-display onto (Dock/Hub/Dongle/Adapter)
// 3) Connect (Dock/Hub/Dongle/Adapter) onto Chromebook
// 4) Open Chrome browser: www.youtube.com and play any video on ""Primary"" screen
// 5) Close Chromebook Lid while video playback is playing on ""Primary"" screen
// Note: Chrome browser window will bounce onto ""Extended"" ext-display without any issue and video continues playing

// Verification:
// See Note: 5)

// "

/////////////////////////////////////////////////////////////////////////////////////
// automation
// "Preperation:
// 1. Monitor.
// 2. Chromebook
// 3. Docking Station
// 4. Type-C cable

// Test Step:
// 1. Power the Chrombook On.
// 2. Sign-in account.
// 3. Connect the external monitor to the docking station via Type-C cable. (Manual)
// 4. Connect the docking station to chromebook via Type-C cable. (switch Type-C & HDMI fixture)
// 5. Click and open the Google Chrome browser from the bottom middle of the screen. (open on primary display)
// 6. Input and navigate the video address ""https://www.youtube.com/watch?v=l4bDVq-nP-0&t=65s""
// 7. Close Chromebook lid.
// 8. Run verification."

// Automation verification
// "1. Check window bounds on external display
// 2. Check the 1Khz video/audio playback by test fixture."

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
		Func:         Dock29Closelid,
		Desc:         "Reconnect a previously used display",
		Contacts:     []string{"allion-sw@allion.com"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      10 * time.Minute,
		Pre:          chrome.LoggedIn(), //Boot-up and Sign-In to the device
		Vars:         []string{"WWCBIP", "ExternalDisplayCamera"},
	})
}

func Dock29Closelid(ctx context.Context, s *testing.State) { // chrome.LoggedIn()

	// set up
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

	// step 3 - connect ext-display to station
	if err := dock29CloselidStep3(ctx, s); err != nil {
		s.Fatal("Failed to execute step 3: ", err)
	}

	// step 4 - connect station to chromebook
	if err := dock29CloselidStep4(ctx, s); err != nil {
		s.Fatal("Failed to execute step 4: ", err)
	}

	// step 5, 6 - play youtube on primary display
	if err := dock29CloselidStep5To6(ctx, s, cr, tconn); err != nil {
		s.Fatal("Failed to execute step 5, 6: ", err)
	}

	// step 7 - power off internal display
	if err := dock29CloselidStep7(ctx, s); err != nil {
		s.Fatal("Failed to execute step 7: ", err)
	}

	// step 8 - check youtube on external display
	if err := dock29CloselidStep8(ctx, s, tconn); err != nil {
		s.Fatal("Failed to execute step 8: ", err)
	}

	// step 9 - check the 1Khz video/audio playback by test fixture.
	if err := dock29CloselidStep9(ctx, s, tconn); err != nil {
		s.Fatal("Failed to execute step 9: ", err)
	}

	// reset chromebook
	if err := utils.SetDisplayPower(ctx, utils.DisplayPowerAllOn); err != nil {
		s.Log("Failed to set all display power on: ", err)
	}

	youtube, err := utils.GetYoutubeWindow(ctx, tconn)
	if err != nil {
		s.Log("Failed to get youtube window: ", err)
	}

	if err := youtube.CloseWindow(ctx, tconn); err != nil {
		s.Log("Failed to close youtube: ", err)
	}

}

func dock29CloselidStep3(ctx context.Context, s *testing.State) error {

	s.Log("Step 3 - Connect the external monitor to the station ")

	if err := utils.ControlFixture(ctx, s, utils.ExtDisp1Type, utils.ExtDisp1Index, utils.ActionPlugin, false); err != nil {
		return errors.Wrap(err, "failed to connect ext-display to station")
	}

	return nil
}

func dock29CloselidStep4(ctx context.Context, s *testing.State) error {

	s.Log("Step 4 - Connect the station to chromebook")

	if err := utils.ControlFixture(ctx, s, utils.StationType, utils.StationIndex, utils.ActionPlugin, false); err != nil {
		return errors.Wrap(err, "failed to connect docking station to chromebook")
	}

	return nil
}

func dock29CloselidStep5To6(ctx context.Context, s *testing.State, cr *chrome.Chrome, tconn *chrome.TestConn) error {

	s.Log("Step 5, 6 - Play youtube")

	if err := utils.PlayYouTube(ctx, cr, tconn); err != nil {
		return errors.Wrap(err, "failed to play youtube")
	}

	// get primary info to compare
	primaryInfo, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get primary display info")
	}

	if err := utils.EnsureYoutubeOnDisplay(ctx, s, tconn, primaryInfo); err != nil {
		return errors.Wrap(err, "failed to ensure youtube on primary display")
	}

	return nil
}

func dock29CloselidStep7(ctx context.Context, s *testing.State) error {

	s.Log("Step 7 - Close chromebook lid")

	if err := utils.SetDisplayPower(ctx, utils.DisplayPowerInternalOffExternalOn); err != nil {
		return errors.Wrap(err, "failed to set internal display power off")
	}

	return nil
}

func dock29CloselidStep8(ctx context.Context, s *testing.State, tconn *chrome.TestConn) error {

	s.Log("Step 8 - Check window bounds on external display")

	// get display info
	infos, err := display.GetInfo(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get display info")
	}

	// ensure youtube on ext-display 1
	// but currently only can get only one display
	// so ext-display 1 will be infos[0]
	if err := utils.EnsureYoutubeOnDisplay(ctx, s, tconn, &infos[0]); err != nil {
		return errors.Wrap(err, "failed to ensure youtube on external display")
	}

	return nil
}

func dock29CloselidStep9(ctx context.Context, s *testing.State, tconn *chrome.TestConn) error {

	s.Log("Step 9 - Check the 1Khz video/audio playback on ext-display 1 by test fixture ")

	if err := utils.CameraCheckPlayback(ctx, s, s.RequiredVar("ExternalDisplayCamera")); err != nil {
		return errors.Wrap(err, "failed to check playback by test fixture")
	}

	return nil
}
