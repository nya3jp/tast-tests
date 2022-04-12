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
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/browser"
	"chromiumos/tast/local/chrome/uiauto/checked"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Dock2PersistentSettings,
		Desc: "Test persistent settings of window bound placement across displays in one use session via a Dock		",
		Contacts:     []string{"flin@google.com", "newmanliu19020@allion.corp-partner.google.com"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      10 * time.Minute,
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

	// Step 2 - Connect two external display to docking station.
	if err := dock2PersistentSettingsStep2(ctx, extDispID1, extDispID2); err != nil {
		s.Fatal("Failed to execute step2: ", err)
	}

	// Step 3 - Connect docking station to chromebook.
	if err := dock2PersistentSettingsStep3(ctx, tconn, dockingID); err != nil {
		s.Fatal("Failed to execute step3 : ", err)
	}

	// Step 4 - Open apps on each external display.
	if err := dock2PersistentSettingsStep4(ctx, tconn, cr, kb); err != nil {
		s.Fatal("Failed to execute step4: ", err)
	}

	// Step 5 - Unplug and plug-in ext-display, then check windows on expected display.
	if err := dock2PersistentSettingsStep5(ctx, tconn, extDispID1, extDispID2); err != nil {
		s.Fatal("Failed to execute step5: ", err)
	}

	// Step 6 - Test primary mode.
	if err := dock2PersistentSettingsStep6(ctx, tconn, kb); err != nil {
		s.Fatal("Failed to execute step6: ", err)
	}

	// Step 7 - Unplug and plug-in ext-display, then check windows on  expected display.
	if err := dock2PersistentSettingsStep7(ctx, tconn, extDispID1, extDispID2); err != nil {
		s.Fatal("Failed to execute step7: ", err)
	}

	// Step 8 - Test mirror mode.
	if err := dock2PersistentSettingsStep8(ctx, tconn); err != nil {
		s.Fatal("Failed to execute step8: ", err)
	}
}

func dock2PersistentSettingsStep2(ctx context.Context, extDispID1, extDispID2 string) error {
	testing.ContextLog(ctx, "Step 2 - Connect two external display to docking station")
	if err := utils.SwitchFixture(ctx, extDispID1, "on", "0"); err != nil {
		return errors.Wrap(err, "failed to plug in 1st external display")
	}
	if err := utils.SwitchFixture(ctx, extDispID2, "on", "0"); err != nil {
		return errors.Wrap(err, "failed to plug in 2nd external display")
	}
	return nil
}

func dock2PersistentSettingsStep3(ctx context.Context, tconn *chrome.TestConn, dockingID string) error {
	testing.ContextLog(ctx, "Step 3 - Connect docking station to Chromebook then check state")
	if err := utils.SwitchFixture(ctx, dockingID, "on", "0"); err != nil {
		return errors.Wrap(err, "failed to plug in docking station")
	}
	if err := utils.VerifyDisplayCount(ctx, tconn, 3); err != nil {
		return errors.Wrap(err, "failed to verify display count")
	}
	if err := utils.VerifyDisplayState(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to verify display state")
	}
	return nil
}

func dock2PersistentSettingsStep4(ctx context.Context, tconn *chrome.TestConn, cr *chrome.Chrome, kb *input.KeyboardEventWriter) error {
	testing.ContextLog(ctx, "Step 4 - Open apps on each external display")

	if _, err := filesapp.Launch(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to launch filesapp")
	}
	if _, err := browser.Launch(ctx, tconn, cr, "https://www.google.com"); err != nil {
		return errors.Wrap(err, "failed to launch browser")
	}

	// Switch two windows to 1st external display.
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

	// Leave files window on first external display.
	// Only switch browser window to second external display.
	ui := uiauto.New(tconn)
	infos, err := display.GetInfo(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get display info")
	}
	browser, err := ash.FindWindow(ctx, tconn, func(w *ash.Window) bool {
		return w.WindowType == ash.WindowTypeBrowser
	})
	if err != nil {
		return errors.Wrap(err, "failed to find browser window")
	}
	if err := browser.ActivateWindow(ctx, tconn); err != nil {
		return err
	}
	expectedRootWindow := nodewith.Name(infos[2].Name).Role(role.Window)
	currentWindow := nodewith.Name(browser.Title).Role(role.Window)
	expectedWindow := currentWindow.Ancestor(expectedRootWindow).First()
	if err := ui.Exists(expectedWindow)(ctx); err != nil {
		testing.ContextLog(ctx, "Expected window not found: ", err)
		testing.ContextLogf(ctx, "Switch window %q to %s", browser.Title, infos[2].Name)
		return uiauto.Combine("switch window to "+infos[2].Name,
			kb.AccelAction("Search+Alt+M"),
			ui.WithTimeout(10*time.Second).WaitUntilExists(expectedWindow),
		)(ctx)
	}
	return nil
}

