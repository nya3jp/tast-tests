// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

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
		Func:         Dock1PersistentSettings,
		Desc:         "Test persistent settings of window bound placement across displays in one use session via a Dock",
		Contacts:     []string{"allion-sw@allion.com"},
		SoftwareDeps: []string{"arc", "chrome"},
		Timeout:      4 * time.Minute,
		Fixture:      "arcBooted",
		Vars:         []string{"WWCBIP", "Docking", "1stExtDisp", "ABC"},
	})
}

func Dock1PersistentSettings(ctx context.Context, s *testing.State) {
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

	testing.ContextLog(ctx, "Step 1 - Boot-up and Sign-In to the device")

	// step 2 - plug ext-display into docking station
	if err := dock1PersistentSettingsStep2(ctx, s); err != nil {
		s.Fatal("Failed to execute step2: ", err)
	}

	// step 3 - plug docking station into chromebook
	if err := dock1PersistentSettingsStep3(ctx, s, tconn); err != nil {
		s.Fatal("Failed to execute step3: ", err)
	}

	// step 4 - open two apps on external display
	if err := dock1PersistentSettingsStep4(ctx, tconn, a); err != nil {
		s.Fatal("Failed to execute step4: ", err)
	}

	// step 5 - unplug and re-plug in, check window bounds on certain display
	if err := dock1PersistentSettingsStep5(ctx, s, tconn); err != nil {
		s.Fatal("Failed to execute step5: ", err)
	}

	// step 6 - switch to primary mode, check windows on certain display
	if err := dock1PersistentSettingsStep6(ctx, tconn, a); err != nil {
		s.Fatal("Failed to execute Step6: ", err)
	}

	// step 7 - unplug and re-plug in, check window bounds on certain display
	if err := dock1PersistentSettingsStep7(ctx, s, tconn); err != nil {
		s.Fatal("Failed to execute step7: ", err)
	}

	// step 8 - test mirror mode
	if err := dock1PersistentSettingsStep8(ctx, tconn, a); err != nil {
		s.Fatal("Failed to execute step8: ", err)
	}

}

func dock1PersistentSettingsStep2(ctx context.Context, s *testing.State) error {
	testing.ContextLog(ctx, "Step 2 - Plug ext-display into docking station")
	if err := utils.SwitchFixture(ctx, s.RequiredVar("1stExtDisp"), "on", "0"); err != nil {
		return errors.Wrap(err, "failed to plug in ext-display")
	}
	return nil
}

func dock1PersistentSettingsStep3(ctx context.Context, s *testing.State, tconn *chrome.TestConn) error {
	testing.ContextLog(ctx, "Step 3 - Plug docking station into Chromebook")
	if err := utils.SwitchFixture(ctx, s.RequiredVar("Docking"), "on", "0"); err != nil {
		return errors.Wrap(err, "failed to plug in docking station")
	}
	// verify display properly
	if err := utils.VerifyDisplayProperly(ctx, tconn, 2); err != nil {
		return errors.Wrap(err, "failed to verify display properly")
	}
	// verify display state
	if err := utils.VerifyDisplayState(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to verfiy display state")
	}
	return nil
}

func dock1PersistentSettingsStep4(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC) error {
	testing.ContextLog(ctx, "Step 4 - Open two chrome windows")

	// install apk for testing
	if err := a.Install(ctx, arc.APKPath(utils.TestappApk)); err != nil {
		return errors.Wrap(err, "failed to install app")
	}

	// start two activity on external display - 1
	for _, param := range []struct {
		pkgName string
		actName string
	}{
		{utils.SettingsPkg, utils.SettingsAct},
		{utils.TestappPkg, utils.TestappAct},
	} {
		testing.ContextLogf(ctx, "Open %s on display 1", param.pkgName)

		if err := utils.StartActivityOnDisplay(ctx, a, tconn, param.pkgName, param.actName, 1); err != nil {
			return errors.Wrap(err, "failed to start activity on display")
		}

		// set window as normal state, in case the window won't jump back to external
		if state, err := ash.SetARCAppWindowState(ctx, tconn, param.pkgName, ash.WMEventNormal); err != nil {
			return errors.Wrap(err, "failed to set window state")
		} else if state != ash.WindowStateNormal {
			return errors.Errorf("unexpected window state; got %s, want %s", state, ash.WindowStateNormal)
		}

		if err := utils.EnsureSetWindowState(ctx, tconn, param.pkgName, ash.WindowStateNormal); err != nil {
			return errors.Wrap(err, "failed to ensure window state")
		}

	}
	return nil
}

