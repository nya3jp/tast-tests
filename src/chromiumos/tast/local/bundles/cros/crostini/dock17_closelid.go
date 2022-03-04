// #17 Have a dual monitor setup and then close DUT lid
// Test Step:
// 1. Power the Chromebook On.
// 2. Sign-in account.
// 3. Connect external monitor to the chromebook. (switch Type-C & HDMI fixture)
// 4. Check external monitor display properly and remember the resolution
// 5. Open any app on internal monitor.
// 6. Close internal monitor power.
// 7. Check window bounds on external monitor display
// 8. Compare external monitor old resolution and resolution for now

package crostini

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/crostini/utils"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Dock17Closelid,
		Desc:         "Verify that display resolution is still okay after lid close & windows are all still  displayed",
		Contacts:     []string{"allion-sw@allion.com"},
		SoftwareDeps: []string{"arc", "chrome"},
		Timeout:      4 * time.Minute,
		Pre:          arc.Booted(),
		Vars:         utils.GetInputVars(),
	})
}

func Dock17Closelid(ctx context.Context, s *testing.State) {

	cr := s.PreValue().(arc.PreData).Chrome
	a := s.PreValue().(arc.PreData).ARC
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	s.Log("Step 1 - Power the Chromebook On ")

	s.Log("Step 2 - Sign-in account ")

	// step3 - connect ext-display to station
	if err := Dock17Closelid_Step3(ctx, s); err != nil {
		s.Fatal("Failed to execute step3: ", err)
	}

	// step 4. connect station to chromebook
	if err := Dock17Closelid_Step4(ctx, s); err != nil {
		s.Fatal("Failed to execute step4: ", err)
	}

	// step 5. Check external monitor display properly and remember the resolution
	original_ext, err := Dock17Closelid_Step5(ctx, s, tconn)
	if err != nil {
		s.Fatal("Failed to execute step5: ", err)
	}

	// step 6. Open any app on internal monitor.
	if err := Dock17Closelid_Step6(ctx, s, a, tconn); err != nil {
		s.Fatal("Failed to execute step6: ", err)
	}

	// step 7. Close internal monitor power.
	if err := Dock17Closelid_Step7(ctx, s); err != nil {
		s.Fatal("Failed to execute step7: ", err)
	}

	// step 8. Check window bounds on external monitor display
	if err := Dock17Closelid_Step8(ctx, s, tconn); err != nil {
		s.Fatal("Failed to execute step8: ", err)
	}

	// step 9. Compare external monitor old resolution and resolution for now
	if err := Dock17Closelid_Step9(ctx, s, tconn, original_ext); err != nil {
		s.Fatal("Failed to execute step9: ", err)
	}

}

// 3. Connect external monitor to the chromebook. (switch Type-C & HDMI fixture)
func Dock17Closelid_Step3(ctx context.Context, s *testing.State) error {

	s.Log("Step 3 - Connect ext-display to station")

	if err := utils.ControlFixture(ctx, s, utils.FixtureExtDisp1, utils.ActionPlugin, false); err != nil {
		return errors.Wrap(err, "failed to connect ext-display to docking station")
	}

	return nil
}

// connect station to chromebook
func Dock17Closelid_Step4(ctx context.Context, s *testing.State) error {

	s.Log("Step 4 - Connect station to chromebook")

	if err := utils.ControlFixture(ctx, s, utils.FixtureStation, utils.ActionPlugin, false); err != nil {
		return errors.Wrap(err, "failed to connect station to chromebook")
	}

	return nil
}

// 4. Check external monitor display properly and remember the resolution
func Dock17Closelid_Step5(ctx context.Context, s *testing.State, tconn *chrome.TestConn) (*display.Info, error) {

	s.Log("Step 5 - Check external monitor display properly and remember the resolution")

	infos, err := display.GetInfo(ctx, tconn)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get display info")
	}

	// check num of diplay
	if len(infos) != 2 {
		return nil, errors.Errorf("failed to get right num of display, got %d, want 2", len(infos))
	}

	return &infos[1], nil
}

// 5. Open any app on internal monitor.
func Dock17Closelid_Step6(ctx context.Context, s *testing.State, a *arc.ARC, tconn *chrome.TestConn) error {

	s.Log("Step 6- Open any app on internal monitor")

	if err := utils.StartActivityOnDisplay(ctx, a, tconn, utils.SettingsPkg, utils.SettingsAct, 0); err != nil {
		s.Fatal("Failed to start activity on display: ")
	}

	return nil
}

// 6. Close internal monitor power.
func Dock17Closelid_Step7(ctx context.Context, s *testing.State) error {

	s.Log("Step 7 - Close internal monitor power")

	if err := utils.SetDisplayPower(ctx, utils.DisplayPowerInternalOffExternalOn); err != nil {
		return errors.Wrap(err, "failed to close internal moniter power")
	}

	return nil
}

// 7. Check window bounds on external monitor display
func Dock17Closelid_Step8(ctx context.Context, s *testing.State, tconn *chrome.TestConn) error {

	s.Log("Step 8 - Check window bounds on external monitor display")

	// get external display
	ext, err := utils.GetExternalDisplay(ctx, s, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get external display")
	}

	// ensure setting window on external display
	if err := utils.EnsureWindowOnDisplay(ctx, tconn, utils.SettingsPkg, ext.ID); err != nil {
		return errors.Wrap(err, "failed to ensure window on external display")
	}

	return nil
}

// 8 Compare external monitor old resolution and resolution for now
func Dock17Closelid_Step9(ctx context.Context, s *testing.State, tconn *chrome.TestConn, wasExtDisp *display.Info) error {

	s.Log("Step 9 - Compare external monitor old resolution and resolution for now")

	// get external display
	ext, err := utils.GetExternalDisplay(ctx, s, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get external dispaly")
	}

	// check resolution
	if wasExtDisp.Bounds.Width != ext.Bounds.Width || wasExtDisp.Bounds.Height != ext.Bounds.Height {

		return errors.New("failed to verify display resolution: ")
	}

	return nil
}
