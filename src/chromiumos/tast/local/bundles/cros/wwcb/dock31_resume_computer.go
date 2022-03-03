// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// 31 Resume a suspended computer with displays connected

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
// 5) Press any keys on (Keyboard or Touchpad or move the Mouse) to Resume the device

// Verification:
// 5) Make sure both the ""Primary and Extended"" screen will wake up without any issue.

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
// 7. Run verification step 3 & 4.
// 8. Move on Touchpad to resume the chromebook system. (use fixture)
// 9. Run verification step 5 & 6."

// Automation verification
// 1. Check the external monitor display properly by test fixture.
// 2. Check the chromebook display properly by test fixture.
// 3. Check the external monitor become dark by test fixture.
// 4. Check the chromebook display become dark by test fixture.
// 5. Check the external monitor display properly by test fixture.
// 6. Check the chromebook display properly by test fixture."

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
		Func:         Dock31ResumeComputer,
		Desc:         "Suspend the computer while external display connected",
		Contacts:     []string{"allion-sw@allion.com"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      4 * time.Minute,
		Pre:          chrome.LoggedIn(), //Boot-up and Sign-In to the device
		Vars:         []string{"WWCBIP", "InternalDisplayCamera", "ExternalDisplayCamera"},
	})
}

func Dock31ResumeComputer(ctx context.Context, s *testing.State) { // chrome.LoggedIn()

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
	if err := dock31ResumeComputerStep3(ctx, s); err != nil {
		s.Fatal("Failed to execute step 3: ", err)
	}

	// step 4 - connect docking station to chromebook
	if err := dock31ResumeComputerStep4(ctx, s); err != nil {
		s.Fatal("Failed to execute step 4: ", err)
	}

	// step 5 - Check the external monitor display properly by test fixture.
	// step 6 - Check the chromebook display properly by test fixture.
	if err := dock31ResumeComputerStep5To6(ctx, s, tconn); err != nil {
		s.Fatal("Failed to execute step 5, 6: ", err)
	}

	// step 7 -  suspend chromebook,
	// step 8 - Check the external monitor become dark by test fixture.
	// step 9 - Check the chromebook display become dark by test fixture.
	if err := dock31ResumeComputerStep7To9(ctx, s, cr); err != nil {
		s.Fatal("Failed to execute step 7, 8, 9: ", err)
	}

	// step 10 - wake up chromebook
	// step 11 - Check the external monitor display properly by test fixture.
	// step 12 - Check the chromebook display properly by test fixture.
	if err := dock31ResumeComputerStep10To12(ctx, s, cr, tconn); err != nil {
		s.Fatal("Failed to execute 10, 11, 12: ", err)
	}

}

func dock31ResumeComputerStep3(ctx context.Context, s *testing.State) error {

	s.Log("Step 3 - Connect the external monitor to the docking station")

	if err := utils.ControlFixture(ctx, s, utils.ExtDisp1Type, utils.ExtDisp1Index, utils.ActionPlugin, false); err != nil {
		return errors.Wrap(err, "failed to connect ext-display to docking station")
	}

	return nil
}

func dock31ResumeComputerStep4(ctx context.Context, s *testing.State) error {

	s.Log("Step 4 - Connect the docking station to chromebook")

	if err := utils.ControlFixture(ctx, s, utils.StationType, utils.StationIndex, utils.ActionPlugin, false); err != nil {
		return errors.Wrap(err, "failed to connect docking station to chromebook")
	}

	return nil
}

func dock31ResumeComputerStep5To6(ctx context.Context, s *testing.State, tconn *chrome.TestConn) error {

	s.Log("Step 5 - Check the external monitor display properly by test fixture")

	s.Log("Step 6 - Check the chromebook display properly by test fixture")

	if err := utils.VerifyDisplayProperly(ctx, tconn, 2); err != nil {
		return errors.Wrap(err, "failed to verify display properly")
	}

	return nil
}

func dock31ResumeComputerStep7To9(ctx context.Context, s *testing.State, cr *chrome.Chrome) error {

	s.Log("Step 7 - suspend chromebook")

	s.Log("Step 8 - Check the external monitor become dark by test fixture")

	s.Log("Step 9 - Check the chromebook display become dark by test fixture")

	// call before suspend
	if err := utils.CameraCheckColorLater(ctx, s, s.RequiredVar("InternalDisplayCamera")); err != nil {
		return errors.Wrap(err, "failed to execute CheckColorLater")
	}

	// 7. Wait 10 minutes, the Chromebook will automatically go to sleep.
	_, err := utils.SuspendChromebook(ctx, s, cr)
	if err != nil {
		return errors.Wrap(err, "failed to suspend then reconnect chromebook")
	}

	if err := utils.CameraCheckColorResult(ctx, "black"); err != nil {
		return errors.Wrap(err, "failed to execute CheckColorResult")
	}

	return nil
}

func dock31ResumeComputerStep10To12(ctx context.Context, s *testing.State, cr *chrome.Chrome, tconn *chrome.TestConn) error {

	s.Log("Step 10 - Wake up chromebook")

	s.Log("Step 11 - Check the external monitor display properly by test fixture")

	s.Log("Step 12 - Check the chromebook display properly by test fixture")

	if err := utils.CameraCheckColor(ctx, s.RequiredVar("InternalDisplayCamera"), "white"); err != nil {
		return errors.Wrap(err, "Fail to check chromebook monitor is white")
	}

	if err := utils.CameraCheckColor(ctx, s.RequiredVar("ExternalDisplayCamera"), "white"); err != nil {
		return errors.Wrap(err, "Fail to check external monitor is white")
	}

	return nil
}
