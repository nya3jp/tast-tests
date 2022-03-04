// #16 Unified Desktop
// "Precondition
// (Please note: Brand / Model number on test result)
// 1. External displays (Single /Dual)
// 2. Docking station /Hub /Dongle
// 3. USB Peripherals (Mouse, Keyboard)
// 4. Connection Type (HDMI/DP/VGA/DVI/USB-C on test result)
// 5. Unified Desktop is enabled from chrome://flags.
// 6. Verify the ability to have a single app span multiple displays

// Procedure
// 1) Boot-up and Sign-In to the device
// 2) Open Chrome browser in the maximized window
// 3) Connect to any external monitor.
// Verify the single browser page spans over internal and external display.
// No flickering, no crash.

// 4) Turn off and on ""Allow windows to span displays"" at chrome://settings/display.
// Verified Unified Desktop option can be turned off and on.

// close unified desktop mode
// 5) Arrange monitor order at chrome://settings/display by dragging the display box to different positions.
// Verify monitor order is correct as arranged.
// No flickering, no cras.

// reopen unified desktop mode
// 6) Change Resolution at chrome://settings/display.
// No flickering, no crash.

// 7) Enter dock mode by closing/open lid.
// No flickering, no crash.

// 8) Suspend and resume the device by running 'powerd_dbus_suspend"".
//  No flickering, no crash.

// 9) Press Ctrl-F4 to switch to mirror mode, and back to Unified view
// Verify mirror mode works.
// No flickering, no crash.

// 10) Add one more external display (2+ monitors) and repeat 3-8.

// 11) Repeat 3-8 in Tablet mode.

// 12) Repeat 3-8 against any ARC++ app which allows full screen.

// Note: Related bugs
// https://bugs.chromium.org/p/chromium/issues/detail?id=511477
// https://bugs.chromium.org/p/chromium/issues/detail?id=520128
// "

package crostini

import (
	"context"
	"encoding/json"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/crostini/utils"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Dock16UnifiedDesktop,
		Desc:         "Unified Desktop",
		Contacts:     []string{"allion-sw@allion.com"},
		Timeout:      4 * time.Minute,
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      "chromePolicyLoggedIn",
		Vars:         utils.GetInputVars(),
	})
}

func Dock16UnifiedDesktop(ctx context.Context, s *testing.State) {

	// cr := s.FixtValue().(*chrome.Chrome)
	// fdms := s.FixtValue().(*fixtures.FixtData).FakeDMS

	// // Connect to Test API to use it with the UI library.
	// tconn, err := cr.TestAPIConn(ctx)
	// if err != nil {
	// 	s.Fatal("Failed to create Test API connection: ", err)
	// }
	// defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// s.Logf("Step 1 - Boot-up and Sign-In to the device")

	// // notice: script didn't describe
	// if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{&policy.UnifiedDesktopEnabledByDefault{
	// 	Stat: policy.StatusSet,
	// 	Val:  true}}); err != nil {
	// 	s.Fatal("Failed to update policies: ", err)
	// }

	// // step 2 - open browser in maximized window
	// if err := Dock16UnifiedDesktop_Step2(ctx, s, cr, tconn); err != nil {
	// 	s.Fatal("Failed to execute step4: ", err)
	// }

	// // step 3 - connect ext-display to station, connect station to chromebook
	// if err := Dock16UnifiedDesktop_Step3(ctx, s, tconn); err != nil {
	// 	s.Fatal("Failed to execute step3: ", err)
	// }

	// // step 4 - turn off / on unified desktop option
	// if err := Dock16UnifiedDesktop_Step4(ctx, s, cr, tconn, fdms); err != nil {
	// 	s.Fatal("Failed to execute step4: ", err)
	// }

	// // step 5 - arrange moniter order
	// if err := Dock16UnifiedDesktop_Step5(ctx, s, tconn); err != nil {
	// 	s.Fatal("Failed to execute step5: ", err)
	// }

	// // step 6 - change resolution
	// if err := Dock16UnifiedDesktop_Step6(ctx, s, tconn); err != nil {
	// 	s.Fatal("Failed to execute step6: ", err)
	// }

	// // step 7 - close / open lid
	// if err := Dock16UnifiedDesktop_Step7(ctx, s); err != nil {
	// 	s.Fatal("Failed to execute step7: ", err)
	// }

	// // step 8 - suspend & wake up chromebook
	// tconn, err = Dock16UnifiedDesktop_Step8(ctx, s, cr)
	// if err != nil {
	// 	s.Fatal("Failed to execute step8:", err)
	// }

	// // step 9 - enter mirror mode, verify mirror works
	// if err := Dock16UnifiedDesktop_Step9(ctx, s, tconn); err != nil {
	// 	s.Fatal("Failed to execute step9: ", err)
	// }

	// // step 10 - add one more monitor, then repeat 3-8
	// if err := Dock16UnifiedDesktop_Step10(ctx, s, cr, tconn, fdms); err != nil {
	// 	s.Fatal("Failed to execute step11:", err)
	// }

	// // step 11 - into tablet mode, then repeat 3-8
	// if err := Dock16UnifiedDesktop_Step11(ctx, s, cr, tconn, fdms); err != nil {
	// 	s.Fatal("Failed to execute step12:", err)
	// }

}

