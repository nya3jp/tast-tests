// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// 28 Reconnect a previously used display

// "Pre-Condition:
// (Please note: Brand / Model number on test result)
// 1. External displays (Single/Dual)
// 2. Docking station / Hub /Dongle /Adapter
// 3. Connection Type (HDMI/DP/VGA/DVI/USB-C on test result)

// Procedure:
// 1) Boot-up and Sign-In to the device
// 2) Connect ext-display onto (Dock/Hub/Dongle/Adapter)
// 3) Connect (Dock/Hub/Dongle/Adapter) onto Chromebook
// 4) Open Chrome browser: www.youtube.com and play any video
// 5) While video is playing drag the Chrome browser window onto ""Extended"" ext-display
// 6) Disconnect ext-display from (Dock/Hub/Dongle/Adapter)
// Note: Chrome browser window will bounce back onto ""Primary"" screen on Chromebook
// 7) Reconnect ext-display back onto (Dock/Hub/Dongle/Adapter)
// Note: Chrome browser window will bounce back onto ""Extended"" ext-display without any issue

// Verification:
// See Note: 6), 7)
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
// 4. Connect the docking station to chromebook via Type-C cable. (switch Type-C & HDMI fixturel)
// 5. Click and open the Google Chrome browser from the bottom middle of the screen.(open on external display)
// 6. Input and navigate the video address ""https://www.youtube.com/watch?v=l4bDVq-nP-0&t=65s""
// 7. Drag the Google Chrome browser from chromebook display to external monitor.
// 8. Run verification step 1.
// 9. Disconnect the external monitor from docking station. (switch Type-C & HDMI fixture)
// 10. Run verification step 2.
// 11. Connect the external monitor to docking station. (switch Type-C & HDMI fixture)
// 12. Run verification step 3 & 4."

// Automation verification
// "1. Check window bounds on external display.
// 2. Check window bounds on primary display.
// 3. Check window bounds on external display.
// 4. Check the 1Khz video/audio playback by test fixture."

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
		Func:         Dock28ReconnectDisplay,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Reconnect a previously used display",
		Contacts:     []string{"flin@google.com", "newmanliu19020@allion.corp-partner.google.com"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      10 * time.Minute,
		Pre:          chrome.LoggedIn(),
		Vars:         []string{"WWCBIP", "DockingID", "1stExtDispID", "ExternalDisplayCamera"},
	})
}

func Dock28ReconnectDisplay(ctx context.Context, s *testing.State) { // chrome.LoggedIn()
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

	// step 3 - connect ext-display to station
	if err := dock28ReconnectDisplayStep3(ctx, extDispID); err != nil {
		s.Fatal("Failed to execute step 3: ", err)
	}
	// step 4 - connect station to chromebook
	if err := dock28ReconnectDisplayStep4(ctx, dockingID); err != nil {
		s.Fatal("Failed to execute step 4: ", err)
	}
	// step 5, 6, 7, 8 - play youtube on exteranl display
	if err := dock28ReconnectDisplayStep5To8(ctx, cr, tconn, kb); err != nil {
		s.Fatal("Failed to execute step 5, 6, 7, 8: ", err)
	}
	// step 9 - disconnect ext-display from docking
	if err := dock28ReconnectDisplayStep9(ctx, extDispID); err != nil {
		s.Fatal("Failed to execute step 9: ", err)
	}
	// step 10 - check youtube bounds on primary display
	if err := dock28ReconnectDisplayStep10(ctx, tconn); err != nil {
		s.Fatal("Failed to execute step 10: ", err)
	}
	// step 11 - connect ext-display to docking
	if err := dock28ReconnectDisplayStep11(ctx, extDispID); err != nil {
		s.Fatal("Failed to execute step 11: ", err)
	}
	// step 12 - check youtube bounds on external display
	if err := dock28ReconnectDisplayStep12(ctx, tconn); err != nil {
		s.Fatal("Failed to execute step 12: ", err)
	}
	// step 13 - Check the 1Khz video/audio playback by test fixture.
	if err := dock28ReconnectDisplayStep13(ctx, extDispID); err != nil {
		s.Fatal("Failed to execute step 13: ", err)
	}
}

func dock28ReconnectDisplayStep3(ctx context.Context, extDispID string) error {
	testing.ContextLog(ctx, "Step 3 - Connect the external monitor to the docking station")
	if err := utils.SwitchFixture(ctx, extDispID, "on", "0"); err != nil {
		return errors.Wrap(err, "failed to connect external display")
	}
	return nil
}

func dock28ReconnectDisplayStep4(ctx context.Context, dockingID string) error {
	testing.ContextLog(ctx, "Step 4 - Connect the docking station to chromebook")
	if err := utils.SwitchFixture(ctx, dockingID, "on", "0"); err != nil {
		return errors.Wrap(err, "failed to connect docking station")
	}
	return nil
}

func dock28ReconnectDisplayStep5To8(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn, kb *input.KeyboardEventWriter) error {
	testing.ContextLog(ctx, "Step 5, 6, 7, 8 - play youtube on external display")
	// play youtube
	if err := utils.PlayYouTube(ctx, cr, tconn); err != nil {
		return errors.Wrap(err, "failed to play youtube")
	}
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

func dock28ReconnectDisplayStep9(ctx context.Context, extDispID string) error {
	testing.ContextLog(ctx, "Step 9 - Disconnect the external monitor from docking station")
	if err := utils.SwitchFixture(ctx, extDispID, "off", "0"); err != nil {
		return errors.Wrap(err, "failed to disconnect external display")
	}
	return nil
}

func dock28ReconnectDisplayStep10(ctx context.Context, tconn *chrome.TestConn) error {
	testing.ContextLog(ctx, "Step 10 - Check window bounds on primary display")
	// get primary info to compare
	primaryInfo, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get primary display info")
	}
	// ensure youtube on primary display
	if err := utils.EnsureYoutubeOnDisplay(ctx, tconn, primaryInfo); err != nil {
		return errors.Wrap(err, "failed to ensure youtube on primary display")
	}
	return nil
}

func dock28ReconnectDisplayStep11(ctx context.Context, extDispID string) error {
	testing.ContextLog(ctx, "Step 11 - Connect the external monitor to docking station")
	if err := utils.SwitchFixture(ctx, extDispID, "on", "0"); err != nil {
		return errors.Wrap(err, "failed to connect external display")
	}
	return nil
}

func dock28ReconnectDisplayStep12(ctx context.Context, tconn *chrome.TestConn) error {
	testing.ContextLog(ctx, "Step 12 - Check window bounds on external display")
	if err := testing.Poll(ctx, func(c context.Context) error {
		// get display info
		infos, err := display.GetInfo(ctx, tconn)
		if err != nil {
			return errors.Wrap(err, "failed to get any display info")
		}
		if err := utils.EnsureYoutubeOnDisplay(ctx, tconn, &infos[1]); err != nil {
			return errors.Wrap(err, "failed to ensure youtube on ext-display1")
		}
		return nil
	}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
		return err
	}
	return nil
}

func dock28ReconnectDisplayStep13(ctx context.Context, extDispID string) error {
	testing.ContextLog(ctx, "Step 13 - Check the 1Khz video/audio playback on ext-display1 by test fixture")
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
