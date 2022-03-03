// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

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

// Package wwcb contains local Tast tests that work with chromebook
package wwcb

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/wwcb/utils"
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
		Fixture:      "arcBooted",
		Vars:         utils.InputArguments,
	})
}

func Dock2PersistentSettings(ctx context.Context, s *testing.State) {

	cr := s.FixtValue().(*arc.PreData).Chrome
	a := s.FixtValue().(*arc.PreData).ARC
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	if err := utils.InitFixtures(ctx); err != nil {
		s.Fatal("Failed to initialize fixtures: ", err)
	}

	s.Log("Step 1 - Boot up and sign in ")

	// step 2 - connect two ext-display to station
	if err := dock2PersistentSettingsStep2(ctx, s); err != nil {
		s.Fatal("Failed to execute step2: ", err)
	}

	testing.Sleep(ctx, 3*time.Second)

	// step 3 - connect station to chromebook
	if err := dock2PersistentSettingsStep3(ctx, s, tconn); err != nil {
		s.Fatal("Failed to execute step3 : ", err)
	}

	testing.Sleep(ctx, 3*time.Second)

	// step 4 - open apps on internal and external display
	if err := dock2PersistentSettingsStep4(ctx, s, tconn, a); err != nil {
		s.Fatal("Failed to execute step4: ", err)
	}

	testing.Sleep(ctx, 3*time.Second)

	// step 5 - unplug and re-plug in, check window bounds on which display
	if err := dock2PersistentSettingsStep5(ctx, s, tconn); err != nil {
		s.Fatal("Failed to execute step5: ", err)
	}

	testing.Sleep(ctx, 3*time.Second)

	// step 6 - test primary display
	if err := dock2PersistentSettingsStep6(ctx, s, tconn, a); err != nil {
		s.Fatal("Failed to execute step6: ", err)
	}

	testing.Sleep(ctx, 3*time.Second)

	// step 7 - unplug and re-plug in, check window bounds on display
	if err := dock2PersistentSettingsStep7(ctx, s, tconn); err != nil {
		s.Fatal("Failed to execute step7: ", err)
	}

	testing.Sleep(ctx, 3*time.Second)

	// step 8 - test mirror mode
	if err := dock2PersistentSettingsStep8(ctx, s, tconn, a); err != nil {
		s.Fatal("Failed to execute step8: ", err)
	}
}

func dock2PersistentSettingsStep2(ctx context.Context, s *testing.State) error {

	s.Log("Step 2 - Connect two ext-display to docking station")

	// plug in 1st ext-display
	if err := utils.ControlFixture(ctx, s, utils.ExtDisp1Type, utils.ExtDisp1Index, utils.ActionPlugin, false); err != nil {
		return errors.Wrap(err, "failed to connect ext-display1 to docking station")
	}

	// plug in 2nd ext-display
	if err := utils.ControlFixture(ctx, s, utils.ExtDisp2Type, utils.ExtDisp2Index, utils.ActionPlugin, false); err != nil {
		return errors.Wrap(err, "failed to connect ext-display2 to docking station")
	}

	return nil
}

func dock2PersistentSettingsStep3(ctx context.Context, s *testing.State, tconn *chrome.TestConn) error {

	s.Log("Step 3 - Connect docking station to chromebook then check state")

	// plug in docking station
	if err := utils.ControlFixture(ctx, s, utils.StationType, utils.StationIndex, utils.ActionPlugin, false); err != nil {
		return errors.Wrap(err, "failed to plug in docking station")
	}

	if err := utils.VerifyDisplayState(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to verify display count")
	}

	return nil
}

func dock2PersistentSettingsStep4(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC) error {

	s.Log("Step 4 - Open apps on internal & external display ")

	// get display info
	infos, err := display.GetInfo(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get display info")
	}

	// check num of display
	if len(infos) != 3 {
		return errors.Errorf("failed to get enough display, got %d, want 3", len(infos))
	}

	// install app
	if err := a.Install(ctx, arc.APKPath(utils.TestappApk)); err != nil {
		return errors.Wrap(err, "failed installing app")
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
				return errors.Wrap(err, "failed to start window on display")
			}

			window, err := ash.FindWindow(ctx, tconn, func(w *ash.Window) bool {
				return w.ARCPackageName == param2.pkgName
			})
			if err != nil {
				return errors.Wrap(err, "failed to find window")
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

func dock2PersistentSettingsStep5(ctx context.Context, s *testing.State, tconn *chrome.TestConn) error {

	s.Log("Step 5 - Unplug and re-plug in ext-display ")

	for _, param1 := range []struct {
		action    utils.ActionState
		dispIndex int
	}{
		{utils.ActionUnplug, 0},
		{utils.ActionPlugin, 2},
	} {
		// control ext-display1
		if err := utils.ControlFixture(ctx, s, utils.ExtDisp1Type, utils.ExtDisp1Index, param1.action, false); err != nil {
			return errors.Wrap(err, "failed to disconnect ext-display from docking station")
		}

		// control ext-display2
		if err := utils.ControlFixture(ctx, s, utils.ExtDisp2Type, utils.ExtDisp2Index, param1.action, false); err != nil {
			return errors.Wrap(err, "failed to disconnect ext-display2 from station")
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
					return errors.Wrapf(err, "failed to ensure %s window on display %d: ", param2.pkgName, param1.dispIndex)
				}

				return nil

			}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
				return err
			}

		}

	}

	return nil
}

