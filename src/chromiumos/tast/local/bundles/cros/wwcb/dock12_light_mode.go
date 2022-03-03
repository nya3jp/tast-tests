// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

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

// Package wwcb contains local Tast tests that work with chromebook
package wwcb

import (
	"context"
	"path/filepath"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/wwcb/utils"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/power/setup"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Dock12LightMode,
		Desc:         "Sunset/Sunrise Light mode",
		Contacts:     []string{"flin@google.com", "newmanliu19020@allion.corp-partner.google.com"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      4 * time.Minute,
		Vars:         []string{"WWCBIP", "DockingID", "1stExtDispID"},
		Pre:          chrome.LoggedIn(),
	})
}

func Dock12LightMode(ctx context.Context, s *testing.State) {
	dockingID := s.RequiredVar("DockingID")
	extDispID1 := s.RequiredVar("1stExtDispID")

	cr := s.PreValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to open connection: ", err)
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

	testing.ContextLog(ctx, "Step 1 - Boot-up and Sign-In to the device")

	// Step 2 - Connect ext-display to docking station.
	if err := dock12LightModeStep2(ctx, extDispID1); err != nil {
		s.Fatal("Failed to execute step 2: ", err)
	}

	// Step 3 - Connect docking station to Chromebook.
	if err := dock12LightModeStep3(ctx, tconn, dockingID); err != nil {
		s.Fatal("Failed to execute step 3: ", err)
	}

	// Step 4 - Open settings then search night light.
	if err := dock12LightModeStep4(ctx, tconn, kb); err != nil {
		s.Fatal("Failed to execute step 4: ", err)
	}

	// Step 5 - Enable night light and adjust the color temperature.
	if err := dock12LightModeStep5(ctx, tconn, kb, extDispID1); err != nil {
		s.Fatal("Failed to execute step 5: ", err)
	}

	testing.Sleep(ctx, time.Second*10)
	uiTree := filepath.Join(s.OutDir(), "ui_tree.txt")
	if err := uiauto.LogRootDebugInfo(ctx, tconn, uiTree); err != nil {
		testing.ContextLog(ctx, "Failed to dump: ", err)
	}
}

func dock12LightModeStep2(ctx context.Context, extDispID1 string) error {
	testing.ContextLog(ctx, "Step 2 - Plug in external monitor to the docking station")
	if err := utils.SwitchFixture(ctx, extDispID1, "on", "0"); err != nil {
		return errors.Wrap(err, "failed to plug in external display")
	}
	return nil
}

func dock12LightModeStep3(ctx context.Context, tconn *chrome.TestConn, dockingID string) error {
	testing.ContextLog(ctx, "Step 3 - Connect docking station to the chromebook")
	if err := utils.SwitchFixture(ctx, dockingID, "on", "0"); err != nil {
		return errors.Wrap(err, "failed to plug in docking station")
	}
	if err := utils.VerifyExternalDisplay(ctx, tconn, true); err != nil {
		return errors.Wrap(err, "failed to verify ext-display is connected")
	}
	return nil
}

func dock12LightModeStep4(ctx context.Context, tconn *chrome.TestConn, kb *input.KeyboardEventWriter) error {
	testing.ContextLog(ctx, "Step 4 - Open settings then search night light")

	const (
		keyword = "Night light"
	)

	settings, err := ossettings.Launch(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "opening settings page failed")
	}

	ui := uiauto.New(tconn)

	// Verify that cursor is focus in the search field.
	if err := ui.WaitUntilExists(ossettings.SearchBoxFinder.Focused())(ctx); err != nil {
		return err
	}

	testing.ContextLogf(ctx, "Search for %q", keyword)

	infos, mismatched, err := settings.SearchWithKeyword(ctx, kb, keyword)
	if err != nil {
		return err
	}

	// Verify search results count.
	if len(infos) == 0 {
		return errors.New("no results found")
	} else if len(infos) > 5 || len(infos) < 1 {
		// The results should show a minimum of 1 or maximum of 5 results.
		return errors.Errorf("unexpected result count, want: [1,5], got: %d", len(infos))
	}

	// Verify mismatch.
	if mismatched {
		return errors.Errorf("unexpected search result, got: [mismatch: %t]", mismatched)
	}

	// Do the navigation.
	if err := kb.TypeKey(ctx, input.KEY_ENTER); err != nil {
		return errors.Wrap(err, "failed to type enter")
	}

	// Switch window to external display.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := utils.SwitchWindowToDisplay(ctx, tconn, kb, true)(ctx); err != nil {
			return err
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second, Interval: 10 * time.Millisecond}); err != nil {
		return errors.Wrap(err, "failed to switch window to external display")
	}

	// Gets info of the browser window, assuming it is the active window.
	ws, err := ash.GetAllWindows(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to obtain the window list")
	}
	browserWinID := ws[0].ID

	// Turn the window into normal state.
	if err := ash.SetWindowStateAndWait(ctx, tconn, browserWinID, ash.WindowStateFullscreen); err != nil {
		return errors.Wrap(err, "failed to set the window state to normal")
	}
	return nil
}

