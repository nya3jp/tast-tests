// 32 Shut down the computer while display is connected

// "Pre-Condition:
// (Please note: Brand / Model number on test result)
// 1. External displays (Single/Dual)
// 2. Docking station / Hub /Dongle /Adapter
// 3. Connection Type (HDMI/DP/VGA/DVI/USB-C on test result)

// Procedure:
// 1) Boot-up and Sign-In to the device
// 2) Connect ext-display onto (Dock/Hub/Dongle/Adapter)
// 3) Connect (Dock/Hub/Dongle/Adapter) onto Chromebook
// Note: both ""Primary and Extended"" display show up without any issue
// 4) Shut down the Chromebook either use (Hold down power button or go to Quick Settings and select Power off button)

// Verification:
// 4) Make sure Chromebook ""Shut Down"" without any issue and no flickering/crash/hang

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
// 4. Connect the docking station to chromebook via Type-C cable. (switch Type-C &HDMI fixture)
// 5. Run verification step 1 & 2.
// 6. At the bottom right, select the time to display the Quick Settings.
// 7. Select Shut down to power off the chromebook system.
// 8. Run verification step 3 & 4."

// Automation verification
// "1. Check the external monitor display properly by test fixture.
// 2. Check the chromebook display properly by test fixture.
// 3. Check the external monitor become dark by test fixture.
// 4. Check the chromebook display become dark by test fixture."

package crostini

import (
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/crostini/utils"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/testing"
	"context"
	"time"
)

// Test Step:
// 1. Power the Chrombook On.
// 2. Sign-in account.
func init() {
	testing.AddTest(&testing.Test{
		Func:         Dock32ShutdownComputer,
		Desc:         "Suspend the computer while external display connected.",
		Contacts:     []string{"allion-sw@allion.com"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      4 * time.Minute,
		Pre:          chrome.LoggedIn(), //Boot-up and Sign-In to the device
		Vars:         utils.GetInputVars(),
	})
}

func Dock32ShutdownComputer(ctx context.Context, s *testing.State) { // chrome.LoggedIn()

	// set up
	cr := s.PreValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to open connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	s.Logf("Step 1 - Power the Chromebook On")

	s.Logf("Step 2 - Sign-in account")

	// step 3 - connect ext-display to station
	if err := Dock32ShutdownComputer_Step3(ctx, s); err != nil {
		s.Fatal("Failed to execute step 3: ", err)
	}
	// step 4 - connect station to chromebook
	if err := Dock32ShutdownComputer_Step4(ctx, s); err != nil {
		s.Fatal("Failed to execute step 4: ", err)
	}

	// step 5, 6 - check chromebook & ext-display
	if err := Dock32ShutdownComputer_Step5To6(ctx, s, tconn); err != nil {
		s.Fatal("Failed to execute step 5, 6: ", err)
	}

	// step 7, 8 - power off chromebook
	if err := Dock32ShutdownComputer_Step7To8(ctx, s); err != nil {
		s.Fatal("Failed to execute step 7, 8: ", err)
	}

	// cuz chromebook has power off,
	// then SSH would lost
	// skip step 9, 10
	// do it from outside

}

// 3. Connect the external monitor to the docking station via Type-C cable.
func Dock32ShutdownComputer_Step3(ctx context.Context, s *testing.State) error {

	s.Logf("Step 3 - Connect the external monitor to the docking station ")

	if err := utils.ControlFixture(ctx, s, utils.FixtureExtDisp1, utils.ActionPlugin, false); err != nil {
		return errors.Wrap(err, "Failed to connect ext-display to docking station: ")
	}

	return nil
}

// 4. Connect the docking station to chromebook via Type-C cable. (switch Type-C &HDMI fixture)
func Dock32ShutdownComputer_Step4(ctx context.Context, s *testing.State) error {

	s.Logf("Step 4 - Connect the docking station to chromebook ")

	if err := utils.ControlFixture(ctx, s, utils.FixtureStation, utils.ActionPlugin, false); err != nil {
		return errors.Wrapf(err, "Failed to connect docking station to chromebook: ")
	}

	return nil
}

// 5. Check the external monitor display properly by test fixture.
// 6. Check the chromebook display properly by test fixture.
func Dock32ShutdownComputer_Step5To6(ctx context.Context, s *testing.State, tconn *chrome.TestConn) error {

	s.Logf("Step 5 - Check the external monitor display properly by test fixture.")

	s.Logf("Step 6 - Check the chromebook display properly by test fixture.")

	if err := utils.VerifyDisplayProperly(ctx, s, tconn, 2); err != nil {
		return errors.Wrap(err, "Failed to verify display properly: ")
	}

	return nil
}

// 7. At the bottom right, select the time to display the Quick Settings.
// 8. Select Shut down to power off the chromebook system.
func Dock32ShutdownComputer_Step7To8(ctx context.Context, s *testing.State) error {

	s.Logf("Step 7, 8 - Power off chromebook")

	if err := utils.PoweroffChromebook(ctx, s); err != nil {
		return errors.Wrap(err, "Failed to power off chromebook: ")
	}

	return nil
}

// 9. Check the external monitor become dark by test fixture.
// 10. Check the chromebook display become dark by test fixture."
func Dock32ShutdownComputer_Step9To10(ctx context.Context, s *testing.State, tconn *chrome.TestConn) error {

	s.Logf("Step 9 -  Check the external monitor become dark by test fixture")

	s.Logf("Step 10 - Check the chromebook display become dark by test fixture")

	if err := utils.CheckColor(ctx, s, utils.InternalDisplay, "black"); err != nil {
		return errors.Wrap(err, "Failed to check chromebook monitor become dark: ")
	}

	if err := utils.CheckColor(ctx, s, utils.ExternalDisplay1, "black"); err != nil {
		return errors.Wrap(err, "Failed to check external monitor become dark: ")
	}

	return nil
}
