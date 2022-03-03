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
		Contacts:     []string{"allion-sw@allion.com"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      10 * time.Minute,
		Pre:          chrome.LoggedIn(), //Boot-up and Sign-In to the device
		Vars:         []string{"WWCBIP", "IntrenalDisplayCamera"},
	})
}

func Dock24DisconnectDisplay(ctx context.Context, s *testing.State) {
	// set up
	cr := s.PreValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to open connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	if err := utils.InitFixtures(ctx); err != nil {
		s.Fatal("Failed to initialize fixtures: ", err)
	}

	s.Log("Step 1 - Power the Chrombook On")

	s.Log("Step 2 - Sign-in account")

	// step 3 - connect ext-display to docking station
	if err := dock24DisconnectDisplayStep3(ctx, s); err != nil {
		s.Fatal("Failed to execute step 3: ", err)
	}

	// step 4 - connect docking station to chromebook
	if err := dock24DisconnectDisplayStep4(ctx, s); err != nil {
		s.Fatal("Failed to execute step 4: ", err)
	}

	// step 5 - check the external monitor display properly
	if err := dock24DisconnectDisplayStep5(ctx, s, tconn); err != nil {
		s.Fatal("Failed to execute step 5: ", err)
	}

	// 	step 6 - Waiting for 6-8 mins the Chromebook will auto sleep. can use command
	// step 7 - Disconnect the external monitor from docking station.(switch Type-C & HDMI fixture)
	// step 8 - Use camera to check if there is no screen (black screen) on the Chromebook screen."
	if err := dock24DisconnectDisplayStep6To8(ctx, s, cr, tconn); err != nil {
		s.Fatal("Failed to execute step 6, 7, 8: ", err)
	}

}

func dock24DisconnectDisplayStep3(ctx context.Context, s *testing.State) error {

	s.Log("Step 3 - Connect the external monitor to the docking station")

	if err := utils.ControlFixture(ctx, s, utils.ExtDisp1Type, utils.ExtDisp1Index, utils.ActionPlugin, false); err != nil {
		return errors.Wrap(err, "failed to connect ext-display to docking station")
	}

	return nil
}

func dock24DisconnectDisplayStep4(ctx context.Context, s *testing.State) error {

	s.Log("Step 4 - Connect the docking station to chromebook via Type-C cable")

	if err := utils.ControlFixture(ctx, s, utils.StationType, utils.StationIndex, utils.ActionPlugin, false); err != nil {
		return errors.Wrap(err, "failed to connect docking station to chromebook")
	}

	return nil
}

func dock24DisconnectDisplayStep5(ctx context.Context, s *testing.State, tconn *chrome.TestConn) error {

	s.Log("Step 5 - Check the external monitor display properly")

	if err := utils.VerifyDisplayProperly(ctx, tconn, 2); err != nil {
		return errors.Wrap(err, "failed to verify display properly")
	}

	return nil
}

func dock24DisconnectDisplayStep6To8(ctx context.Context, s *testing.State, cr *chrome.Chrome, tconn *chrome.TestConn) error {

	s.Log("Step 6 - Suspend then reconnect chromebook")

	s.Log("Step 7 - Disconnect the external monitor from docking station ")

	s.Log("Step 8 - Use camera to check if there is no screen (black screen) on the Chromebook screen")

	if err := utils.ControlFixture(ctx, s, utils.ExtDisp1Type, utils.ExtDisp1Index, utils.ActionUnplug, true); err != nil {
		return errors.Wrap(err, "failed to disconnect ext-display from docking station")
	}

	if err := utils.CameraCheckColorLater(ctx, s, s.RequiredVar("InternalDisplayCamera")); err != nil {
		return errors.Wrap(err, "failed to tell camera capture screen later")
	}

	_, err := utils.SuspendChromebook(ctx, s, cr)
	if err != nil {
		return errors.Wrap(err, "failed to suspend then reconnect chromebook")
	}

	if err := utils.CameraCheckColorResult(ctx, "black"); err != nil {
		return errors.Wrap(err, "failed to use camera to check if there is no screen")
	}

	return nil
}
