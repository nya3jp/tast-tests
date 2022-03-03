// 12 Sunset/Sunrise Light mode

// "Pre-Condition:
// (Please note: Brand / Model number on test result)
// 1. External displays (Single /Dual)
// 2. Docking station /Hub /Dongle
// 3. Connection Type (HDMI/DP/VGA/DVI/USB-C on test result)

// Procedure:
// 1) Boot-up and Sign-In to the device
// 2) Connect ext-display to (Docking station or Hub)
// 3) Connect (Docking station or Hub) to Chromebook
// 4) Go to ""Quick Settings Menu and Setting /Device /Displays
// Note: By default (Night Light - Off) both ""Primary and Ext-Display"" screen should NOT dim)
// 5) Now turn (Night Light - On)
// Note: Both (""Primary and Ext-Display"" screen should be dimmed)

// 6)  Adjust the (Color temperature - Cool/Warm)
// Note: Both ""Primary and Ext-Display"" screen should see color changed

// Verification:
// See Note: 4), 5), 6)
///////////////////////////////////////////////////////////////////////////
// automation step
// "Test Step:
// 1. Power the Chromebook On.
// 2. Sign-in account.
// 3. Connect external monitor to the docking station or hub (Manual)
// 4. Connect docking station or hub to the chromebook (turn on usb switch power)
// 5. Run verification 1.
// 6. Open files app on internal monitor.(相機必須指定判斷範圍，files app上的白色區塊)
// 7. Run verification 2.
// 8. Turn Night Light - On.
// 10. Set internal & external color temperature to cooler
// 11. Run verification 3.
// 12. Set internal & external color temperature to warmer
// 13. Run verification 4."

// verification
// 1. Check external monitor properly.
// 2. Check internal and external (Night Light - OFF),use camera check screen color ,should be white
// 3. Check internal and external (Night Light - ON),use camera check screen color,should be white
// 4. use camera check screen color,should be yellow"

package crostini

import (
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/crostini/utils"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/power/setup"
	"chromiumos/tast/testing"
	"context"
	"time"
)

var defaultOSSettingsPollOptions = &testing.PollOptions{
	Timeout:  10 * time.Second,
	Interval: 1 * time.Second,
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Dock12LightMode,
		Desc:         "Sunset/Sunrise Light mode",
		Contacts:     []string{"allion-sw@allion.com"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      4 * time.Minute,
		Pre:          chrome.LoggedIn(), //Boot-up and Sign-In to the device
		Vars:         utils.GetInputVars(),
	})
}

func Dock12LightMode(ctx context.Context, s *testing.State) {
	// set up
	cr := s.PreValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to open connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	s.Log("Step 1 - Power the Chromebook On ")

	s.Log("Step 2 - Sign-in account ")

	// step 3 - connect ext-display to docking station
	if err := Dock12LightMode_Step3(ctx, s); err != nil {
		s.Fatal("Failed to execute step 3: ", err)
	}

	// step 4 - connect docking station to chromebook
	if err := Dock12LightMode_Step4(ctx, s); err != nil {
		s.Fatal("Failed to execute step 4: ", err)
	}

	// step 5 - check external display
	if err := Dock12LightMode_Step5(ctx, s, tconn); err != nil {
		s.Fatal("Failed to execute step 5: ", err)
	}

	// step 6 - open files app on internal display
	if err := Dock12LightMode_Step6(ctx, s, tconn); err != nil {
		s.Fatal("Failed to execute step 6: ", err)
	}

	// step 7 - turn on night light mode
	if err := Dock12LightMode_Step7(ctx, s, tconn); err != nil {
		s.Fatal("Failed to execute step 7: ", err)
	}

	// step 8 - set color temperature cooler
	if err := Dock12LightMode_Step8(ctx, s, tconn); err != nil {
		s.Fatal("Failed to execute step 8: ", err)
	}

	// step 9 - use camera to get first-time color
	color, err := Dock12LightMode_Step9(ctx, s)
	if err != nil {
		s.Fatal("Failed to execute step 9: ", err)
	}

	// step 10 - set color temperature warmer
	if err := Dock12LightMode_Step10(ctx, s); err != nil {
		s.Fatal("Failed to execute step 10: ", err)
	}

	// step 11 - use camera to get screen color, then compare with it
	if err := Dock12LightMode_Step11(ctx, s, color); err != nil {
		s.Fatal("Failed to execute step 11: ", err)
	}

}

// 3. Connect external monitor to the docking station or hub (Manual)
func Dock12LightMode_Step3(ctx context.Context, s *testing.State) error {

	s.Logf("Step 3 - Connect external monitor to the docking station")

	if err := utils.ControlFixture(ctx, s, utils.FixtureExtDisp1, utils.ActionPlugin, false); err != nil {
		return errors.Wrap(err, "Failed to connect ext-display to docking station: ")
	}

	return nil
}