func dock2PersistentSettingsStep5(ctx context.Context, tconn *chrome.TestConn, extDispID1, extDispID2 string) error {
	testing.ContextLog(ctx, "Step 5 - Unplug and plug-in external display")

	// Unplug ext-display then verify windows on internal display.
	if err := utils.SwitchFixture(ctx, extDispID2, "off", "0"); err != nil {
		return errors.Wrap(err, "failed to unplug 2nd external display")
	}
	if err := utils.VerifyDisplayCount(ctx, tconn, 2); err != nil {
		return errors.Wrap(err, "failed to verify display count")
	}
	if err := utils.SwitchFixture(ctx, extDispID1, "off", "0"); err != nil {
		return errors.Wrap(err, "failed to unplug 1st external display")
	}
	if err := utils.VerifyDisplayCount(ctx, tconn, 1); err != nil {
		return errors.Wrap(err, "failed to verify display count")
	}
	if err := utils.VerifyAllWindowsOnDisplay(ctx, tconn, false); err != nil {
		return errors.Wrap(err, "failed to verify windows on internal display")
	}

	// Plug-in ext-diplay then verify windows on external display.
	if err := utils.SwitchFixture(ctx, extDispID2, "on", "0"); err != nil {
		return errors.Wrap(err, "failed to plug in 2nd external display")
	}
	if err := utils.VerifyDisplayCount(ctx, tconn, 2); err != nil {
		return errors.Wrap(err, "failed to verify display count")
	}
	if err := utils.SwitchFixture(ctx, extDispID1, "on", "0"); err != nil {
		return errors.Wrap(err, "failed to plug in 1st external display")
	}
	if err := utils.VerifyDisplayCount(ctx, tconn, 3); err != nil {
		return errors.Wrap(err, "failed to verify display count")
	}

	// Verify files window on 1st external display.
	// Verfiy browser window on 2nd external display.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		infos, err := display.GetInfo(ctx, tconn)
		if err != nil {
			return errors.Wrap(err, "failed to get display info")
		}
		files, err := ash.FindWindow(ctx, tconn, func(w *ash.Window) bool {
			return w.WindowType == ash.WindowTypeExtension
		})
		if err != nil {
			return errors.Wrap(err, "failed to find files window")
		}
		if files.DisplayID != infos[1].ID {
			return errors.Errorf("files window is showing on the wrong display, got %d, want %d", files.DisplayID, infos[1].ID)
		}
		browser, err := ash.FindWindow(ctx, tconn, func(w *ash.Window) bool {
			return w.WindowType == ash.WindowTypeBrowser
		})
		if err != nil {
			return errors.Wrap(err, "failed to find browser window")
		}
		if browser.DisplayID != infos[2].ID {
			return errors.Errorf("browser window is showing on the wrong display, got %d, want %d", files.DisplayID, infos[2].ID)
		}
		return nil
	}, &testing.PollOptions{Timeout: 30 * time.Second, Interval: 2 * time.Second}); err != nil {
		return err
	}
	return nil
}

func dock2PersistentSettingsStep6(ctx context.Context, tconn *chrome.TestConn, kb *input.KeyboardEventWriter) error {
	testing.ContextLog(ctx, "Step 6 - Test primary mode")

	infos, err := display.GetInfo(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get display info")
	}

	if err := utils.EnsureDisplayPrimary(ctx, tconn, &infos[0]); err != nil {
		return errors.Wrap(err, "failed to ensure internal display is primary")
	}

	// Switch windows to internal display.
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
		return errors.Wrap(err, "failed to switch windows to internal display")
	}

	if err := utils.EnsureDisplayPrimary(ctx, tconn, &infos[1]); err != nil {
		return errors.Wrap(err, "failed to set 1st external display to be primary")
	}

	if err := utils.VerifyAllWindowsOnDisplay(ctx, tconn, true); err != nil {
		return errors.Wrap(err, "failed to verify windows on 1st external display")
	}
	return nil
}

