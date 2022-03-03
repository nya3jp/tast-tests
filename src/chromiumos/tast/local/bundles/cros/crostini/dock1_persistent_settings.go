// Pre-Condition:
// (Please note: Brand / Model number on test result)
// 1. External displays
// 2. Docking station
// 3. Connection Type (HDMI/DP/VGA/DVI/USB-C on test result)

// Procedure:
// 1)  Boot-up and Sign-In to the device and enable "persistent window bounds in multi-displays scenario".
// 2)  Connect ext-display to (Docking station)
// 3)  Connect (Docking station) to Chromebook
// -  Now State:
// *  Internal display will show up as (Primary)
// *  External display will show up as (Extended)

// 4)  Open (and drag if needed) two chrome windows/ App windows.
// - Now State:
// *  Internal display (Primary) : Internal.window
// *  External display (Extended): Monitor. window
// - Now State:
// *  Two Chrome App windows bounds on External display.

// 5) Unplug and re-plug-in External display.
// - Now State:
// ** Unplug: (two Chrome App windows) will bounds back to Internal Primary Display
// ** Plug-In: (two Chrome App windows) will bounds back to Ext-Display

// 6) Moving two window/app to Primary display, then change External display as primary display using Alt+ F4 / Display settings.
// - Now State:
// *  External display will become (Primary) display
// *  Primary display will become (Extended) display
// **Make note of window bounds on External display.

// 7) Unplug and re-plug-in External display.
// - Now State:
// *  External display window should switch between internal and external displays using previous window bounds.

// 8) Change External display to (Mirror) mode using Ctrl+F4 / Display settings and then exit (Mirror) mode.
// - Now State:
// *  Both Primary and External display window should show up as (Mirror) mode
// *  After exit (Mirror) mode Internal display show as (Primary) display and External display show as (Extended) display
// *  External display window should switch between internal and external displays using previous window bounds.

// Note:
// Known Issues: crbug.com/821611 , crbug.com/821614
package crostini

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/crostini/utils"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Dock1PersistentSettings,
		Desc:         "Test persistent settings of window bound placement across displays in one use session via a Dock",
		Contacts:     []string{"allion-sw@allion.com"},
		SoftwareDeps: []string{"arc", "chrome"},
		Timeout:      4 * time.Minute,
		Pre:          arc.Booted(), // 1)  Boot-up and Sign-In to the device
		Vars:         utils.GetInputVars(),
	})
}

func Dock1PersistentSettings(ctx context.Context, s *testing.State) {

	cr := s.PreValue().(arc.PreData).Chrome
	a := s.PreValue().(arc.PreData).ARC
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	s.Logf("Step 1 - Boot-up and Sign-In to the device  ")

	// step 2 - connect ext-display to station
	if err := Dock1PersistentSettings_Step2(ctx, s); err != nil {
		s.Fatal("Failed to execute step2: ", err)
	}

	// step 3 - connect station to chromebook
	if err := Dock1PersistentSettings_Step3(ctx, s, tconn); err != nil {
		s.Fatal("Failed to execute step3: ", err)
	}

	// step 4 - open two apps on external display
	if err := Dock1PersistentSettings_Step4(ctx, s, tconn, a); err != nil {
		s.Fatal("Failed to execute step4: ", err)
	}

	// step 5 - unplug and re-plug in, check window bounds which display
	if err := Dock1PersistentSettings_Step5(ctx, s, tconn); err != nil {
		s.Fatal("Failed to execute step5: ", err)
	}

	// step 6 - Switch to primary mode, check
	if err := Dock1PersistentSettings_Step6(ctx, s, tconn, a); err != nil {
		s.Fatal("Failed to execute Step6: ", err)
	}

	// step 7 - unplug and re-plug in
	if err := Dock1PersistentSettings_Step7(ctx, s, tconn); err != nil {
		s.Fatal("Failed to execute step7: ", err)
	}

	// step 8 - test mirror mode
	if err := Dock1PersistentSettings_Step8(ctx, s, tconn, a); err != nil {
		s.Fatal("Failed to execute step8: ", err)
	}
}