// 2) Open Chrome browser in the maximized window
func Dock16UnifiedDesktop_Step2(ctx context.Context, s *testing.State, cr *chrome.Chrome, tconn *chrome.TestConn) error {

	s.Log("Step 2 - Open Chrome browser in the maximized window")

	// open chrome to url
	_, err := cr.NewConn(ctx, "https://www.google.com/")
	if err != nil {
		s.Fatal(err)
	}
	// defer conn.Close()

	// find browser
	browser, err := ash.FindWindow(ctx, tconn, func(window *ash.Window) bool {
		return window.WindowType == ash.WindowTypeBrowser
	})

	// maximized
	if err := ash.SetWindowStateAndWait(ctx, tconn, browser.ID, ash.WindowStateFullscreen); err != nil {
		s.Fatal("Failed to set window to full screen: ", err)
	}

	// list display info
	infos, err := display.GetInfo(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get display info")
	}

	// check num of display
	if len(infos) < 2 {
		return errors.Errorf("failed to get enough display, got %d, want 2", len(infos))
	}

	// verify unified desktop is working or not
	for _, info := range infos {
		if info.ID == browser.DisplayID {
			return errors.New("Under unified desktop mode and window is maximized, shall unable to get windows info: ")
		}
	}

	return nil
}

// 3) Connect to any external monitor.
// Verify the single browser page spans over internal and external display.
// No flickering, no crash.
// connect station to chromebook
func Dock16UnifiedDesktop_Step3(ctx context.Context, s *testing.State, tconn *chrome.TestConn) error {

	s.Log("Step 3.1 - Connect ext-display to station")

	if err := utils.ControlFixture(ctx, s, utils.FixtureExtDisp1, utils.ActionPlugin, false); err != nil {
		return errors.Wrap(err, "failed to plug in ext-display to docking station")
	}

	s.Log("Step 3.2 - Connect station to chromebook")

	// plug in station
	if err := utils.ControlFixture(ctx, s, utils.FixtureStation, utils.ActionPlugin, false); err != nil {
		return errors.Wrap(err, "failed to plug in station")
	}

	return nil
}

// 4) Turn off and on ""Allow windows to span displays"" at chrome://settings/display.
// Verified Unified Desktop option can be turned off and on.
func Dock16UnifiedDesktop_Step4(ctx context.Context, s *testing.State, cr *chrome.Chrome, tconn *chrome.TestConn, fdms *fakedms.FakeDMS) error {

	s.Log("Step 4 - Turn off and on unified desktop option ")

	for _, param := range []struct {
		name  string
		value *policy.UnifiedDesktopEnabledByDefault
	}{
		{
			name: "disenabled",
			value: &policy.UnifiedDesktopEnabledByDefault{
				Stat: policy.StatusSet,
				Val:  false},
		},
		{
			name: "enabled",
			value: &policy.UnifiedDesktopEnabledByDefault{
				Stat: policy.StatusSet,
				Val:  true},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {

			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{param.value}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

		})
	}

	return nil
}

// 5) Arrange monitor order at chrome://settings/display by dragging the display box to different positions.
// Verify monitor order is correct as arranged.
// No flickering, no cras.
func Dock16UnifiedDesktop_Step5(ctx context.Context, s *testing.State, tconn *chrome.TestConn) error {

	s.Log("Step 5 - Arrange monitor order")

	infos, err := display.GetInfo(ctx, tconn)
	if err != nil {
		return err
	}

	var internalDisplayInfo, externalDisplayInfo display.Info
	for _, info := range infos {
		if info.IsInternal {
			internalDisplayInfo = info
		} else if externalDisplayInfo.ID == "" {
			// Get the first external display info.
			externalDisplayInfo = info
		}
	}

	// Relayout external display and make sure the windows will not move their positions or show black background.
	for _, relayout := range []struct {
		name   string
		offset coords.Point
	}{
		{"Relayout external display on top of internal display", coords.NewPoint(0, -externalDisplayInfo.Bounds.Height)},
		{"Relayout external display on bottom of internal display", coords.NewPoint(0, internalDisplayInfo.Bounds.Height)},
		{"Relayout external display to the left side of internal display", coords.NewPoint(-externalDisplayInfo.Bounds.Width, 0)},
		{"Relayout external display to the right side of internal display", coords.NewPoint(internalDisplayInfo.Bounds.Width, 0)},
	} {
		utils.RunOrFatal(ctx, s, relayout.name, func(ctx context.Context, s *testing.State) error {
			p := display.DisplayProperties{BoundsOriginX: &relayout.offset.X, BoundsOriginY: &relayout.offset.Y}
			if err := display.SetDisplayProperties(ctx, tconn, externalDisplayInfo.ID, p); err != nil {
				return err
			}

			return nil
		})
	}

	return nil
}

