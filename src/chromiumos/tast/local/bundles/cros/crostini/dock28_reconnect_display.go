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

package crostini

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/crostini/utils"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/testing"
)

// Test Step:
// 1. Power the Chrombook On.
// 2. Sign-in account.
func init() {
	testing.AddTest(&testing.Test{
		Func:         Dock28ReconnectDisplay,
		Desc:         "Reconnect a previously used display",
		Contacts:     []string{"allion-sw@allion.com"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      10 * time.Minute,
		Pre:          chrome.LoggedIn(), //Boot-up and Sign-In to the device
		Vars:         utils.GetInputVars(),
	})
}

func Dock28ReconnectDisplay(ctx context.Context, s *testing.State) { // chrome.LoggedIn()

	// set up
	cr := s.PreValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to open connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	s.Log("Step 1 - Power the Chrombook On")

	s.Log("Step 2 - Sign-in account")

	// step 3 - connect ext-display to station
	if err := Dock28ReconnectDisplay_Step3(ctx, s); err != nil {
		s.Fatal("Failed to execute step 3: ", err)
	}

	// step 4 - connect station to chromebook
	if err := Dock28ReconnectDisplay_Step4(ctx, s); err != nil {
		s.Fatal("Failed to execute step 4: ", err)
	}

	// step 5, 6, 7, 8 - play youtube on exteranl display
	if err := Dock28ReconnectDisplay_Step5To8(ctx, s, cr, tconn); err != nil {
		s.Fatal("Failed to execute step 5, 6, 7, 8: ", err)
	}

	// step 9 - disconnect station from chromebook
	if err := Dock28ReconnectDisplay_Step9(ctx, s); err != nil {
		s.Fatal("Failed to execute step 9: ", err)
	}

	// step 10 - check youtube on primary display
	if err := Dock28ReconnectDisplay_Step10(ctx, s, tconn); err != nil {
		s.Fatal("Failed to execute step 10: ", err)
	}

	// step 11 - connect station
	if err := Dock28ReconnectDisplay_Step11(ctx, s); err != nil {
		s.Fatal("Failed to execute step 11: ", err)
	}

	// step 12 - check youtube on external display
	if err := Dock28ReconnectDisplay_Step12(ctx, s, tconn); err != nil {
		s.Fatal("Failed to execute step 12: ", err)
	}

	// step 13 - check playback
	if err := Dock28ReconnectDisplay_Step13(ctx, s, tconn); err != nil {
		s.Fatal("Failed to execute step 13: ", err)
	}
}

// 3. Connect the external monitor to the docking station via Type-C cable. (Manual)
func Dock28ReconnectDisplay_Step3(ctx context.Context, s *testing.State) error {

	s.Log("Step 3 - Connect the external monitor to the docking station")

	if err := utils.ControlFixture(ctx, s, utils.FixtureExtDisp1, utils.ActionPlugin, false); err != nil {
		return errors.Wrap(err, "failed to connect ext-display to docking station")
	}

	return nil
}

// 4. Connect the docking station to chromebook via Type-C cable. (switch Type-C & HDMI fixturel)
func Dock28ReconnectDisplay_Step4(ctx context.Context, s *testing.State) error {

	s.Log("Step 4 - Connect the docking station to chromebook")

	if err := utils.ControlFixture(ctx, s, utils.FixtureStation, utils.ActionPlugin, false); err != nil {
		return errors.Wrap(err, "failed to connect docking to chromebook")
	}

	return nil
}

// 5. Click and open the Google Chrome browser from the bottom middle of the screen.(open on external display)
// 6. Input and navigate the video address ""https://www.youtube.com/watch?v=l4bDVq-nP-0&t=65s""
// 7. Drag the Google Chrome browser from chromebook display to external monitor.
// 8. Check window bounds on external display.
func Dock28ReconnectDisplay_Step5To8(ctx context.Context, s *testing.State, cr *chrome.Chrome, tconn *chrome.TestConn) error {

	s.Log("Step 5, 6, 7, 8 - play youtube on external display")

	// play youtube
	if err := utils.PlayYouTube(ctx, cr, tconn); err != nil {
		return errors.Wrap(err, "failed to play youtube")
	}

	if err := testing.Poll(ctx, func(c context.Context) error {

		// get display info
		infos, err := display.GetInfo(ctx, tconn)
		if err != nil {
			return errors.Wrap(err, "failed to get display info")
		}

		// get youtube window
		youtube, err := utils.GetYoutubeWindow(ctx, tconn)
		if err != nil {
			return errors.Wrap(err, "failed to get youtube window")
		}

		// move youtube form internal to external
		if err := utils.MoveWindowToDisplay(ctx, s, tconn, youtube, &infos[1]); err != nil {
			return errors.Wrap(err, "failed to move youtube to ext-display 1 ")
		}

		return nil
	}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
		return err
	}

	return nil
}

// 9. Disconnect the external monitor from docking station. (switch Type-C & HDMI fixture)
func Dock28ReconnectDisplay_Step9(ctx context.Context, s *testing.State) error {

	s.Log("Step 9 - Disconnect the external monitor from docking station")

	if err := utils.ControlFixture(ctx, s, utils.FixtureExtDisp1, utils.ActionUnplug, false); err != nil {
		return errors.Wrap(err, "failed to disconnect ext-display from docking station")
	}

	return nil
}

// 10. Check window bounds on primary display.
func Dock28ReconnectDisplay_Step10(ctx context.Context, s *testing.State, tconn *chrome.TestConn) error {

	s.Log("Step 10 - Check window bounds on primary display")

	// get primary info to compare
	primaryInfo, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get primary display info")
	}

	// ensure youtube on primary display
	if err := utils.EnsureYoutubeOnDisplay(ctx, s, tconn, primaryInfo); err != nil {
		return errors.Wrap(err, "failed to ensure youtube on primary display")
	}

	return nil
}

// 11. Connect the external monitor to docking station. (switch Type-C & HDMI fixture)
func Dock28ReconnectDisplay_Step11(ctx context.Context, s *testing.State) error {

	s.Log("Step 11 - Connect the external monitor to docking station")

	if err := utils.ControlFixture(ctx, s, utils.FixtureExtDisp1, utils.ActionPlugin, false); err != nil {
		return errors.Wrap(err, "failed to connect ext-display to docking station")
	}

	return nil
}

// 12. Check window bounds on external display.
func Dock28ReconnectDisplay_Step12(ctx context.Context, s *testing.State, tconn *chrome.TestConn) error {

	s.Log("Step 12 - Check window bounds on external display")

	if err := testing.Poll(ctx, func(c context.Context) error {
		// get display info
		infos, err := display.GetInfo(ctx, tconn)
		if err != nil {
			return errors.Wrap(err, "failed to get any display info")
		}

		if err := utils.EnsureYoutubeOnDisplay(ctx, s, tconn, &infos[1]); err != nil {
			return errors.Wrap(err, "failed to ensure youtube on ext-display1")
		}

		return nil
	}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
		return err
	}

	return nil
}

// 13. Check the 1Khz video/audio playback by test fixture."
func Dock28ReconnectDisplay_Step13(ctx context.Context, s *testing.State, tconn *chrome.TestConn) error {

	s.Log("Step 13 - Check the 1Khz video/audio playback on ext-display1 by test fixture")

	if err := utils.CheckPlaybackByFixture(ctx, s, utils.ExternalDisplay1); err != nil {
		return errors.Wrap(err, "failed to check playback by fixture")
	}

	return nil
}