// 2)  Connect ext-display to (Docking station)
func Dock1PersistentSettings_Step2(ctx context.Context, s *testing.State) error {

	s.Logf("Step 2 - Connect ext-display to (Docking station) ")

	if err := utils.ControlFixture(ctx, s, utils.FixtureExtDisp1, utils.ActionPlugin, false); err != nil {
		return errors.Wrap(err, "Failed to connect ext-display to docking station: ")
	}

	return nil
}

// 3)  Connect (Docking station) to Chromebook
// -  Now State:
// *  Internal display will show up as (Primary)
// *  External display will show up as (Extended)
func Dock1PersistentSettings_Step3(ctx context.Context, s *testing.State, tconn *chrome.TestConn) error {

	s.Logf("Step 3 - Connect (Docking station) to Chromebook ")

	if err := utils.ControlFixture(ctx, s, utils.FixtureStation, utils.ActionPlugin, false); err != nil {
		return errors.Wrapf(err, "Failed to plug in docking station: ")
	}

	// verification
	if err := testing.Poll(ctx, func(c context.Context) error {

		// get display
		infos, err := display.GetInfo(ctx, tconn)
		if err != nil {
			return errors.Wrap(err, "Failed to get display info: ")
		}

		// check num of display
		if len(infos) < 2 {
			return errors.Errorf("Failed to get enough display, got %d, at lest 2", len(infos))
		}

		// -  Now State:
		// *  Internal display will show up as (Primary)
		// *  External display will show up as (Extended)
		for _, info := range infos {
			// internal
			if info.IsInternal == true {
				if info.IsPrimary == false {
					return errors.Wrap(err, "Internal display should show up as (Primary)")
				}
			}
			// external
			if info.IsInternal == false {
				if info.IsPrimary == true {
					return errors.Wrap(err, "External display should show up as (Extended)")
				}
			}
		}

		return nil

	}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
		return errors.Wrap(err, "Failed to verify display status: ")
	}

	return nil
}

// 4)  Open (and drag if needed) two chrome windows/ App windows.
// - Now State:
// *  Internal display (Primary) : Internal.window
// *  External display (Extended): Monitor. window
// - Now State:
// *  Two Chrome App windows bounds on External display.
func Dock1PersistentSettings_Step4(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC) error {

	s.Logf("Step 4 - Open (and drag if needed) two chrome windows/ App windows.")

	// install apk for testing
	if err := a.Install(ctx, arc.APKPath(utils.TestappApk)); err != nil {
		return errors.Errorf("Failed installing app: ")
	}

	// start two activity on external display - 1
	for _, param := range []struct {
		pkgName string
		actName string
	}{
		{utils.SettingsPkg, utils.SettingsAct},
		{utils.TestappPkg, utils.TestappAct},
	} {

		s.Logf("Open %s on display 1", param.pkgName)

		if err := utils.StartActivityOnDisplay(ctx, a, tconn, param.pkgName, param.actName, 1); err != nil {
			return errors.Wrap(err, "Failed to start activity on display: ")
		}

		// set window as normal state, in case the window won't jump back to external
		if state, err := ash.SetARCAppWindowState(ctx, tconn, param.pkgName, ash.WMEventNormal); err != nil {
			s.Errorf("Failed to set window state to %s for Settings app: %v", ash.WMEventNormal, err)
		} else if state != ash.WindowStateNormal {
			s.Errorf("Unexpected window state: got %s; want %s", state, ash.WindowStateNormal)
		}
	}

	return nil
}

