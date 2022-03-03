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
// 2)  Connect external display to (Docking station)
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
// ** Plug-In: (two Chrome App windows) will bounds back to external display

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

// Package wwcb contains local Tast tests that work with Chromebook
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
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Dock1PersistentSettings,
		Desc:         "Test persistent settings of window bound placement across displays in one use session via a Dock",
		Contacts:     []string{"flin@google.com", "newmanliu19020@allion.corp-partner.google.com"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Timeout:      4 * time.Minute,
		Vars:         []string{"WWCBIP", "DockingID", "1stExtDispID"},
		Pre:          chrome.LoggedIn(),
	})
}

func Dock1PersistentSettings(ctx context.Context, s *testing.State) {
	extDispID := s.RequiredVar("1stExtDispID")
	dockingID := s.RequiredVar("DockingID")

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

	cleanup, err := utils.InitFixtures(ctx)
	if err != nil {
		s.Fatal("Failed to initialize fixtures: ", err)
	}
	defer cleanup(ctx)

	testing.ContextLog(ctx, "Step 1 - Boot-up and sign in to the device")

	// Step 2 - Plug external display into docking station.
	if err := dock1PersistentSettingsStep2(ctx, extDispID); err != nil {
		s.Fatal("Failed to execute step 2: ", err)
	}

	// Step 3 - Plug docking station into Chromebook.
	if err := dock1PersistentSettingsStep3(ctx, tconn, dockingID); err != nil {
		s.Fatal("Failed to execute step 3: ", err)
	}

	// Step 4 - Open two Chrome windows on external display.
	if err := dock1PersistentSettingsStep4(ctx, tconn, cr, kb); err != nil {
		s.Fatal("Failed to execute step 4: ", err)
	}

	// Step 5 - Unplug and re-plug in, check windows on expected display.
	if err := dock1PersistentSettingsStep5(ctx, tconn, extDispID); err != nil {
		s.Fatal("Failed to execute step 5: ", err)
	}

	// Step 6 - Test primary mode, check windows on expected display.
	if err := dock1PersistentSettingsStep6(ctx, tconn, kb); err != nil {
		s.Fatal("Failed to execute Step 6: ", err)
	}

	// Step 7 - Unplug and re-plug in, check windows on expected display.
	if err := dock1PersistentSettingsStep7(ctx, tconn, extDispID); err != nil {
		s.Fatal("Failed to execute step 7: ", err)
	}

	// Step 8 - Test mirror mode, check display attributes and windows on display.
	if err := dock1PersistentSettingsStep8(ctx, tconn, kb); err != nil {
		s.Fatal("Failed to execute step 8: ", err)
	}
}

func dock1PersistentSettingsStep2(ctx context.Context, extDispID string) error {
	testing.ContextLog(ctx, "Step 2 - Plug external display into docking station")
	if err := utils.SwitchFixture(ctx, extDispID, "on", "0"); err != nil {
		return errors.Wrap(err, "failed to plug in external display")
	}
	return nil
}

func dock1PersistentSettingsStep3(ctx context.Context, tconn *chrome.TestConn, dockingID string) error {
	testing.ContextLog(ctx, "Step 3 - Plug docking station into Chromebook")
	if err := utils.SwitchFixture(ctx, dockingID, "on", "0"); err != nil {
		return errors.Wrap(err, "failed to plug in docking station")
	}
	if err := utils.VerifyExternalDisplay(ctx, tconn, true); err != nil {
		return errors.Wrap(err, "failed to verify a connected external display")
	}
	if err := utils.VerifyDisplayState(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to verfiy display state")
	}
	return nil
}

func dock1PersistentSettingsStep4(ctx context.Context, tconn *chrome.TestConn, cr *chrome.Chrome, kb *input.KeyboardEventWriter) error {
	testing.ContextLog(ctx, "Step 4 - Open two Chrome windows on external display")

	if _, err := filesapp.Launch(ctx, tconn); err != nil {
		return err
	}

	if _, err := browser.Launch(ctx, tconn, cr, "https://www.google.com"); err != nil {
		return err
	}

	// Switch window to external display.
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

func dock1PersistentSettingsStep5(ctx context.Context, tconn *chrome.TestConn, extDispID string) error {
	testing.ContextLog(ctx, "Step 5 - Unplug and plug in the external display again")

	// Unplug external display then verify windows on internal display.
	if err := utils.SwitchFixture(ctx, extDispID, "off", "0"); err != nil {
		return errors.Wrap(err, "failed to unplug external display")
	}
	if err := utils.VerifyAllWindowsOnDisplay(ctx, tconn, false); err != nil {
		return errors.Wrap(err, "failed to verify all windows on internal display")
	}

	// Plug in external display then verify windows on external display.
	if err := utils.SwitchFixture(ctx, extDispID, "on", "0"); err != nil {
		return errors.Wrap(err, "failed to plug in external display")
	}
	if err := utils.VerifyAllWindowsOnDisplay(ctx, tconn, true); err != nil {
		return errors.Wrap(err, "failed to verify all windows on external display")
	}
	return nil
}

func dock1PersistentSettingsStep6(ctx context.Context, tconn *chrome.TestConn, kb *input.KeyboardEventWriter) error {
	testing.ContextLog(ctx, "Step 6 - Test primary mode")

	infos, err := utils.GetInternalAndExternalDisplays(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get internal and external display")
	}

	if err := utils.EnsureDisplayPrimary(ctx, tconn, &infos.Internal); err != nil {
		return errors.Wrap(err, "failed to ensure internal is primary")
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

	testing.ContextLog(ctx, "Change external display as primary display, then verify windows on external display")

	if err := utils.EnsureDisplayPrimary(ctx, tconn, &infos.External); err != nil {
		return errors.Wrap(err, "failed to ensure external display is primary")
	}

	if err := utils.VerifyAllWindowsOnDisplay(ctx, tconn, true); err != nil {
		return errors.Wrap(err, "failed to verify all windows on external display")
	}
	return nil
}

func dock1PersistentSettingsStep7(ctx context.Context, tconn *chrome.TestConn, extDispID string) error {
	testing.ContextLog(ctx, "Step 7 - Unplug and plug in the external display again")

	// Unplug external display then verify windows on internal display.
	if err := utils.SwitchFixture(ctx, extDispID, "off", "0"); err != nil {
		return errors.Wrap(err, "failed to unplug external display")
	}
	if err := utils.VerifyAllWindowsOnDisplay(ctx, tconn, false); err != nil {
		return errors.Wrap(err, "failed to verify all windows on internal display")
	}

	// Plug in external display then verify windows on external display.
	if err := utils.SwitchFixture(ctx, extDispID, "on", "0"); err != nil {
		return errors.Wrap(err, "failed to plug in external display")
	}
	if err := utils.VerifyAllWindowsOnDisplay(ctx, tconn, true); err != nil {
		return errors.Wrap(err, "failed to verify all windows on external display")
	}
	return nil
}

func dock1PersistentSettingsStep8(ctx context.Context, tconn *chrome.TestConn, kb *input.KeyboardEventWriter) error {
	testing.ContextLog(ctx, "Step 8 - Test mirror mode")

	intDispInfo, err := display.GetInternalInfo(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get internal display info")
	}

	if err := utils.EnsureDisplayPrimary(ctx, tconn, intDispInfo); err != nil {
		return errors.Wrap(err, "failed to ensure internal display is primary")
	}

	testing.ContextLog(ctx, "Enter mirror mode, then verify each display mirror source ID")

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
			return errors.Errorf("unexcepted mirror source ID: got %s, want %s", intDispInfo.MirroringSourceID, intDispInfo.ID)
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