func dock1PersistentSettingsStep5(ctx context.Context, s *testing.State, tconn *chrome.TestConn) error {
	testing.ContextLog(ctx, "Step 5 - Unplug and re-plug-in external display")

	for _, param := range []struct {
		action    string
		dispIndex int
		dispNum   int
	}{
		{"off", 0, 1}, // internal
		{"on", 1, 2},  // external
	} {
		// control ext-display to plug in or unplug
		if err := utils.SwitchFixture(ctx, s.RequiredVar("1stExtDisp"), param.action, "0"); err != nil {
			return errors.Wrapf(err, "failed to %s ext-display ", param.action)
		}

		for _, pkgName := range []string{
			utils.SettingsPkg,
			utils.TestappPkg,
		} {
			testing.ContextLogf(ctx, "Checking %s window is on %d display", pkgName, param.dispIndex)

			// retry in 30s
			if err := testing.Poll(ctx, func(ctx context.Context) error {
				infos, err := display.GetInfo(ctx, tconn)
				if err != nil {
					return err
				}
				// check display number first
				// in case this issue->Panic: runtime error: index out of range [1] with length 1
				if len(infos) != param.dispNum {
					return errors.New("number of display is not correct")
				}
				if err := utils.EnsureWindowOnDisplay(ctx, tconn, pkgName, infos[param.dispIndex].ID); err != nil {
					return errors.Wrapf(err, "failed to ensure %s window on display %d", pkgName, param.dispIndex)
				}
				return nil
			}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
				return err
			}
		}
	}
	return nil
}

func dock1PersistentSettingsStep6(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC) error {
	testing.ContextLog(ctx, "Step 6 - Test primary mode")

	// get display info
	infos, err := display.GetInfo(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get display info")
	}

	// ensure internal display is primary
	if err := utils.EnsureDisplayIsPrimary(ctx, tconn, &infos[0]); err != nil {
		return errors.Wrap(err, "failed to ensure internal is primary")
	}

	// reopen window on internal display
	if err := utils.ReopenAllWindowsOnInternal(ctx, tconn, a); err != nil {
		return errors.Wrap(err, "failed to reopen window on internal display")
	}

	// need to delay, in case execution failed
	// if don't sleep, would cause fail on the next move
	// after setting external display be primary
	// the window should be on the primary display, which mean is on external display
	// but, in fact, screen would refresh then window still on internal display, this is unexpected behavior
	testing.Sleep(ctx, 5*time.Second)

	testing.ContextLog(ctx, "Change ext-display as primary display")

	// set first external display to be primary
	if err := utils.EnsureDisplayIsPrimary(ctx, tconn, &infos[1]); err != nil {
		return errors.Wrap(err, "failed to set ext-display 1 to be primary")
	}

	// ensure two apps on external display1
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		for _, pkgName := range []string{
			utils.SettingsPkg,
			utils.TestappPkg,
		} {
			if err := utils.EnsureWindowOnDisplay(ctx, tconn, pkgName, infos[1].ID); err != nil {
				return errors.Wrapf(err, "failed to ensure [%s] window on ext-display ", pkgName)
			}
		}
		return nil
	}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
		return err
	}
	return nil
}