// 5) Unplug and re-plug-in External display.
// - Now State:
// ** Unplug: (two Chrome App windows) will bounds back to Internal Primary Display
// ** Plug-In: (two Chrome App windows) will bounds back to Ext-Display
func Dock1PersistentSettings_Step5(ctx context.Context, s *testing.State, tconn *chrome.TestConn) error {

	s.Logf("Step 5 - Unplug and re-plug-in External display.")

	for _, param1 := range []struct {
		action    utils.ActionState
		dispIndex int
	}{
		{utils.ActionUnplug, 0},
		{utils.ActionPlugin, 1},
	} {
		// unplug display
		if err := utils.ControlFixture(ctx, s, utils.FixtureExtDisp1, param1.action, false); err != nil {
			return errors.Wrap(err, "Failed to disconnect ext-display from docking station: ")
		}

		for _, param2 := range []struct {
			pkgName string
		}{
			{utils.SettingsPkg},
			{utils.TestappPkg},
		} {
			s.Logf("Checking %s window is on %d display", param2.pkgName, param1.dispIndex)

			// retry in 30s
			if err := testing.Poll(ctx, func(ctx context.Context) error {

				infos, err := display.GetInfo(ctx, tconn)
				if err != nil {
					return err
				}

				if err := utils.EnsureWindowOnDisplay(ctx, tconn, param2.pkgName, infos[param1.dispIndex].ID); err != nil {
					return errors.Wrapf(err, "Failed to ensure %s window on display %d: ", param2.pkgName, param1.dispIndex)
				}

				return nil

			}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
				return err
			}

		}

	}

	return nil
}

// 6) Moving two window/app to Primary display, then change External display as primary display using Alt+ F4 / Display settings.
// - Now State:
// *  External display will become (Primary) display
// *  Primary display will become (Extended) display
// **Make note of window bounds on External display.
func Dock1PersistentSettings_Step6(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC) error {

	s.Logf("Step 6 - Test primary mode ")

	// get display info
	infos, err := display.GetInfo(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "Failed to get display info: ")
	}

	// make sure internal display is primary
	if err := utils.EnsureDisplayIsPrimary(ctx, s, tconn, &infos[0]); err != nil {
		return errors.Wrapf(err, "Failed to ensure internal is primary: ")
	}

	// reopen window on internal
	if err := utils.ReopenAllWindowsOnInternal(ctx, s, tconn, a); err != nil {
		return errors.Wrap(err, "Failed to reopen window on internal display: ")
	}

	// need to delay, in case execution failed
	time.Sleep(5 * time.Second)

	s.Logf("Let ext-display 1 become primary ")

	// set first external display to be primary
	if err := utils.EnsureDisplayIsPrimary(ctx, s, tconn, &infos[1]); err != nil {
		return errors.Wrap(err, "Failed to set ext-display 1 to be primary: ")
	}

	// retry checking in 30s
	if err := testing.Poll(ctx, func(ctx context.Context) error {

		// ensure two app on external display1
		for _, param := range []struct {
			packageName string
		}{
			{utils.SettingsPkg},
			{utils.TestappPkg},
		} {
			ensureWin := utils.EnsureWindowOnDisplay(ctx, tconn, param.packageName, infos[1].ID)
			if err := ensureWin; err != nil {
				return errors.Wrapf(err, "Failed to ensure [%s] window on display { Seq:%d, Id:%s, Name:%s} ",
					&infos[1], 1, &infos[1].ID, &infos[1].Name)
			}

		}

		return nil

	}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
		return err
	}

	return nil
}

// 7) Unplug and re-plug-in External display.
// - Now State:
// *  External display window should switch between internal and external displays using previous window bounds.
func Dock1PersistentSettings_Step7(ctx context.Context, s *testing.State, tconn *chrome.TestConn) error {

	s.Logf("Step 7 - Unplug and re-plug external display ")

	for _, param1 := range []struct {
		action    utils.ActionState
		dispIndex int
	}{
		{utils.ActionUnplug, 0},
		{utils.ActionPlugin, 1},
	} {

		// unplug display
		if err := utils.ControlFixture(ctx, s, utils.FixtureExtDisp1, param1.action, false); err != nil {
			return errors.Wrap(err, "Failed to control fixture: ")
		}

		for _, param2 := range []struct {
			pkgName string
		}{
			{utils.SettingsPkg},
			{utils.TestappPkg},
		} {
			s.Logf("Checking %s window is on %d display", param2.pkgName, param1.dispIndex)

			// retry in 30s
			if err := testing.Poll(ctx, func(ctx context.Context) error {

				infos, err := display.GetInfo(ctx, tconn)
				if err != nil {
					return err
				}

				if err := utils.EnsureWindowOnDisplay(ctx, tconn, param2.pkgName, infos[param1.dispIndex].ID); err != nil {
					return errors.Wrapf(err, "Failed to ensure %s window on display %d: ", param2.pkgName, param1.dispIndex)
				}

				return nil

			}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
				return err
			}

		}

	}

	return nil
}

