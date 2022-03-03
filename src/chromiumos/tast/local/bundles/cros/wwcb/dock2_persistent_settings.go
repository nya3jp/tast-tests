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
	"chromiumos/tast/local/bundles/cros/wwcb/utils"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/uiauto/browser"
	"chromiumos/tast/local/chrome/uiauto/checked"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Dock2PersistentSettings,
		Desc: "Test persistent settings of window bound placement across displays in one use session via a Dock		",
		Contacts:     []string{"flin@google.com", "newmanliu19020@allion.corp-partner.google.com"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      4 * time.Minute,
		Vars:         []string{"WWCBIP", "DockingID", "1stExtDispID", "2ndExtDispID"},
		Pre:          chrome.LoggedIn(),
	})
}

func Dock2PersistentSettings(ctx context.Context, s *testing.State) {
	dockingID := s.RequiredVar("DockingID")
	extDispID1 := s.RequiredVar("1stExtDispID")
	extDispID2 := s.RequiredVar("2ndExtDispID")

	cr := s.PreValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to create a keyboard: ", err)
	}
	defer kb.Close()

	if err := utils.InitFixtures(ctx); err != nil {
		s.Fatal("Failed to initialize fixtures: ", err)
	}

	testing.ContextLog(ctx, "Step 1 - Boot up and sign in")

	// step 2 - connect two ext-display to docking
	if err := dock2PersistentSettingsStep2(ctx, extDispID1, extDispID2); err != nil {
		s.Fatal("Failed to execute step2: ", err)
	}
	// step 3 - connect docking to chromebook
	if err := dock2PersistentSettingsStep3(ctx, tconn, dockingID); err != nil {
		s.Fatal("Failed to execute step3 : ", err)
	}
	// step 4 - open apps on each external display
	if err := dock2PersistentSettingsStep4(ctx, tconn, cr, kb); err != nil {
		s.Fatal("Failed to execute step4: ", err)
	}
	// step 5 - unplug and re-plug in, check window bounds on which display
	if err := dock2PersistentSettingsStep5(ctx, tconn, extDispID1, extDispID2); err != nil {
		s.Fatal("Failed to execute step5: ", err)
	}
	// step 6 - test primary display
	if err := dock2PersistentSettingsStep6(ctx, tconn, kb); err != nil {
		s.Fatal("Failed to execute step6: ", err)
	}
	// step 7 - unplug and re-plug in, check window bounds on display
	if err := dock2PersistentSettingsStep7(ctx, tconn, extDispID1, extDispID2); err != nil {
		s.Fatal("Failed to execute step7: ", err)
	}
	// step 8 - test mirror mode
	if err := dock2PersistentSettingsStep8(ctx, tconn); err != nil {
		s.Fatal("Failed to execute step8: ", err)
	}
}

func dock2PersistentSettingsStep2(ctx context.Context, extDispID1, extDispID2 string) error {
	testing.ContextLog(ctx, "Step 2 - Connect two ext-display to docking station")
	if err := utils.SwitchFixture(ctx, extDispID1, "on", "0"); err != nil {
		return errors.Wrap(err, "failed to plug in 1st ext-display")
	}
	if err := utils.SwitchFixture(ctx, extDispID2, "on", "0"); err != nil {
		return errors.Wrap(err, "failed to plug in 2nd ext-display")
	}
	return nil
}

func dock2PersistentSettingsStep3(ctx context.Context, tconn *chrome.TestConn, dockingID string) error {
	testing.ContextLog(ctx, "Step 3 - Connect docking station to chromebook then check state")
	if err := utils.SwitchFixture(ctx, dockingID, "on", "0"); err != nil {
		return errors.Wrap(err, "failed to plug in dockint station")
	}
	// verify display properly
	if err := utils.VerifyDisplayProperly(ctx, tconn, 3); err != nil {
		return errors.Wrap(err, "failed to verify display properly")
	}
	if err := utils.VerifyDisplayState(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to verify display count")
	}
	return nil
}