func dock1PersistentSettingsStep7(ctx context.Context, s *testing.State, tconn *chrome.TestConn) error {
	testing.ContextLog(ctx, "Step 7 - Unplug and re-plug external display")

	for _, param := range []struct {
		action    string
		dispIndex int
		dispNum   int
	}{
		{"off", 0, 1}, // internal
		{"on", 1, 2},  // external
	} {
		// control ext-display to plug in or unplug
		if err := utils.SwitchFixture(ctx, s.RequiredVar("1stExtDisp"), param.action, "0"); err != nil {
			return errors.Wrapf(err, "failed to %s ext-display", param.action)
		}

		for _, pkgName := range []string{
			utils.SettingsPkg,
			utils.TestappPkg,
		} {
			testing.ContextLogf(ctx, "Checking %s window is on %d display", pkgName, param.dispIndex)

			// retry in 30s
			if err := testing.Poll(ctx, func(ctx context.Context) error {
				infos, err := display.GetInfo(ctx, tconn)
				if err != nil {
					return err
				}
				if len(infos) != param.dispNum {
					return errors.New("number of display is not correct")
				}
				if err := utils.EnsureWindowOnDisplay(ctx, tconn, pkgName, infos[param.dispIndex].ID); err != nil {
					return errors.Wrapf(err, "failed to ensure %s window on display %d", pkgName, param.dispIndex)
				}
				return nil
			}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
				return err
			}
		}
	}
	return nil
}

func dock1PersistentSettingsStep8(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC) error {
	testing.ContextLog(ctx, "Step 8 - Test mirror mode")

	// get display info
	infos, err := display.GetInfo(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get display info")
	}

	// make sure internal display is primary
	if err := utils.EnsureDisplayIsPrimary(ctx, tconn, &infos[0]); err != nil {
		return errors.Wrap(err, "failed to ensure internal is primary")
	}

	testing.ContextLog(ctx, "Enter mirror mode, then check display's mirror source id")

	// declare keyboard object
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to find keyboard")
	}
	defer kb.Close()

	// send "Ctrl+F4" to enter mirror mode
	if err := kb.Accel(ctx, "Ctrl+F4"); err != nil {
		return errors.Wrap(err, "failed to enter mirror mode")
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
			return errors.Wrap(err, "failed to get display infos in mirror mode")
		}

		// num of display should be just one in mirror mode
		if len(infos) > 1 {
			return errors.Errorf("failed to get right num of display; got %d, want 1", len(infos))
		}

		// check mirror source id
		for _, info := range infos {
			if intDispInfo.ID != info.MirroringSourceID {
				return errors.Wrap(err, "failed to check mirror source id")
			}
		}
		return nil
	}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
		return err
	}

	testing.ContextLog(ctx, "Exit mirror mode, then check display attributes and windows is on internal display")

	// send "Ctrl+F4" to exit mirror mode
	if err := kb.Accel(ctx, "Ctrl+F4"); err != nil {
		return errors.Wrap(err, "failed to exit mirror mode")
	}

	// retry checking in 30s
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		// get display info in mirror mode
		infos, err := display.GetInfo(ctx, tconn)
		if err != nil {
			return errors.Wrap(err, "failed to get display infos after exit mirror mode")
		}

		// After exit (Mirror) mode Internal display show as (Primary) display and External display show as (Extended) display
		for _, info := range infos {
			if info.IsInternal { // internal
				if !info.IsPrimary { // shall be primary
					return errors.Wrap(err, "failed to check internal is primary after exit mirror mode")
				}
			} else if !info.IsInternal { // external
				if info.IsPrimary { // shall not be primary
					return errors.Wrap(err, "failed to check external is not primary after exit mirror mode")
				}
			}
		}

		// ensure 2 apps on internal display
		if err := utils.EnsureWindowOnDisplay(ctx, tconn, utils.SettingsPkg, infos[0].ID); err != nil {
			return errors.Wrap(err, "failed to ensure setting window on internal display")
		}

		if err := utils.EnsureWindowOnDisplay(ctx, tconn, utils.TestappPkg, infos[0].ID); err != nil {
			return errors.Wrap(err, "failed to ensure testapp window on internal display")
		}
		return nil
	}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
		return err
	}
	return nil
}