// 6) Change Resolution at chrome://settings/display.
// No flickering, no crash.
func Dock16UnifiedDesktop_Step6(ctx context.Context, s *testing.State, tconn *chrome.TestConn) error {

	s.Log("Step 6 - Change resolution ")

	// rule
	// build in - 1536x864
	// ext-disp1 - 1920x1080
	// ext-disp2 - 2560x1440

	// when displays are 2
	// 3072x864
	// 3840x1080

	// when displays are 3
	// 7680x1440
	// 5760x1080
	// 4608x864

	// open setting to device
	if _, err := ossettings.LaunchAtPage(
		ctx,
		tconn,
		nodewith.Name("Device").Role(role.Link),
	); err != nil {
		return errors.Wrap(err, "opening settings page failed")
	}

	// click search settings
	if err := ui.StableFindAndClick(
		ctx,
		tconn,
		ui.FindParams{
			Name: "Search settings",
			Role: ui.RoleTypeSearchBox,
		},
		defaultOSSettingsPollOptions,
	); err != nil {
		return errors.Wrap(err, "opening display menu failed")
	}

	// declare keyboard
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create keyboard")
	}

	// type in resolution
	if err := kb.Type(ctx, "Resolution"); err != nil {
		return errors.Wrap(err, "failed to type in resolution")
	}

	time.Sleep(1 * time.Second)

	// search resolution
	if err := kb.TypeKey(ctx, input.KEY_ENTER); err != nil {
		return errors.Wrap(err, "failed to type enter")
	}

	time.Sleep(1 * time.Second)

	// click resolution
	if err := kb.TypeKey(ctx, input.KEY_ENTER); err != nil {
		return errors.Wrap(err, "failed to type enter")
	}

	time.Sleep(1 * time.Second)

	// choose upper element
	if err := kb.TypeKey(ctx, input.KEY_UP); err != nil {
		return errors.Wrap(err, "failed to type enter")
	}

	time.Sleep(1 * time.Second)

	// click to select resolution
	if err := kb.TypeKey(ctx, input.KEY_ENTER); err != nil {
		return errors.Wrap(err, "failed to type enter")
	}

	time.Sleep(1 * time.Second)

	// click to confirm change
	if err := kb.TypeKey(ctx, input.KEY_ENTER); err != nil {
		return errors.Wrap(err, "failed to type enter")
	}

	time.Sleep(1 * time.Second)

	return nil

}

// 7) Enter dock mode by closing/open lid.
// No flickering, no crash.
func Dock16UnifiedDesktop_Step7(ctx context.Context, s *testing.State) error {

	s.Log("Step 7 - Enter dock mode by closing/open lid")

	for _, param := range []struct {
		state utils.DisplayPowerState
	}{
		{utils.DisplayPowerInternalOffExternalOn},
		{utils.DisplayPowerAllOn},
	} {
		if err := utils.SetDisplayPower(ctx, param.state); err != nil {
			return err
		}
	}

	return nil
}

// 8) Suspend and resume the device by running 'powerd_dbus_suspend"".
//  No flickering, no crash.
func Dock16UnifiedDesktop_Step8(ctx context.Context, s *testing.State, cr *chrome.Chrome) (*chrome.TestConn, error) {

	s.Log("Step 8 - Suspend and resume the device by running 'powerd_dbus_suspend'")

	tconn, err := utils.SuspendChromebook(ctx, s, cr)

	if err != nil {
		return nil, err
	}

	return tconn, nil

}

