// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// 25 Disconnect display while computer is shut down

// "Pre-Condition:
// (Please note: Brand / Model number on test result)
// 1. External displays (Single/Dual)
// 2. Docking station / Hub /Dongle
// 3. Connection Type (HDMI/DP/VGA/DVI/USB-C on test result)

// Procedure:
// 1) Boot-up and Sign-In to the device
// 2) Connect ext-display to (Dock/Hub)
// 3) Connect (Dock/Hub) to Chromebook
// 4) Disconnect the display while Chromebook is shut down

// Verification:
// 4) Make sure Chromebook shut down without any issue

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
// 6. At the bottom right, select the time to display the Quick Settings.
// 7. Select Shut down to power off the chromebook system.
// 8. Disconnect the external monitor from docking station.
// 9. Run verification step 2."

// Automation verification
// "1. Check the external monitor display properly by test fixture.
// 2. Wait 2-3 min and use camera to check if there is no screen (black screen) on the Chromebook screen."

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
		Func:         Dock25DisconnectDisplay,
		Desc:         "Disconnect display while computer is shut down",
		Contacts:     []string{"allion-sw@allion.com"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      2 * time.Minute,
		Pre:          chrome.LoggedIn(), //Boot-up and Sign-In to the device
		Vars:         []string{"WWCB", "InternalDisplayCamera"},
	})
}

func Dock25DisconnectDisplay(ctx context.Context, s *testing.State) {

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

	// step 3 - connect ext-display to docking
	if err := dock25DisconnectDisplayStep3(ctx, s); err != nil {
		s.Fatal("Failed to execute step 3: ", err)
	}
	// step 4 - connect docking to chromebook
	if err := dock25DisconnectDisplayStep4(ctx, s); err != nil {
		s.Fatal("Failed to execute step 4: ", err)
	}

	// step 5 - check ext-display properly
	if err := dock25DisconnectDisplayStep5(ctx, s, tconn); err != nil {
		s.Fatal("Failed to execute step 5: ", err)
	}

	// step 6, 7 - power off chromebook
	if err := dock25DisconnectDisplayStep6To7(ctx, s, tconn); err != nil {
		s.Fatal("Failed to execute step 6, 7: ", err)
	}

	// because chromebok has powered off,
	// then SSH would lost,
	// do it from outside
	// 8. Disconnect the external monitor from docking station.
	// 9. Wait 2-3 min and use camera to check if there is no screen (black screen) on the Chromebook screen.

}

func dock25DisconnectDisplayStep3(ctx context.Context, s *testing.State) error {

	s.Log("Step 3 - Connect the external monitor to the docking station via Type-C cable")

	if err := utils.ControlFixture(ctx, s, utils.ExtDisp1Type, utils.ExtDisp1Index, utils.ActionPlugin, false); err != nil {
		return errors.Wrap(err, "failed to plug in ext-display to docking station")
	}

	return nil
}

func dock25DisconnectDisplayStep4(ctx context.Context, s *testing.State) error {

	s.Log("Step 4 - Connect the docking station to chromebook via Type-C cable")

	if err := utils.ControlFixture(ctx, s, utils.StationType, utils.StationIndex, utils.ActionPlugin, false); err != nil {
		return errors.Wrap(err, "failed to connect docking station to chromebook")
	}

	return nil
}

func dock25DisconnectDisplayStep5(ctx context.Context, s *testing.State, tconn *chrome.TestConn) error {

	s.Log("Step 5 - Check the external monitor display properly by test fixture")

	if err := utils.VerifyDisplayProperly(ctx, tconn, 2); err != nil {
		return errors.Wrap(err, "failed to verify display properly")
	}

	return nil
}

func dock25DisconnectDisplayStep6To7(ctx context.Context, s *testing.State, tconn *chrome.TestConn) error {

	s.Log("Step 6, 7 - power off chromebook")

	if err := utils.PoweroffChromebook(ctx, s); err != nil {
		return errors.Wrap(err, "failed to power off chromebook")
	}

	return nil
}

func dock25DisconnectDisplayStep8(ctx context.Context, s *testing.State) error {

	s.Log("Step 8 - Disconnect the external monitor from docking station")

	if err := utils.ControlFixture(ctx, s, utils.StationType, utils.StationIndex, utils.ActionUnplug, false); err != nil {
		return errors.Wrap(err, "failed to disconnect docking staion from chromebook")
	}

	return nil
}

func dock25DisconnectDisplayStep9(ctx context.Context, s *testing.State, tconn *chrome.TestConn) error {

	s.Log("Step 9 - Use camera to check if there is no screen (black screen) on the Chromebook screen")

	if err := utils.CameraCheckColor(ctx, s.RequiredVar("InternalDisplayCamera"), "black"); err != nil {
		return errors.Wrap(err, "failed to use camera to check chromebook screen is black")
	}

	return nil
}