func dock2PersistentSettingsStep4(ctx context.Context, tconn *chrome.TestConn, cr *chrome.Chrome, kb *input.KeyboardEventWriter) error {
	testing.ContextLog(ctx, "Step 4 - Open apps on internal & external display")
	// launch apps
	if _, err := filesapp.Launch(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to launch filesapp")
	}
	if _, err := browser.Launch(ctx, tconn, cr, "https://www.google.com"); err != nil {
		return errors.Wrap(err, "failed to launch browser")
	}
	// switch all windows to 1st external display
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		ws, err := ash.GetAllWindows(ctx, tconn)
		if err != nil {
			return err
		}
		for _, w := range ws {
			if err := w.ActivateWindow(ctx, tconn); err != nil {
				return err
			}
			if err := utils.SwitchWindowToDisplay(ctx, tconn, kb, true)(ctx); err != nil {
				return err
			}
		}
		return nil
	}, &testing.PollOptions{Timeout: 30 * time.Second, Interval: 2 * time.Second}); err != nil {
		return errors.Wrap(err, "failed to switch windows to external display")
	}
	return nil
}

func dock2PersistentSettingsStep5(ctx context.Context, tconn *chrome.TestConn, extDispID1, extDispID2 string) error {
	testing.ContextLog(ctx, "Step 5 - Unplug and re-plug in ext-display")
	// unplug then verify windows on internal display
	if err := utils.SwitchFixture(ctx, extDispID1, "off", "0"); err != nil {
		return errors.Wrap(err, "failed to unplug 1st ext-display")
	}
	if err := utils.SwitchFixture(ctx, extDispID2, "off", "0"); err != nil {
		return errors.Wrap(err, "failed to unplug 2nd ext-display")
	}
	if err := utils.VerifyAllWindowsOnDisplay(ctx, tconn, false); err != nil {
		return errors.Wrap(err, "failed to verify windows on internal display")
	}

	// re-plug-in then verify windows on external display
	if err := utils.SwitchFixture(ctx, extDispID1, "on", "0"); err != nil {
		return errors.Wrap(err, "failed to plug in 1st ext-display")
	}
	if err := utils.SwitchFixture(ctx, extDispID2, "on", "0"); err != nil {
		return errors.Wrap(err, "failed to plug in 2nd ext-display")
	}
	if err := utils.VerifyDisplayProperly(ctx, tconn, 3); err != nil {
		return errors.Wrap(err, "failed to verify display properly")
	}
	if err := utils.VerifyAllWindowsOnDisplay(ctx, tconn, true); err != nil {
		return errors.Wrap(err, "failed to verify windows on external display")
	}
	return nil
}
func dock2PersistentSettingsStep6(ctx context.Context, tconn *chrome.TestConn, kb *input.KeyboardEventWriter) error {
	testing.ContextLog(ctx, "Step 6 - Test primary mode")
	// get display infos
	infos, err := display.GetInfo(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get display info")
	}
	// ensure internal display is primary
	if err := utils.EnsureDisplayIsPrimary(ctx, tconn, &infos[0]); err != nil {
		return errors.Wrap(err, "failed to set internal display is primary")
	}
	// move windows to internal display
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		ws, err := ash.GetAllWindows(ctx, tconn)
		if err != nil {
			return err
		}
		for _, w := range ws {
			if err := w.ActivateWindow(ctx, tconn); err != nil {
				return err
			}
			if err := utils.SwitchWindowToDisplay(ctx, tconn, kb, false)(ctx); err != nil {
				return err
			}
		}
		return nil
	}, &testing.PollOptions{Timeout: 30 * time.Second, Interval: 2 * time.Second}); err != nil {
		return err
	}

	// check displays is enough
	if len(infos) != 3 {
		return errors.Errorf("failed to get correct number of display; got %d, want 3", len(infos))
	}
	// set 1st external display to be primary
	if err := utils.EnsureDisplayIsPrimary(ctx, tconn, &infos[1]); err != nil {
		return errors.Wrap(err, "failed to set 1st external display to be primary")
	}
	// verfiy display state
	intDispInfo, err := display.GetInternalInfo(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get internal display info")
	}
	if intDispInfo.IsPrimary {
		return errors.New("internal display should not be primary")
	}
	// verfiy windows on 1st ext-display
	if err := utils.VerifyAllWindowsOnDisplay(ctx, tconn, true); err != nil {
		return err
	}
	return nil
}

