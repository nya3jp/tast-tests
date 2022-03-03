// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// 26 Boot computer with new Display already connected

// "Pre-Condition:
// (Please note: Brand / Model number on test result)
// 1. External displays (Single/Dual)
// 2. Docking station / Hub /Dongle
// 3. Connection Type (HDMI/DP/VGA/DVI/USB-C on test result)

// Procedure:
// 1) Connect ext-display to (Dock/Hub)
// 2) Connect (Dock/Hub) to Chromebook
// 3) Boot-up and Sign-In to the device

// Verification:
// 4) Make sure Chromebook ""Bootup"" successfully without any issue and both screens show up.

// "

/////////////////////////////////////////////////////////////////////////////////////
// automation
// "Preperation:
// 1. Monitor (Type-C)
// 2. Chromebook
// 3. Docking Station
// 4. Type-C cable

// Test Step:
// 1. Connect the external monitor to the docking station via Type-C cable. (Manual)
// 2. Connect the docking station to chromebook via Type-C cable. (switch Type-C & HDMI fixture)
// 3. Power the Chrombook On.
// 4. Sign-in account.
// 5. Run verification."

// Automation verification
// 1. Check the external monitor display properly by test fixture.
// 2. Check the chromebook display properly by test fixture."

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
		Func:         Dock26BootComputer,
		Desc:         "Boot computer with new Display already connected",
		Contacts:     []string{"allion-sw@allion.com"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      4 * time.Minute,
		Pre:          chrome.LoggedIn(), //Boot-up and Sign-In to the device
		Vars:         utils.InputArguments,
	})
}

func Dock26BootComputer(ctx context.Context, s *testing.State) { // chrome.LoggedIn()

	cr := s.PreValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to open connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	if err := utils.InitFixtures(ctx); err != nil {
		s.Fatal("Failed to initialize fixtures: ", err)
	}

	s.Log("Step 1 - Connect external moniter to docking station manually in advance ")
	s.Log("Step 2 - connect docking station to chromebook manually in advance ")
	s.Log("Step 3 - Power the Chrombook On")
	s.Log("Step 4 - Sign-in account. ")

	// only verify
	// step 5, 6 - check display properly
	if err := dock26BootComputerStep5To6(ctx, s, tconn); err != nil {
		s.Fatal("Failed to execute 5, 6: ", err)
	}
}

func dock26BootComputerStep5To6(ctx context.Context, s *testing.State, tconn *chrome.TestConn) error {

	s.Log("Step 5, 6 - Check external display info")

	if err := utils.VerifyDisplayProperly(ctx, tconn, 2); err != nil {
		return errors.Wrap(err, "failed to verify display properly")
	}

	return nil
}