func dock12LightModeStep5(ctx context.Context, tconn *chrome.TestConn, kb *input.KeyboardEventWriter, extDispID string) error {
	testing.ContextLog(ctx, "Step 5 - Enable night light and adjust the color temperature")

	originalPicture, err := utils.ScreenCapture(ctx, extDispID)
	if err != nil {
		return errors.Wrapf(err, "failed to take a picture of %s", extDispID)
	}

	originalColor, err := utils.GetPiColorHotValue(ctx, originalPicture)
	if err != nil {
		return errors.Wrap(err, "failed to get hot value")
	}
	testing.ContextLog(ctx, "original color is "+originalColor)

	CleanupCallback, err := setup.SetNightLightEnabled(ctx, tconn, true)
	if err != nil {
		return errors.Wrap(err, "failed to enable night light")
	}
	defer CleanupCallback(ctx)

	nlPicture, err := utils.ScreenCapture(ctx, extDispID)
	if err != nil {
		return errors.Wrapf(err, "failed to take a picture of %s", extDispID)
	}

	nlColor, err := utils.GetPiColorHotValue(ctx, nlPicture)
	if err != nil {
		return errors.Wrap(err, "failed to get hot value")
	}
	testing.ContextLog(ctx, "night light enabled coloreis "+nlColor)

	ui := uiauto.New(tconn)

	contaitner := nodewith.Role(role.GenericContainer).Name("Color temperature")
	Slider := nodewith.Role(role.Slider).Ancestor(contaitner)
	if err := uiauto.Combine("focus on slider",
		ui.WaitUntilExists(Slider),
		ui.FocusAndWait(Slider))(ctx); err != nil {
		return err
	}

	// Increase color temperature.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := kb.TypeKey(ctx, input.KEY_UP); err != nil {
			return errors.Wrap(err, "failed to type keyup")
		}

		sliderInfo, err := ui.Info(ctx, Slider)
		if err != nil {
			return err
		}

		if sliderInfo.Value != "100" {
			return errors.New("unable to increase silder value to 100")
		}
		return nil
	}, &testing.PollOptions{Timeout: 15 * time.Second, Interval: 10 * time.Millisecond}); err != nil {
		return errors.Wrap(err, "failed to increase color temperature")
	}

	warmerPicture, err := utils.ScreenCapture(ctx, extDispID)
	if err != nil {
		return errors.Wrapf(err, "failed to take a picture of %s", extDispID)
	}

	warmerColor, err := utils.GetPiColorHotValue(ctx, warmerPicture)
	if err != nil {
		return errors.Wrap(err, "failed to get hot value")
	}
	testing.ContextLog(ctx, "warmer color is "+warmerColor)

	// Decrease color temperature.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := kb.TypeKey(ctx, input.KEY_DOWN); err != nil {
			return errors.Wrap(err, "failed to key down")
		}

		sliderInfo, err := ui.Info(ctx, Slider)
		if err != nil {
			return err
		}

		if sliderInfo.Value != "0" {
			return errors.New("unable to decrease slider value to 0")
		}
		return nil
	}, &testing.PollOptions{Timeout: 15 * time.Second, Interval: 10 * time.Millisecond}); err != nil {
		return errors.Wrap(err, "failed to decrease color temperature")
	}

	coolerPicture, err := utils.ScreenCapture(ctx, extDispID)
	if err != nil {
		return errors.Wrapf(err, "failed to take a picture of %s", extDispID)
	}

	coolerColor, err := utils.GetPiColorHotValue(ctx, coolerPicture)
	if err != nil {
		return errors.Wrap(err, "failed to get hot value")
	}
	testing.ContextLog(ctx, "cooler color is "+coolerColor)
	return nil
}