func dock2PersistentSettingsStep7(ctx context.Context, tconn *chrome.TestConn, extDispID1, extDispID2 string) error {
	testing.ContextLog(ctx, "Step 7 - Unplug and re-plug in ext-display")
	// unplug then verify windows on internal display
	if err := utils.SwitchFixture(ctx, extDispID1, "off", "0"); err != nil {
		return errors.Wrap(err, "failed to unplug 1st ext-display")
	}
	if err := utils.SwitchFixture(ctx, extDispID2, "off", "0"); err != nil {
		return errors.Wrap(err, "failed to unplug 2nd ext-display")
	}
	if err := utils.VerifyAllWindowsOnDisplay(ctx, tconn, false); err != nil {
		return errors.Wrap(err, "failed to verify windows on internal display")
	}

	// re-plug-in then verify windows on external display
	if err := utils.SwitchFixture(ctx, extDispID1, "on", "0"); err != nil {
		return errors.Wrap(err, "failed to plug in 1st ext-display")
	}
	if err := utils.SwitchFixture(ctx, extDispID2, "on", "0"); err != nil {
		return errors.Wrap(err, "failed to plug in 2nd ext-display")
	}
	if err := utils.VerifyDisplayProperly(ctx, tconn, 3); err != nil {
		return err
	}
	if err := utils.VerifyAllWindowsOnDisplay(ctx, tconn, true); err != nil {
		return errors.Wrap(err, "failed to verify windows on external display")
	}
	return nil
}

func dock2PersistentSettingsStep8(ctx context.Context, tconn *chrome.TestConn) error {
	testing.ContextLog(ctx, "Step 8 - Test mirror mode")
	// make sure internal display is primary
	intDispInfo, err := display.GetInternalInfo(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get internal display info")
	}
	if err := utils.EnsureDisplayIsPrimary(ctx, tconn, intDispInfo); err != nil {
		return errors.Wrap(err, "failed to ensure internal is primary")
	}

	testing.ContextLog(ctx, "Enter mirror mode, then check display's mirror source id")
	// enter mirror mode
	if err := utils.SetMirrorDisplay(ctx, tconn, checked.True); err != nil {
		return errors.Wrap(err, "failed to enter mirror mode")
	}
	// verify number of display
	if err := utils.VerifyDisplayProperly(ctx, tconn, 1); err != nil {
		return errors.Wrap(err, "failed to verify display properly")
	}
	// verify display attributes
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		infos, err := display.GetInfo(ctx, tconn)
		if err != nil {
			return errors.Wrap(err, "failed to get display infos in mirror mode")
		}
		// check mirror source id
		for _, info := range infos {
			if intDispInfo.ID != info.MirroringSourceID {
				return errors.New("failed to check mirror source id")
			}
		}
		return nil
	}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
		return errors.Wrap(err, "failed to verify display attributes")
	}

	testing.ContextLog(ctx, "Exit mirror mode, then check display attributes and windows is on internal display")
	// unset mirror mode
	if err := utils.SetMirrorDisplay(ctx, tconn, checked.False); err != nil {
		return errors.Wrap(err, "failed to exit mirror mode")
	}
	// verify display state
	if err := utils.VerifyDisplayState(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to verfiy display state")
	}
	// verify windows on internal display
	if err := utils.VerifyAllWindowsOnDisplay(ctx, tconn, false); err != nil {
		return errors.Wrap(err, "failed to verify all windows on internal display")
	}
	return nil
}
