// #2 Windows Persistent settings for dual display through a Dock
// Pre-Condition:
// (Please note: Brand / Model number on test result)
// 1. External displays
// 2. Docking station
// 3. Connection Type (RunOrFatalRunOrFatal/DP/VGA/DVI/USB-C on test result)

// Procedure:
// 1)  Boot-up and Sign-In to the device and enable "persistent window bounds in multi-displays scenario".
// 2)  Connect two ext-display to (Docking station)
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
// *  Unplug: (two Chrome App windows) will bounds back to Internal Primary Display
// *  Plug-In: (two Chrome App windows) will bounds back to Ext-Display

// 6) Change External display as primary display using Alt+ F4 / Display settings.
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
		Func: Dock2PersistentSettings,
		Desc: "Test persistent settings of window bound placement across displays in one use session via a Dock		",
		Contacts:     []string{"allion-sw@allion.com"},
		SoftwareDeps: []string{"arc", "chrome"},
		Timeout:      4 * time.Minute, // 1)  Boot-up and Sign-In to the device and enable "persistent window bounds in multi-displays scenario".
		Pre:          arc.Booted(),
		Vars:         utils.GetInputVars(),
	})
}

func Dock2PersistentSettings(ctx context.Context, s *testing.State) {

	cr := s.PreValue().(arc.PreData).Chrome
	a := s.PreValue().(arc.PreData).ARC
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	s.Logf("Step 1 - Boot up and sign in ")

	// step 2 - connect two ext-display to station
	if err := Dock2PersistentSettings_Step2(ctx, s); err != nil {
		s.Fatal("Failed to execute step2: ", err)
	}

	time.Sleep(3 * time.Second)

	// step 3 - connect station to chromebook
	if err := Dock2PersistentSettings_Step3(ctx, s, tconn); err != nil {
		s.Fatal("Failed to execute step3 : ", err)
	}

	time.Sleep(3 * time.Second)

	// step 4 - open apps on internal and external display
	if err := Dock2PersistentSettings_Step4(ctx, s, tconn, a); err != nil {
		s.Fatal("Failed to execute step4: ", err)
	}

	time.Sleep(3 * time.Second)

	// step 5 - unplug and re-plug in
	if err := Dock2PersistentSettings_Step5(ctx, s, tconn); err != nil {
		s.Fatal("Failed to execute step5: ", err)
	}

	time.Sleep(3 * time.Second)

	// step 6 - test primary display
	if err := Dock2PersistentSettings_Step6(ctx, s, tconn, a); err != nil {
		s.Fatal("Failed to execute step6: ", err)
	}

	time.Sleep(3 * time.Second)

	// step 7 - unplug and re-plug in - as same as step 5
	if err := Dock2PersistentSettings_Step7(ctx, s, tconn); err != nil {
		s.Fatal("Failed to execute step7: ", err)
	}

	time.Sleep(3 * time.Second)

	// step 8 - test mirror mode
	if err := Dock2PersistentSettings_Step8(ctx, s, tconn, a); err != nil {
		s.Fatal("Failed to execute step8: ", err)
	}
}

// 2)  Connect two ext-display to (Docking station)
func Dock2PersistentSettings_Step2(ctx context.Context, s *testing.State) error {

	s.Logf("Step 2 - Connect two ext-display to docking station")

	// plug in ext-display

	if err := utils.ControlFixture(ctx, s, utils.FixtureExtDisp1, utils.ActionPlugin, false); err != nil {
		return errors.Wrap(err, "Failed to connect ext-display1 to docking station: ")
	}

	if err := utils.ControlFixture(ctx, s, utils.FixtureExtDisp2, utils.ActionPlugin, false); err != nil {
		return errors.Wrap(err, "Failed to connect ext-display2 to docking station: ")
	}

	return nil
}