// 4. Connect docking station or hub to the chromebook (turn on usb switch power)
func Dock12LightMode_Step4(ctx context.Context, s *testing.State) error {

	s.Logf("Step 4 - Connect docking station to the chromebook ")

	// connect docking station
	if err := utils.ControlFixture(ctx, s, utils.FixtureStation, utils.ActionPlugin, false); err != nil {
		return errors.Wrap(err, "Failed to plug in docking station: ")
	}

	return nil
}

// 5. Check external monitor properly.
func Dock12LightMode_Step5(ctx context.Context, s *testing.State, tconn *chrome.TestConn) error {

	s.Logf("Step 5 - Check external display info")

	// get display info
	infos, err := display.GetInfo(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "Failed to get display info")
	}

	if len(infos) < 2 {
		return errors.Errorf("Failed to get correct num of display, got %d, at least 2", len(infos))
	}

	return nil
}

// 6. Open files app on internal monitor.(相機必須指定判斷範圍，files app上的白色區塊)
func Dock12LightMode_Step6(ctx context.Context, s *testing.State, tconn *chrome.TestConn) error {

	s.Logf("Step 6 - Open settings on internal display")

	// open setting to device
	if _, err := ossettings.LaunchAtPage(
		ctx,
		tconn,
		nodewith.Name("Device").Role(role.Link),
	); err != nil {
		return errors.Wrap(err, "opening settings page failed")
	}

	// click maximize button to maximize screen
	if err := ui.StableFindAndClick(
		ctx,
		tconn,
		ui.FindParams{
			Name: "Maximize",
			Role: ui.RoleTypeButton,
		},
		defaultOSSettingsPollOptions,
	); err != nil {
		return errors.Wrap(err, "failed to click maximum button")
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
		return errors.Wrap(err, "Failed to create keyboard: ")
	}

	// key in night light
	if err := kb.Type(ctx, "Night light"); err != nil {
		return errors.Wrap(err, "failed to type night light: ")
	}

	time.Sleep(1 * time.Second)

	// to find night light
	if err := kb.TypeKey(ctx, input.KEY_ENTER); err != nil {
		return errors.Wrap(err, "failed to type enter: ")
	}

	time.Sleep(5 * time.Second)

	return nil

}

// 7. Turn Night Light - On.
func Dock12LightMode_Step7(ctx context.Context, s *testing.State, tconn *chrome.TestConn) error {

	s.Logf("Step 7 - Turn night light on")

	if _, err := setup.SetNightLightEnabled(ctx, tconn, true); err != nil {
		return errors.Wrapf(err, "Failed to set night light enable to true: ")
	}

	return nil
}

// 8. Set internal & external color temperature to cooler
func Dock12LightMode_Step8(ctx context.Context, s *testing.State, tconn *chrome.TestConn) error {

	s.Logf("Step 8 - Set internal & external color temperature to cooler")

	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "Failed to create keyboard: ")
	}

	time.Sleep(1 * time.Second)

	// move to seekbar
	if err := kb.TypeKey(ctx, input.KEY_TAB); err != nil {
		return errors.Wrap(err, "failed to type enter: ")
	}

	for i := 0; i < 100; i++ {

		// move slider to left
		if err := kb.TypeKey(ctx, input.KEY_DOWN); err != nil {
			return errors.Wrap(err, "failed to type enter: ")
		}

	}

	return nil
}

// 9. Check internal and external (Night Light - ON),use camera to get screen color
func Dock12LightMode_Step9(ctx context.Context, s *testing.State) (string, error) {

	s.Logf("Step 9 - Use camera to get current screen color")

	time.Sleep(5 * time.Second)

	color, err := utils.GetColor(ctx, s, utils.InternalDisplay)
	if err != nil {
		return "", errors.Wrap(err, "Failed to execute GetPiColor: ")
	}

	return color, nil
}

// 10. Set internal & external color temperature to warmer
func Dock12LightMode_Step10(ctx context.Context, s *testing.State) error {

	s.Logf("Step 10 -  Set internal & external color temperature to warmer")

	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "Failed to create keyboard: ")
	}

	for i := 0; i < 100; i++ {

		// move slider to right
		if err := kb.TypeKey(ctx, input.KEY_UP); err != nil {
			return errors.Wrap(err, "failed to type enter: ")
		}

	}

	return nil
}

// 11. use camera to check screen, then compare with it
func Dock12LightMode_Step11(ctx context.Context, s *testing.State, firstColor string) error {

	s.Logf("Step 11 - use camera to get current screen color, then compare with first time color - %s", firstColor)

	secondColor, err := utils.GetColor(ctx, s, utils.InternalDisplay)

	if err != nil {
		return errors.Wrap(err, "Failed to execute GetPiColor: ")
	}

	s.Logf("Frist-time color is %s", firstColor)

	s.Logf("Second-time color is %s", secondColor)

	if firstColor == secondColor {
		return errors.Errorf("First-time color should not be as same as second-time color")
	}

	return nil
}
