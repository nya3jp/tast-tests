// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// 30 Suspend the computer while external display connected.

// "Pre-Condition:
// (Please note: Brand / Model number on test result)
// 1. External displays (Single/Dual)
// 2. Docking station / Hub /Dongle /Adapter
// 3. Connection Type (HDMI/DP/VGA/DVI/USB-C on test result)

// Procedure:
// 1) Boot-up and Sign-In to the device
// 2) Connect ext-display onto (Dock/Hub/Dongle/Adapter)
// 3) Connect (Dock/Hub/Dongle/Adapter) onto Chromebook
// 4) By default, Chromebook will automatically go to sleep if it does nothing for 6-8 minutes without closing the lid.

// Verification:
// 4) Make sure both the ""Primary and Extended"" screen will go to sleep.

// "

/////////////////////////////////////////////////////////////////////////////////////
// automation
// "Preperation:
// 1. Monitor.
// 2. Chromebook
// 3. Docking Station
// 4. Type-C cable

// Test Step:
// 1. Power the Chrombook On.
// 2. Sign-in account.
// 3. Connect the external monitor to the docking station via Type-C cable. (Manual)
// 4. Connect the docking station to chromebook via Type-C cable. (switch Type-C & HDMI fixture)
// 5. Run verification step 1 & 2.
// 6. Wait 10 minutes, the Chromebook will automatically go to sleep.
// 7. Run verification step 3 & 4."

// Automation verification
// "1. Check the external monitor display properly by test fixture.
// 2. Check the chromebook display properly by test fixture.
// 3. Check the external monitor become dark by test fixture.
// 4. Check the chromebook display become dark by test fixture."

// Package wwcb contains local Tast tests that work with chromebook
package wwcb

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/wwcb/utils"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Dock30SuspendComputer,
		Desc:         "Suspend the computer while external display connected",
		Contacts:     []string{"flin@google.com", "newmanliu19020@allion.corp-partner.google.com"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      4 * time.Minute,
		Pre:          chrome.LoggedIn(),
		Vars:         []string{"WWCBIP", "DockingID", "1stExtDispID", "InternalDisplayCamera"},
	})
}

func Dock30SuspendComputer(ctx context.Context, s *testing.State) { // chrome.LoggedIn()
	extDispID := s.RequiredVar("1stExtDispID")
	dockingID := s.RequiredVar("DockingID")

	cr := s.PreValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to open connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	if err := utils.InitFixtures(ctx); err != nil {
		s.Fatal("Failed to initialize fixtures: ", err)
	}

	testing.ContextLog(ctx, "Step 1 - Power the Chrombook On")

	testing.ContextLog(ctx, "Step 2 - Sign-in account")

	// step 3 - connect ext-display to station
	if err := dock30SuspendComputerStep3(ctx, extDispID); err != nil {
		s.Fatal("Failed to execute step 3: ", err)
	}

	// step 4 - connect station to chromebook
	if err := dock30SuspendComputerStep4(ctx, dockingID); err != nil {
		s.Fatal("Failed to execute step 4: ", err)
	}

	// Step 5 - check the external monitor display properly by test fixture
	// Step 6 - check the chromebook display properly by test fixture
	if err := dock30SuspendComputerStep5To6(ctx, tconn); err != nil {
		s.Fatal("Failed to execute step 5, 6: ", err)
	}

	// step 7 - suspend chromebook
	// step 8 - Check the external monitor become dark by test fixture.
	// step 9 - Check the chromebook display become dark by test fixture.
	if err := dock30SuspendComputerStep7To9(ctx, s, cr); err != nil {
		s.Fatal("Failed to execute step 7, 8, 9: ", err)
	}

}

func dock30SuspendComputerStep3(ctx context.Context, extDispID string) error {
	testing.ContextLog(ctx, "Step 3 - Connect the external monitor to the docking station")
	if err := utils.SwitchFixture(ctx, extDispID, "on", "0"); err != nil {
		return errors.Wrap(err, "failed to connect external display")
	}
	return nil
}

func dock30SuspendComputerStep4(ctx context.Context, dockingID string) error {
	testing.ContextLog(ctx, "Step 4 - Connect the docking station to chromebook")
	if err := utils.SwitchFixture(ctx, dockingID, "on", "0"); err != nil {
		return errors.Wrap(err, "failed to connect docking station")
	}
	return nil
}

func dock30SuspendComputerStep5To6(ctx context.Context, tconn *chrome.TestConn) error {
	testing.ContextLog(ctx, "Step 5 - Check the external monitor display properly by test fixture")
	testing.ContextLog(ctx, "Step 6 - Check the chromebook display properly by test fixture")
	if err := utils.VerifyDisplayCount(ctx, tconn, 2); err != nil {
		return errors.Wrap(err, "failed to verify display properly")
	}
	return nil
}

func dock30SuspendComputerStep7To9(ctx context.Context, s *testing.State, cr *chrome.Chrome) error {
	testing.ContextLog(ctx, "Step 7 - Suspend chromebook")
	testing.ContextLog(ctx, "Step 8 - Check the external monitor become dark by test fixture")
	testing.ContextLog(ctx, "Step 9 - Check the chromebook display become dark by test fixture")
	// call before suspend
	if err := utils.CameraCheckColorLater(ctx, s, s.RequiredVar("InternalDisplayCamera")); err != nil {
		return errors.Wrap(err, "failed to execute CheckColorLater")
	}

	// suspend chromebook
	if err := utils.SuspendAndResume(ctx, cr, 15); err != nil {
		return errors.Wrap(err, "failed to suspend then reconnect chromebook")
	}

	// check color
	if err := utils.CameraCheckColorResult(ctx, "black"); err != nil {
		return errors.Wrap(err, "failed to execute CheckColorResult")
	}
	return nil
}