// 3)  Connect (Docking station) to Chromebook
// -  Now State:
// *  Internal display will show up as (Primary)
// *  External display will show up as (Extended)
func Dock2PersistentSettings_Step3(ctx context.Context, s *testing.State, tconn *chrome.TestConn) error {

	s.Logf("Step 3 - Connect docking station to chromebook then check state")

	// plug in docking station
	if err := utils.ControlFixture(ctx, s, utils.FixtureStation, utils.ActionPlugin, false); err != nil {
		return errors.Wrap(err, "Failed to plug in docking station: ")
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {

		// get display
		infos, err := display.GetInfo(ctx, tconn)
		if err != nil {
			return errors.Wrap(err, "Failed to get display info: ")
		}

		// check num of display
		if len(infos) != 3 {
			return errors.Errorf("num of infos is not enough, got %d, want 3", len(infos))
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
		return err
	}

	return nil
}

// 4)  Open (and drag if needed) two chrome windows/ App windows.
// - Now State:
// *  Internal display (Primary) : Internal.window
// *  External display (Extended): Monitor. window
// - Now State:
// *  Two Chrome App windows bounds on External display.
func Dock2PersistentSettings_Step4(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC) error {

	s.Logf("Step 4 - Open apps on internal & external display ")

	// get display info
	infos, err := display.GetInfo(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "Failed to get display info: ")
	}

	// check num of display
	if len(infos) != 3 {
		return errors.Errorf("Failed to get enough display, got %d, want 3", len(infos))
	}

	// install app
	if err := a.Install(ctx, arc.APKPath(utils.TestappApk)); err != nil {
		return errors.Wrap(err, "Failed installing app: ")
	}

	for _, param1 := range []struct {
		displayID  int
		isCloseAct bool
	}{
		{1, true},
		{2, false},
	} {
		// start two activity on external display
		for _, param2 := range []struct {
			pkgName string
			actName string
		}{
			{utils.SettingsPkg, utils.SettingsAct},
			{utils.TestappPkg, utils.TestappAct},
		} {
			s.Logf("Start [%s] window on external display - %d ", param2.pkgName, param1.displayID)

			if err := utils.StartActivityOnDisplay(ctx, a, tconn, param2.pkgName, param2.actName, 1); err != nil {
				return errors.Wrap(err, "Failed to start window on display: ")
			}

			window, err := ash.FindWindow(ctx, tconn, func(w *ash.Window) bool {
				return w.ARCPackageName == param2.pkgName
			})
			if err != nil {
				return errors.Wrap(err, "Failed to find window: ")
			}

			if param1.isCloseAct == true {
				s.Logf("Close [%s] window on display - %d", param2.pkgName, param1.displayID)

				if err := window.CloseWindow(ctx, tconn); err != nil {
					return err
				}
			}

		}

	}

	return nil

}

// 5) Unplug and re-plug-in External display.
// - Now State:
// *  Unplug: (two Chrome App windows) will bounds back to Internal Primary Display
// *  Plug-In: (two Chrome App windows) will bounds back to Ext-Display
func Dock2PersistentSettings_Step5(ctx context.Context, s *testing.State, tconn *chrome.TestConn) error {

	s.Logf("Step 5 - Unplug and re-plug in ext-display ")

	for _, param1 := range []struct {
		action    utils.ActionState
		dispIndex int
	}{
		{utils.ActionUnplug, 0},
		{utils.ActionPlugin, 2},
	} {
		// control ext-display1
		if err := utils.ControlFixture(ctx, s, utils.FixtureExtDisp1, param1.action, false); err != nil {
			return errors.Wrap(err, "Failed to disconnect ext-display from docking station: ")
		}

		// control ext-display2
		if err := utils.ControlFixture(ctx, s, utils.FixtureExtDisp1, param1.action, false); err != nil {
			return errors.Wrap(err, "Failed to disconnect ext-display2 from station: ")
		}

		for _, param2 := range []struct {
			pkgName string
		}{
			{utils.SettingsPkg},
			{utils.TestappPkg},
		} {
			s.Logf("Checking %s window is on %s display", param2.pkgName, param1.dispIndex)

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

// 6) Change External display as primary display using Alt+ F4 / Display settings.
// - Now State:
// *  External display will become (Primary) display
// *  Primary display will become (Extended) display
// **Make note of window bounds on External display.
func Dock2PersistentSettings_Step6(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC) error {

	s.Logf("Step 6 - Test primary mode ")

	// get display info
	infos, err := display.GetInfo(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "Failed to get display info")
	}

	// check external is enough for test
	if len(infos) != 3 {
		return errors.Errorf("Num of display is not right, got %d, want 3", len(infos))
	}

	// make sure internal display is primary
	if err := utils.EnsureDisplayIsPrimary(ctx, s, tconn, &infos[0]); err != nil {
		return errors.Wrapf(err, "Failed to ensure internal is primary: ")
	}

	s.Logf("Reopen all windows on internal display")

	// reopen window on internal
	if err := utils.ReopenAllWindowsOnInternal(ctx, s, tconn, a); err != nil {
		return errors.Wrap(err, "Failed to reopen window on internal display")
	}

	time.Sleep(3 * time.Second)

	s.Logf("Let external display 2 become primary ")

	// set second external display to be primary
	if err := utils.EnsureDisplayIsPrimary(ctx, s, tconn, &infos[2]); err != nil {
		return errors.Wrap(err, "Failed to set second external display to be primary")
	}

	time.Sleep(3 * time.Second)

	// retry check in 5s
	if err := testing.Poll(ctx, func(ctx context.Context) error {

		// get primary info to compare
		primaryInfo, err := display.GetPrimaryInfo(ctx, tconn)
		if err != nil {
			return errors.Wrap(err, "Failed to get primary display info")
		}

		// check external display 2 become primary
		if infos[2].ID != primaryInfo.ID {
			return errors.Wrapf(err, "Failed to let external display become primary")
		}

		// ensure two app on external display
		for _, param := range []struct {
			packageName string
		}{
			{utils.SettingsPkg},
			{utils.TestappPkg},
		} {

			ensure := utils.EnsureWindowOnDisplay(ctx, tconn, param.packageName, infos[2].ID)
			if err := ensure; err != nil {
				return errors.Wrapf(err, "Failed to ensure [%s] window on display {seq:%d,ID:%s, Name:%s} ",
					param.packageName, 2, infos[2].ID, infos[2].Name)
			}

		}
		return nil
	}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
		return err
	}
	return nil

}

// as same as step5
// 7) Unplug and re-plug-in External display.
// - Now State:
// *  External display window should switch between internal and external displays using previous window bounds.
func Dock2PersistentSettings_Step7(ctx context.Context, s *testing.State, tconn *chrome.TestConn) error {

	s.Logf("Step 7 - Unplug and re-plug in ext-display ")

	for _, param1 := range []struct {
		action    utils.ActionState
		dispIndex int
	}{
		{utils.ActionUnplug, 0},
		{utils.ActionPlugin, 2},
	} {
		// control ext-display1
		if err := utils.ControlFixture(ctx, s, utils.FixtureExtDisp1, param1.action, false); err != nil {
			return errors.Wrap(err, "Failed to disconnect ext-display from docking station: ")
		}

		// control ext-display2
		if err := utils.ControlFixture(ctx, s, utils.FixtureExtDisp2, param1.action, false); err != nil {
			return errors.Wrap(err, "Failed to disconnect ext-display2 from station: ")
		}

		for _, param2 := range []struct {
			pkgName string
		}{
			{utils.SettingsPkg},
			{utils.TestappPkg},
		} {
			s.Logf("Checking %s window is on %s display", param2.pkgName, param1.dispIndex)

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
func Dock2PersistentSettings_Step8(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC) error {

	s.Logf("Step 8 - Test mirror mode ")

	// get display info
	infos, err := display.GetInfo(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "Failed to get display info: ")
	}

	// make sure internal display is primary
	if err := utils.EnsureDisplayIsPrimary(ctx, s, tconn, &infos[0]); err != nil {
		return errors.Wrapf(err, "Failed to ensure internal is primary: ")
	}

	// waiting for acceptable time
	time.Sleep(3 * time.Second)

	s.Logf("Enter mirror mode, then check display's mirror source id")

	// declare keyboard object
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "Failed to find keyboard")
	}
	defer kb.Close()

	// Press Ctrl+F4, enter or exit mirror mode
	enterMirror := kb.Accel(ctx, "Ctrl+F4")
	if err := enterMirror; err != nil {
		return errors.Wrapf(err, "Failed to enter mirror mode")
	}

	time.Sleep(3 * time.Second)

	// retry in 5s
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

	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		return err
	}

	s.Logf("Exit mirror mode, then check display attributes and windows is on internal display")

	time.Sleep(3 * time.Second)

	// Press Ctrl+F4, exit mirror mode
	exitMirror := kb.Accel(ctx, "Ctrl+F4")
	if err := exitMirror; err != nil {
		return errors.Wrap(err, "Failed to exit mirror mode")
	}

	time.Sleep(3 * time.Second)

	// retry in 5s
	if err := testing.Poll(ctx, func(ctx context.Context) error {

		// get display info in mirror mode
		infos, err := display.GetInfo(ctx, tconn)
		if err != nil {
			return errors.Wrap(err, "Failed to get display infos after exit mirror mode")
		}

		// num of display should be 3 after exiting mirror mode
		if len(infos) != 3 {
			return errors.Errorf("Failed to get right num of display, got %d, want 3", len(infos))
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
		for _, param := range []struct {
			packgeName string
		}{
			{utils.SettingsPkg},
			{utils.SettingsAct},
		} {

			dispInfo := infos[0]

			// ensure window on display
			ensureWin := utils.EnsureWindowOnDisplay(ctx, tconn, param.packgeName, dispInfo.ID)
			if err := ensureWin; err != nil {
				return errors.Wrapf(err, "Failed to ensure window on display {pkgName:%s, seq:%d, ID:%s, Name:%s} ",
					param.packgeName, 0, dispInfo.ID, dispInfo.Name)
			}
		}

		return nil

	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		return err
	}

	return nil
}
