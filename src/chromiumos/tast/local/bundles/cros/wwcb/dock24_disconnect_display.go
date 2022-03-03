// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// 24 Disconnect display while computer suspended

// "Pre-Condition:
// (Please note: Brand / Model number on test result)
// 1. External displays (Single/Dual)
// 2. Docking station / Hub /Dongle
// 3. Connection Type (HDMI/DP/VGA/DVI/USB-C on test result)

// Procedure:
// 1) Boot-up and Sign-In to the device
// 2) Connect ext-display to (Dock/Hub)
// 3) Connect (Dock/Hub) to Chromebook
// 4) Let the device in suspended mode
// 5) Disconnect ext-display from (Dock/Hub) while in suspended mode

// Verification:
// 4) Make sure Chromebook ""Primary"" screen does not wakeup and no crash/reboot
// "

/////////////////////////////////////////////////////////////////////////////////////
// automation
// "Preperation:
// 1. Monitor (Type-C)
// 2. Chromebook
// 3. Docking Station
// 4. Type-C cable

// Test Step:
// 1. Power the Chrombook On.
// 2. Sign-in account.
// 3. Connect the external monitor to the docking station via Type-C cable. (Manual)
// 4. Connect the docking station to chromebook via Type-C cable. (switch Type-C & HDMI fixture)
// 5. Run verification step 1.
// 6. Waiting for 6-8 mins the Chromebook will auto sleep. can use command
// 7. Disconnect the external monitor from docking station.(switch Type-C & HDMI fixture)
// 8. Run verification step 2."

// Automation verification
// "1. Check the external monitor display properly by test fixture.
// 2. Use camera to check if there is no screen (black screen) on the Chromebook screen."

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
		Func:         Dock24DisconnectDisplay,
		Desc:         "Disconnect display while computer suspended",
		Contacts:     []string{"flin@google.com", "newmanliu19020@allion.corp-partner.google.com"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      10 * time.Minute,
		Pre:          chrome.LoggedIn(),
		Vars:         []string{"WWCBIP", "DockingID", "1stExtDispID", "InternalDisplayCamera"},
	})
}

func Dock24DisconnectDisplay(ctx context.Context, s *testing.State) {
	extDispID := s.RequiredVar("1stExtDispID")
	dockingID := s.RequiredVar("DockingID")

	cr := s.PreValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to open connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	cleanup, err := utils.InitFixtures(ctx)
	if err != nil {
		s.Fatal("Failed to initialize fixtures: ", err)
	}
	defer cleanup(ctx)

	testing.ContextLog(ctx, "Step 1 - Power the Chrombook On")

	testing.ContextLog(ctx, "Step 2 - Sign-in account")

	// step 3 - connect ext-display to docking station
	if err := dock24DisconnectDisplayStep3(ctx, extDispID); err != nil {
		s.Fatal("Failed to execute step 3: ", err)
	}

	// step 4 - connect docking station to chromebook
	if err := dock24DisconnectDisplayStep4(ctx, dockingID); err != nil {
		s.Fatal("Failed to execute step 4: ", err)
	}

	// step 5 - check the external monitor display properly
	if err := dock24DisconnectDisplayStep5(ctx, tconn); err != nil {
		s.Fatal("Failed to execute step 5: ", err)
	}

	// step 6 - Waiting for 6-8 mins the Chromebook will auto sleep. can use command
	// step 7 - Disconnect the external monitor from docking station.(switch Type-C & HDMI fixture)
	// step 8 - Use camera to check if there is no screen (black screen) on the Chromebook screen."
	if err := dock24DisconnectDisplayStep6To8(ctx, s, cr, tconn, extDispID); err != nil {
		s.Fatal("Failed to execute step 6, 7, 8: ", err)
	}

}

func dock24DisconnectDisplayStep3(ctx context.Context, extDispID string) error {
	testing.ContextLog(ctx, "Step 3 - Connect the external monitor to the docking station")
	if err := utils.SwitchFixture(ctx, extDispID, "on", "0"); err != nil {
		return errors.Wrap(err, "failed to connect external display")
	}
	return nil
}

func dock24DisconnectDisplayStep4(ctx context.Context, dockingID string) error {
	testing.ContextLog(ctx, "Step 4 - Connect the docking station to chromebook via Type-C cable")
	if err := utils.SwitchFixture(ctx, dockingID, "on", "0"); err != nil {
		return errors.Wrap(err, "failed to connect docking station")
	}
	return nil
}

func dock24DisconnectDisplayStep5(ctx context.Context, tconn *chrome.TestConn) error {
	testing.ContextLog(ctx, "Step 5 - Check the external monitor display properly")
	if err := utils.VerifyDisplayCount(ctx, tconn, 2); err != nil {
		return errors.Wrap(err, "failed to verify display properly")
	}
	return nil
}

func dock24DisconnectDisplayStep6To8(ctx context.Context, s *testing.State, cr *chrome.Chrome, tconn *chrome.TestConn, extDispID string) error {

	testing.ContextLog(ctx, "Step 6 - Suspend then reconnect chromebook")

	testing.ContextLog(ctx, "Step 7 - Disconnect the external monitor from docking station")

	testing.ContextLog(ctx, "Step 8 - Use camera to check if there is no screen (black screen) on the Chromebook screen")

	if err := utils.SwitchFixture(ctx, extDispID, "off", "0"); err != nil {
		return errors.Wrap(err, "failed to disconnect external display")
	}
	if err := utils.CameraCheckColorLater(ctx, s, s.RequiredVar("InternalDisplayCamera")); err != nil {
		return errors.Wrap(err, "failed to tell camera capture screen later")
	}

	if err := utils.SuspendAndResume(ctx, cr, 15); err != nil {
		return errors.Wrap(err, "failed to suspend then reconnect chromebook")
	}

	if err := utils.CameraCheckColorResult(ctx, "black"); err != nil {
		return errors.Wrap(err, "failed to use camera to check if there is no screen")
	}
	return nil
}