func dock2PersistentSettingsStep7(ctx context.Context, tconn *chrome.TestConn, extDispID1, extDispID2 string) error {
	testing.ContextLog(ctx, "Step 7 - Unplug and plug-in external display")

	// Unplug ext-display then verify windows on internal display.
	if err := utils.SwitchFixture(ctx, extDispID2, "off", "0"); err != nil {
		return errors.Wrap(err, "failed to unplug 2nd external display")
	}
	if err := utils.VerifyDisplayCount(ctx, tconn, 2); err != nil {
		return errors.Wrap(err, "failed to verify display count")
	}
	if err := utils.SwitchFixture(ctx, extDispID1, "off", "0"); err != nil {
		return errors.Wrap(err, "failed to unplug 1st external display")
	}
	if err := utils.VerifyDisplayCount(ctx, tconn, 1); err != nil {
		return errors.Wrap(err, "failed to verify display count")
	}
	if err := utils.VerifyAllWindowsOnDisplay(ctx, tconn, false); err != nil {
		return errors.Wrap(err, "failed to verify windows on internal display")
	}

	// Plug-in ext-display then verify windows on external display.
	if err := utils.SwitchFixture(ctx, extDispID2, "on", "0"); err != nil {
		return errors.Wrap(err, "failed to plug in 2nd external display")
	}
	if err := utils.VerifyDisplayCount(ctx, tconn, 2); err != nil {
		return errors.Wrap(err, "failed to verify display count")
	}
	if err := utils.SwitchFixture(ctx, extDispID1, "on", "0"); err != nil {
		return errors.Wrap(err, "failed to plug in 1st external display")
	}
	if err := utils.VerifyDisplayCount(ctx, tconn, 3); err != nil {
		return errors.Wrap(err, "failed to verify display count")
	}
	if err := utils.VerifyAllWindowsOnDisplay(ctx, tconn, true); err != nil {
		return errors.Wrap(err, "failed to verify windows on 1st external display")
	}
	return nil
}

func dock2PersistentSettingsStep8(ctx context.Context, tconn *chrome.TestConn) error {
	testing.ContextLog(ctx, "Step 8 - Test mirror mode")

	intDispInfo, err := display.GetInternalInfo(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get internal display info")
	}

	if err := utils.EnsureDisplayPrimary(ctx, tconn, intDispInfo); err != nil {
		return errors.Wrap(err, "failed to ensure internal is primary")
	}

	testing.ContextLog(ctx, "Enter mirror mode, then verify display mirror source ID")

	if err := utils.SetMirrorDisplay(ctx, tconn, checked.True); err != nil {
		return errors.Wrap(err, "failed to enter mirror mode")
	}

	// Verify internal display MirroringSourceID and ID are the same.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		intDispInfo, err := display.GetInternalInfo(ctx, tconn)
		if err != nil {
			return errors.Wrap(err, "failed to get display infos in mirror mode")
		}
		if intDispInfo.ID != intDispInfo.MirroringSourceID {
			return errors.Errorf("unexcepted mirror source ID: got %d, want %d", intDispInfo.MirroringSourceID, intDispInfo.ID)
		}
		return nil
	}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
		return errors.Wrap(err, "failed to verify mirror source ID")
	}

	testing.ContextLog(ctx, "Exit mirror mode, then verify display state and windows are on internal display")

	if err := utils.SetMirrorDisplay(ctx, tconn, checked.False); err != nil {
		return errors.Wrap(err, "failed to exit mirror mode")
	}

	if err := utils.VerifyDisplayState(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to verfiy display state")
	}

	if err := utils.VerifyAllWindowsOnDisplay(ctx, tconn, false); err != nil {
		return errors.Wrap(err, "failed to verify all windows on internal display")
	}
	return nil
}