// 9) Press Ctrl-F4 to switch to mirror mode, and back to Unified view
// Verify mirror mode works.
// No flickering, no crash.
func Dock16UnifiedDesktop_Step9(ctx context.Context, s *testing.State, tconn *chrome.TestConn) error {

	s.Log("Step 9 - Press Ctrl-F4 to switch to mirror mode, verify mirror mode works, and back to Unified view")

	// declare keyboard object
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to find keyboard")
	}
	defer kb.Close()

	time.Sleep(5 * time.Second)

	// send "Ctrl+F4" to enter mirror mode
	if err := kb.Accel(ctx, "Ctrl+F4"); err != nil {
		return errors.Wrap(err, "failed to enter mirror mode")
	}
	// defer kb.Accel(ctx, "Ctrl+F4") // back to normal mode

	time.Sleep(5 * time.Second)

	primaryInfo, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		return err
	}

	obj, _ := json.Marshal(primaryInfo)
	s.Logf("Primary info is %s", string(obj))

	if primaryInfo.ID != primaryInfo.MirroringSourceID {
		return errors.New("failed to enter mirror mode: ")
	}

	return nil
}

// 10) Add one more external display (2+ monitors) and repeat 3-8.
func Dock16UnifiedDesktop_Step10(ctx context.Context, s *testing.State, cr *chrome.Chrome, tconn *chrome.TestConn, fdms *fakedms.FakeDMS) error {

	s.Log("Step 10 - Add one more external display (2+ monitors) and repeat 3-8")

	// connect 2nd monitor
	if err := utils.ControlFixture(ctx, s, utils.FixtureExtDisp2, utils.ActionPlugin, false); err != nil {
		return errors.Wrap(err, "failed to plug ext-display 2 into docking station")
	}

	// repeat 3-8
	// step 3 - connect ext-display to station, connect station to chromebook
	if err := Dock16UnifiedDesktop_Step3(ctx, s, tconn); err != nil {
		return errors.Wrap(err, "failed to execute step3")
	}

	// step 4 - turn off / on unified desktop option
	if err := Dock16UnifiedDesktop_Step4(ctx, s, cr, tconn, fdms); err != nil {
		return errors.Wrap(err, "failed to execute step4")
	}

	// step 5 - arrange moniter order
	if err := Dock16UnifiedDesktop_Step5(ctx, s, tconn); err != nil {
		return errors.Wrap(err, "failed to execute step5")
	}

	// step 6 - change resolution
	if err := Dock16UnifiedDesktop_Step6(ctx, s, tconn); err != nil {
		return errors.Wrap(err, "failed to execute step6")
	}

	// step 7 - close / open lid
	if err := Dock16UnifiedDesktop_Step7(ctx, s); err != nil {
		return errors.Wrap(err, "failed to execute step7")
	}

	// step 8 - suspend & wake up chromebook
	tconn, err := Dock16UnifiedDesktop_Step8(ctx, s, cr)
	if err != nil {
		return errors.Wrap(err, "failed to execute step8")
	}

	return nil
}

// 11) Repeat 3-8 in Tablet mode.
func Dock16UnifiedDesktop_Step11(ctx context.Context, s *testing.State, cr *chrome.Chrome, tconn *chrome.TestConn, fdms *fakedms.FakeDMS) error {

	s.Log("Step 11 - Repeat 3-8 in Tablet mode")
	// into tablet mode
	// ensure tablet mode is enabled
	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, true)
	if err != nil {
		return errors.Wrap(err, "failed to ensure in tablet mode ")
	}
	defer cleanup(ctx)

	// repeat 3-8
	// step 3 - connect ext-display to station, connect station to chromebook
	if err := Dock16UnifiedDesktop_Step3(ctx, s, tconn); err != nil {
		return errors.Wrap(err, "failed to execute step3")
	}

	// step 4 - turn off / on unified desktop option
	if err := Dock16UnifiedDesktop_Step4(ctx, s, cr, tconn, fdms); err != nil {
		return errors.Wrap(err, "failed to execute step4")
	}

	// step 5 - arrange moniter order
	if err := Dock16UnifiedDesktop_Step5(ctx, s, tconn); err != nil {
		return errors.Wrap(err, "failed to execute step5")
	}

	// step 6 - change resolution
	if err := Dock16UnifiedDesktop_Step6(ctx, s, tconn); err != nil {
		return errors.Wrap(err, "failed to execute step6")
	}

	// step 7 - close / open lid
	if err := Dock16UnifiedDesktop_Step7(ctx, s); err != nil {
		return errors.Wrap(err, "failed to execute step7")
	}

	// step 8 - suspend & wake up chromebook
	tconn, err = Dock16UnifiedDesktop_Step8(ctx, s, cr)
	if err != nil {
		return errors.Wrap(err, "failed to execute step8")
	}

	return nil
}

// 12) Repeat 3-8 against any ARC++ app which allows full screen.
func Dock16UnifiedDesktop_Step13(ctx context.Context, s *testing.State, cr *chrome.Chrome, tconn *chrome.TestConn) error {

	// has trouble with doing "chromePolicyLoggedIn" & "arc.Booted()" at same time
	return nil
}