// 8) Change External display to (Mirror) mode using Ctrl+F4 / Display settings and then exit (Mirror) mode.
// - Now State:
// *  Both Primary and External display window should show up as (Mirror) mode
// *  After exit (Mirror) mode Internal display show as (Primary) display and External display show as (Extended) display
// *  External display window should switch between internal and external displays using previous window bounds.
func Dock1PersistentSettings_Step8(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC) error {

	s.Logf("Step 8 - [Test mirror mode] ")

	// get display info
	infos, err := display.GetInfo(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "Failed to get display info: ")
	}

	// make sure internal display is primary
	if err := utils.EnsureDisplayIsPrimary(ctx, s, tconn, &infos[0]); err != nil {
		return errors.Wrapf(err, "Failed to ensure internal is primary: ")
	}

	s.Logf("Enter mirror mode, then check display's mirror source id")

	// declare keyboard object
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "Failed to find keyboard")
	}
	defer kb.Close()

	// send "Ctrl+F4" to enter mirror mode
	if err := kb.Accel(ctx, "Ctrl+F4"); err != nil {
		return errors.Wrapf(err, "Failed to enter mirror mode")
	}

	// retry checking in 30s
	if err := testing.Poll(ctx, func(ctx context.Context) error {

		// get internal display info
		intDispInfo, err := display.GetInternalInfo(ctx, tconn)
		if err != nil {
			return errors.Wrap(err, "Faild to get internal display info")
		}

		// get display info in mirror mode
		infos, err := display.GetInfo(ctx, tconn)
		if err != nil {
			return errors.Wrap(err, "Failed to get display infos in mirror mode")
		}

		// num of display should be just one in mirror mode
		if len(infos) > 1 {
			return errors.Errorf("Failed to get right num of display, got %d, want 1", len(infos))
		}

		// check mirror source id
		for _, info := range infos {
			if intDispInfo.ID != info.MirroringSourceID {
				return errors.Wrap(err, "Failed to check mirror source id")
			}
		}

		return nil

	}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
		return err
	}

	s.Logf("Exit mirror mode, then check display attributes and windows in on internal display")

	time.Sleep(5 * time.Second)

	// send "Ctrl+F4" to exit mirror mode
	if err := kb.Accel(ctx, "Ctrl+F4"); err != nil {
		return errors.Wrap(err, "Failed to exit mirror mode")
	}

	// retry checking in 30s
	if err := testing.Poll(ctx, func(ctx context.Context) error {

		// get display info in mirror mode
		infos, err := display.GetInfo(ctx, tconn)
		if err != nil {
			return errors.Wrap(err, "Failed to get display infos after exit mirror mode")
		}

		// After exit (Mirror) mode Internal display show as (Primary) display and External display show as (Extended) display
		for _, info := range infos {
			if info.IsInternal == true { // internal
				if info.IsPrimary == false { // shall be primary
					return errors.Wrap(err, "Failed to check internal is primary after exit mirror mode")
				}
			} else if info.IsInternal == false { // external
				if info.IsPrimary == true { // shall not be primary
					return errors.Wrap(err, "Failed to check external is not primary after exit mirror mode")
				}
			}
		}

		// ensure 2 apps on internal display
		if err := utils.EnsureWindowOnDisplay(ctx, tconn, utils.SettingsPkg, infos[0].ID); err != nil {
			return errors.Wrap(err, "Failed to ensure setting window on internal display: ")
		}

		if err := utils.EnsureWindowOnDisplay(ctx, tconn, utils.TestappPkg, infos[0].ID); err != nil {
			return errors.Wrap(err, "Failed to ensure testapp window on internal display: ")
		}

		return nil

	}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
		return err
	}

	return nil
}
