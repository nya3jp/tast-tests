// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

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
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Dock17Closelid,
		Desc:         "Verify that display resolution is still okay after lid close & windows are all still  displayed",
		Contacts:     []string{"flin@google.com", "newmanliu19020@allion.corp-partner.google.com"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      10 * time.Minute,
		Pre:          chrome.LoggedIn(),
		Vars:         []string{"WWCBIP", "DockingID", "1stExtDispID"},
	})
}

func Dock17Closelid(ctx context.Context, s *testing.State) {
	extDispID := s.RequiredVar("1stExtDispID")
	dockingID := s.RequiredVar("DockingID")

	cr := s.PreValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	cleanup, err := utils.InitFixtures(ctx)
	if err != nil {
		s.Fatal("Failed to initialize fixtures: ", err)
	}
	defer cleanup(ctx)

	testing.ContextLog(ctx, "Step 1 - Power the Chromebook On")

	testing.ContextLog(ctx, "Step 2 - Sign-in account")

	// Step 3 - Connect ext-display to docking station.
	if err := dock17CloselidStep3(ctx, extDispID); err != nil {
		s.Fatal("Failed to execute step3: ", err)
	}

	// Step 4 - Connect docking station to Chromebook.
	if err := dock17CloselidStep4(ctx, dockingID); err != nil {
		s.Fatal("Failed to execute step4: ", err)
	}

	// Step 5 - Power off Chromebook display,
	// then veriy window bounds on external,
	// and verify resolution remain the same.
	if err := dock17CloselidStep5(ctx, cr, tconn); err != nil {
		s.Fatal("Failed to execute step5: ", err)
	}
}

func dock17CloselidStep3(ctx context.Context, extDispID string) error {
	testing.ContextLog(ctx, "Step 3 - Connect external display to docking station")
	if err := utils.SwitchFixture(ctx, extDispID, "on", "0"); err != nil {
		return errors.Wrap(err, "failed to connect external display")
	}
	return nil
}

func dock17CloselidStep4(ctx context.Context, dockingID string) error {
	testing.ContextLog(ctx, "Step 4 - Connect station to chromebook")
	if err := utils.SwitchFixture(ctx, dockingID, "on", "0"); err != nil {
		return errors.Wrap(err, "failed to connect docking station")
	}
	return nil
}

func dock17CloselidStep5(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn) error {
	testing.ContextLog(ctx, "Step 5 - Verify window bounds on external display after power off Chromebook display")

	if err := utils.VerifyExternalDisplay(ctx, tconn, true); err != nil {
		return errors.Wrap(err, "failed to verify ext-display is connected")
	}

	before, err := utils.GetInternalAndExternalDisplays(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get internal and external display")
	}

	if _, err := browser.Launch(ctx, tconn, cr, "https://www.google.com"); err != nil {
		return errors.Wrap(err, "failed to launch browser")
	}

	if err := utils.SetDisplayPower(ctx, utils.DisplayPowerInternalOffExternalOn); err != nil {
		return errors.Wrap(err, "failed to close internal moniter power")
	}

	// Verify window bounds on external display.
	// Verify resolution remain the same.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		after, err := display.GetPrimaryInfo(ctx, tconn)
		if err != nil {
			return errors.Wrap(err, "failed to get primary display info")
		}

		browser, err := ash.FindWindow(ctx, tconn, func(w *ash.Window) bool {
			return w.WindowType == ash.WindowTypeBrowser
		})
		if err != nil {
			return errors.Wrap(err, "failed to find browser window")
		}

		if browser.DisplayID != after.ID {
			return errors.Errorf("unexpected display ID; go %q, want %q", browser.DisplayID, after.ID)
		}

		if before.External.Bounds.Width != after.Bounds.Width || before.External.Bounds.Height != after.Bounds.Height {
			return errors.New("resolution should not be changed")
		}
		return nil
	}, &testing.PollOptions{Timeout: 30 * time.Second, Interval: 2 * time.Second}); err != nil {
		return err
	}
	return nil
}
