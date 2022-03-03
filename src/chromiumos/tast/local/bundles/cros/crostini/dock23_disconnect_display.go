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

package crostini

import (
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/crostini/utils"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/testing"
	"context"
	"time"
)

// 1. Power the Chrombook On.
// 2. Sign-in account.
func init() {
	testing.AddTest(&testing.Test{
		Func:         Dock23DisconnectDisplay,
		Desc:         "Soft-disconnect a display",
		Contacts:     []string{"allion-sw@allion.com"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "arcBooted", //Boot-up and Sign-In to the device
		Timeout:      10 * time.Minute,
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
		Vars: utils.GetInputVars(),
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

	s.Logf("Step 1 - Power the Chrombook On")

	s.Logf("Step 2 - Sign-in account.")

	// step 3 - connect ext-display to docking station
	if err := Dock23DisconnectDisplay_Step3(ctx, s); err != nil {
		s.Fatal("Failed to execute step 3: ", err)
	}

	// step 4 - connect docking station to chromebook
	if err := Dock23DisconnectDisplay_Step4(ctx, s); err != nil {
		s.Fatal("Failed to execute step 4: ", err)
	}

	// step 5, 6 - play youtube on internal display
	if err := Dock23DisconnectDisplay_Step5To6(ctx, s, cr, tconn); err != nil {
		s.Fatal("Failed to execute step 5, 6: ", err)
	}

	// step 7  - turn off internal display
	if err := Dock23DisconnectDisplay_Step7(ctx, s); err != nil {
		s.Fatal("Failed to execute step 7: ", err)
	}

	// step 8  - check youtube on external display
	if err := Dock23DisconnectDisplay_Step8(ctx, s, tconn); err != nil {
		s.Fatal("Failed to execute step 8: ", err)
	}

	// step 9  - check keyboard.. etc (using event)
	if err := Dock23DisconnectDisplay_Step9(ctx, s); err != nil {
		s.Fatal("Failed to execute step 9: ", err)
	}

	// step 10 - turn on internal display
	if err := Dock23DisconnectDisplay_Step10(ctx, s); err != nil {
		s.Fatal("Failed to execute step 10: ", err)
	}

	// step 11 - check youtube on primary display
	if err := Dock23DisconnectDisplay_Step11(ctx, s, tconn); err != nil {
		s.Fatal("Failed to execute step 11: ", err)
	}

	// step 12 - check playback
	if err := Dock23DisconnectDisplay_Step12(ctx, s, tconn); err != nil {
		s.Fatal("Failed to execute step 12: ", err)
	}

	// reset chromebook
	youtube, err := utils.GetYoutubeWindow(ctx, tconn)
	if err != nil {
		s.Logf("Failed to get youtube window: ", err)
	}

	if err := youtube.CloseWindow(ctx, tconn); err != nil {
		s.Logf("Failed to close youtube: ", err)
	}
}

// 3. Connect the external monitor to the docking station via Type-C cable.
func Dock23DisconnectDisplay_Step3(ctx context.Context, s *testing.State) error {

	s.Logf("Step 3 - Connect the ext-display to docking station")

	if err := utils.ControlFixture(ctx, s, utils.FixtureExtDisp1, utils.ActionPlugin, false); err != nil {
		return errors.Wrap(err, "Failed to connect ext-display to docking station: ")
	}

	return nil
}

// 4. Connect the docking station to chromebook via Type-C cable. (switch Type-C & HDMI fixture)
func Dock23DisconnectDisplay_Step4(ctx context.Context, s *testing.State) error {

	s.Logf("Step 4 - Connect the docking station to chromebook")

	if err := utils.ControlFixture(ctx, s, utils.FixtureStation, utils.ActionPlugin, false); err != nil {
		return errors.Wrapf(err, "Failed to connect docking station to chromebook: ")
	}

	return nil
}

// 5. Click and open the Google Chrome browser from the bottom middle of the screen.
// 6. Input and navigate the video address ""https://www.youtube.com/watch?v=l4bDVq-nP-0&t=65s""
func Dock23DisconnectDisplay_Step5To6(ctx context.Context, s *testing.State, cr *chrome.Chrome, tconn *chrome.TestConn) error {

	s.Logf("Step 5, 6 - Play youtube")

	if err := utils.PlayYouTube(ctx, cr, tconn); err != nil {
		return errors.Wrap(err, "Failed to play youtube: ")
	}

	return nil
}

// 7. internal display power off
func Dock23DisconnectDisplay_Step7(ctx context.Context, s *testing.State) error {

	s.Logf("Step 7 - Internal display off ")

	// 7. internal display power off
	if err := utils.SetDisplayPower(ctx, utils.DisplayPowerInternalOffExternalOn); err != nil {
		return errors.Wrap(err, "Failed to set internal display power off: ")
	}

	return nil
}

// 8. Check window bounds on external display
func Dock23DisconnectDisplay_Step8(ctx context.Context, s *testing.State, tconn *chrome.TestConn) error {

	s.Logf("Step 8 - Check window bounds on external display ")

	// get display info
	infos, err := display.GetInfo(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "Failed to get display info: ")
	}

	// check num of infos
	if len(infos) != 1 {
		return errors.Errorf("Failed to get num of display , got %d, at least 1", len(infos))
	}

	// ensure youtube on ext-display 1
	if err := utils.EnsureYoutubeOnDisplay(ctx, s, tconn, &infos[0]); err != nil {
		return errors.Wrap(err, "Failed to ensure youtube on ext-display1: ")
	}

	return nil
}

// 9. Check Keyboard /Mouse/ Touchpad work without any issue (use tast Event)
func Dock23DisconnectDisplay_Step9(ctx context.Context, s *testing.State) error {

	s.Logf("Step 9 - Check Keyboard /Mouse/ Touchpad work ")

	// check keyboard
	if err := utils.VerifyKeyboard(ctx, s); err != nil {
		return errors.Wrap(err, "Failed to verify keyboard: ")
	}

	// check mouse
	if err := utils.VerifyMouse(ctx, s); err != nil {
		return errors.Wrap(err, "Failed to verify mouse")
	}

	return nil
}

// 10. internal display power on
func Dock23DisconnectDisplay_Step10(ctx context.Context, s *testing.State) error {

	s.Logf("Step 10 - Internal display power on ")

	// 10. internal display power on
	if err := utils.SetDisplayPower(ctx, utils.DisplayPowerAllOn); err != nil {
		return errors.Wrap(err, "Failed to set internal display power on: ")
	}

	return nil
}

// 11. Check window bounds on Primary display
func Dock23DisconnectDisplay_Step11(ctx context.Context, s *testing.State, tconn *chrome.TestConn) error {

	s.Logf("Step 11 - Check window bounds on Primary display ")

	// get primary display info
	primaryInfo, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		return errors.Wrapf(err, "Failed to get the primary display info: ")
	}

	// ensure youtbe on primary display
	if err := utils.EnsureYoutubeOnDisplay(ctx, s, tconn, primaryInfo); err != nil {
		return errors.Wrapf(err, "Failed to  ensure youtube on primary display: ")
	}

	return nil
}

// 12. Check the 1Khz video/audio playback  by test fixture."
func Dock23DisconnectDisplay_Step12(ctx context.Context, s *testing.State, tconn *chrome.TestConn) error {

	s.Logf("Step 12 - Check playback ")

	if err := utils.CheckPlaybackByFixture(ctx, s, utils.InternalDisplay); err != nil {
		return errors.Wrapf(err, "Failed to check playback")
	}

	return nil
}
