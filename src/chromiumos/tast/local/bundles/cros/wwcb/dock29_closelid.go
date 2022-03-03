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
		Contacts:     []string{"flin@google.com", "newmanliu19020@allion.corp-partner.google.com"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      10 * time.Minute,
		Pre:          chrome.LoggedIn(),
		Vars:         []string{"WWCBIP", "DockingID", "1stExtDispID", "ExternalDisplayCamera"},
	})
}

func Dock29Closelid(ctx context.Context, s *testing.State) { // chrome.LoggedIn()
	extDispID := s.RequiredVar("1stExtDispID")
	dockingID := s.RequiredVar("DockingID")

	cr := s.PreValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to open connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	cleanup, err := utils.InitFixtures(ctx)
	if err != nil {
		s.Fatal("Failed to initialize fixtures: ", err)
	}
	defer cleanup(ctx)

	testing.ContextLog(ctx, "Step 1 - Power the Chrombook On")

	testing.ContextLog(ctx, "Step 2 - Sign-in account")

	// step 3 - connect ext-display to station
	if err := dock29CloselidStep3(ctx, extDispID); err != nil {
		s.Fatal("Failed to execute step 3: ", err)
	}
	// step 4 - connect station to chromebook
	if err := dock29CloselidStep4(ctx, dockingID); err != nil {
		s.Fatal("Failed to execute step 4: ", err)
	}
	if err := utils.VerifyDisplayCount(ctx, tconn, 2); err != nil {
		s.Fatal("Failed to verify display properly: ", err)
	}
	infos, err := utils.GetInternalAndExternalDisplays(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get internal & external display: ", err)
	}
	// step 5, 6 - play youtube on primary display
	if err := dock29CloselidStep5To6(ctx, cr, tconn); err != nil {
		s.Fatal("Failed to execute step 5, 6: ", err)
	}
	// step 7 - power off internal display
	if err := dock29CloselidStep7(ctx); err != nil {
		s.Fatal("Failed to execute step 7: ", err)
	}
	// step 8 - check youtube on external display
	if err := dock29CloselidStep8(ctx, tconn, &infos.External); err != nil {
		s.Fatal("Failed to execute step 8: ", err)
	}
	// step 9 - check the 1Khz video/audio playback by test fixture.
	if err := dock29CloselidStep9(ctx, extDispID); err != nil {
		s.Fatal("Failed to execute step 9: ", err)
	}
	// reset chromebook
	if err := utils.SetDisplayPower(ctx, utils.DisplayPowerAllOn); err != nil {
		testing.ContextLog(ctx, "Failed to set all display power on: ", err)
	}
	youtube, err := utils.GetYoutubeWindow(ctx, tconn)
	if err != nil {
		testing.ContextLog(ctx, "Failed to get youtube window: ", err)
	}
	if err := youtube.CloseWindow(ctx, tconn); err != nil {
		testing.ContextLog(ctx, "Failed to close youtube: ", err)
	}
}

func dock29CloselidStep3(ctx context.Context, extDispID string) error {
	testing.ContextLog(ctx, "Step 3 - Connect the external monitor to the station")
	if err := utils.SwitchFixture(ctx, extDispID, "on", "0"); err != nil {
		return errors.Wrap(err, "failed to connect external display")
	}
	return nil
}

func dock29CloselidStep4(ctx context.Context, dockingID string) error {
	testing.ContextLog(ctx, "Step 4 - Connect the station to chromebook")
	if err := utils.SwitchFixture(ctx, dockingID, "on", "0"); err != nil {
		return errors.Wrap(err, "failed to connect docking station")
	}
	return nil
}

func dock29CloselidStep5To6(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn) error {
	testing.ContextLog(ctx, "Step 5, 6 - Play youtube")
	if err := utils.PlayYouTube(ctx, cr, tconn); err != nil {
		return errors.Wrap(err, "failed to play youtube")
	}
	primaryInfo, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get primary display info")
	}
	if err := utils.EnsureYoutubeOnDisplay(ctx, tconn, primaryInfo); err != nil {
		return errors.Wrap(err, "failed to ensure youtube on primary display")
	}
	return nil
}

func dock29CloselidStep7(ctx context.Context) error {
	testing.ContextLog(ctx, "Step 7 - Close chromebook lid")
	if err := utils.SetDisplayPower(ctx, utils.DisplayPowerInternalOffExternalOn); err != nil {
		return errors.Wrap(err, "failed to set internal display power off")
	}
	return nil
}

func dock29CloselidStep8(ctx context.Context, tconn *chrome.TestConn, extDispInfo *display.Info) error {
	testing.ContextLog(ctx, "Step 8 - Check window bounds on external display")
	// ensure youtube on ext-display 1
	if err := utils.EnsureYoutubeOnDisplay(ctx, tconn, extDispInfo); err != nil {
		return errors.Wrap(err, "failed to ensure youtube on external display")
	}
	return nil
}

func dock29CloselidStep9(ctx context.Context, extDispID string) error {
	testing.ContextLog(ctx, "Step 9 - Check the 1Khz video/audio playback on ext-display 1 by test fixture")
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