func dock2PersistentSettingsStep6(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC) error {

	s.Log("Step 6 - Test primary mode ")

	// get display info
	infos, err := display.GetInfo(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get display info")
	}

	// check external is enough for test
	if len(infos) != 3 {
		return errors.Errorf("Num of display is not right, got %d, want 3", len(infos))
	}

	// make sure internal display is primary
	if err := utils.EnsureDisplayIsPrimary(ctx, tconn, &infos[0]); err != nil {
		return errors.Wrap(err, "failed to ensure internal is primary")
	}

	s.Log("Reopen all windows on internal display")

	// reopen window on internal
	if err := utils.ReopenAllWindowsOnInternal(ctx, tconn, a); err != nil {
		return errors.Wrap(err, "failed to reopen window on internal display")
	}

	testing.Sleep(ctx, 3*time.Second)

	s.Log("Let external display 2 become primary ")

	// set second external display to be primary
	if err := utils.EnsureDisplayIsPrimary(ctx, tconn, &infos[2]); err != nil {
		return errors.Wrap(err, "failed to set second external display to be primary")
	}

	testing.Sleep(ctx, 3*time.Second)

	// retry check in 5s
	if err := testing.Poll(ctx, func(ctx context.Context) error {

		// get primary info to compare
		primaryInfo, err := display.GetPrimaryInfo(ctx, tconn)
		if err != nil {
			return errors.Wrap(err, "failed to get primary display info")
		}

		// check external display 2 become primary
		if infos[2].ID != primaryInfo.ID {
			return errors.Wrap(err, "failed to let external display become primary")
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
				return errors.Wrapf(err, "failed to ensure [%s] window on display {seq:%d,ID:%s, Name:%s} ",
					param.packageName, 2, infos[2].ID, infos[2].Name)
			}

		}
		return nil
	}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
		return err
	}
	return nil

}

func dock2PersistentSettingsStep7(ctx context.Context, s *testing.State, tconn *chrome.TestConn) error {

	s.Log("Step 7 - Unplug and re-plug in ext-display ")

	for _, param1 := range []struct {
		action    utils.ActionState
		dispIndex int
	}{
		{utils.ActionUnplug, 0},
		{utils.ActionPlugin, 2},
	} {
		// control ext-display1
		if err := utils.ControlFixture(ctx, s, utils.ExtDisp1Type, utils.ExtDisp1Index, param1.action, false); err != nil {
			return errors.Wrap(err, "failed to disconnect ext-display from docking station")
		}

		// control ext-display2
		if err := utils.ControlFixture(ctx, s, utils.ExtDisp2Type, utils.ExtDisp2Index, param1.action, false); err != nil {
			return errors.Wrap(err, "failed to disconnect ext-display2 from station")
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
					return errors.Wrapf(err, "failed to ensure %s window on display %d: ", param2.pkgName, param1.dispIndex)
				}

				return nil

			}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
				return err
			}

		}
	}

	return nil
}

func dock2PersistentSettingsStep8(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC) error {

	s.Log("Step 8 - Test mirror mode ")

	// get display info
	infos, err := display.GetInfo(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get display info")
	}

	// make sure internal display is primary
	if err := utils.EnsureDisplayIsPrimary(ctx, tconn, &infos[0]); err != nil {
		return errors.Wrap(err, "failed to ensure internal is primary")
	}

	// waiting for acceptable time
	testing.Sleep(ctx, 3*time.Second)

	s.Log("Enter mirror mode, then check display's mirror source id")

	// declare keyboard object
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to find keyboard")
	}
	defer kb.Close()

	// Press Ctrl+F4, enter or exit mirror mode
	enterMirror := kb.Accel(ctx, "Ctrl+F4")
	if err := enterMirror; err != nil {
		return errors.Wrap(err, "failed to enter mirror mode")
	}

	testing.Sleep(ctx, 3*time.Second)

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
			return errors.Wrap(err, "failed to get display infos in mirror mode")
		}

		// num of display should be just one in mirror mode
		if len(infos) > 1 {
			return errors.Errorf("failed to get right num of display, got %d, want 1", len(infos))
		}

		// check mirror source id
		for _, info := range infos {
			if intDispInfo.ID != info.MirroringSourceID {
				return errors.Wrap(err, "failed to check mirror source id")
			}
		}

		return nil

	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		return err
	}

	s.Log("Exit mirror mode, then check display attributes and windows is on internal display")

	testing.Sleep(ctx, 3*time.Second)

	// Press Ctrl+F4, exit mirror mode
	exitMirror := kb.Accel(ctx, "Ctrl+F4")
	if err := exitMirror; err != nil {
		return errors.Wrap(err, "failed to exit mirror mode")
	}

	testing.Sleep(ctx, 3*time.Second)

	// retry in 5s
	if err := testing.Poll(ctx, func(ctx context.Context) error {

		// get display info in mirror mode
		infos, err := display.GetInfo(ctx, tconn)
		if err != nil {
			return errors.Wrap(err, "failed to get display infos after exit mirror mode")
		}

		// num of display should be 3 after exiting mirror mode
		if len(infos) != 3 {
			return errors.Errorf("failed to get right num of display, got %d, want 3", len(infos))
		}

		// After exit (Mirror) mode Internal display show as (Primary) display and External display show as (Extended) display
		for _, info := range infos {
			if info.IsInternal == true { // internal
				if info.IsPrimary == false { // shall be primary
					return errors.Wrap(err, "failed to check internal is primary after exit mirror mode")
				}
			} else if info.IsInternal == false { // external
				if info.IsPrimary == true { // shall not be primary
					return errors.Wrap(err, "failed to check external is not primary after exit mirror mode")
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
				return errors.Wrapf(err, "failed to ensure window on display {pkgName:%s, seq:%d, ID:%s, Name:%s} ",
					param.packgeName, 0, dispInfo.ID, dispInfo.Name)
			}
		}

		return nil

	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		return err
	}

	return nil
}
