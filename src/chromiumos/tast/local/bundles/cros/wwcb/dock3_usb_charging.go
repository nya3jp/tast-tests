// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

/***
#15 USB Charging via a powered Dock
Pre-Condition:
(Please note: Brand / Model number on test result)
1. External displays
2. Docking station
3. Connection Type (HDMI/DP/VGA/DVI/USB-C on test result)

Procedure:
1)  Boot-up and Sign-In to the device
2)  Connect ext-display to (Powered Docking station)
3)  Connect (Powered Docking station) to Chromebook


Verification:
- Chrome Book /Chrome Box "Battery" icon should show (Lighting Bolt charging) indicator
***/

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
		Func:         Dock3UsbCharging,
		Desc:         "Test power charging via a powered Dock over USB-C",
		Contacts:     []string{"allion-sw@allion.com"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      4 * time.Minute,
		Vars:         utils.InputArguments,
		Pre:          chrome.LoggedIn(), // 1)  Boot-up and Sign-In to the device
	})
}

func Dock3UsbCharging(ctx context.Context, s *testing.State) {

	// set up
	cr := s.PreValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	if err := utils.InitFixtures(ctx); err != nil {
		s.Fatal("Failed to initialize fixtures: ", err)
	}

	s.Log("Step 1 - Boot-up and Sign-In to the device")

	// step 2 - connect ext-display to station
	if err := dock3UsbChargingStep2(ctx, s); err != nil {
		s.Fatal("Failed to execute step2: ", err)
	}

	// step 3 - connect station to chromebook
	if err := dock3UsbChargingStep3(ctx, s); err != nil {
		s.Fatal("Failed to execute step3: ", err)
	}

	// step 4 - check chromebook is charging or not
	if err := dock3UsbChargingStep4(ctx, s); err != nil {
		s.Fatal("Failed to execute step4: ", err)
	}

}

func dock3UsbChargingStep2(ctx context.Context, s *testing.State) error {

	s.Log("Step 2 - Connect ext-display to docking station")

	if err := utils.ControlFixture(ctx, s, utils.ExtDisp1Type, utils.ExtDisp1Index, utils.ActionPlugin, false); err != nil {
		return errors.Wrap(err, "failed to connect ext-display to docking station")
	}

	return nil
}

func dock3UsbChargingStep3(ctx context.Context, s *testing.State) error {

	s.Log("Step 3 - Connect docking station to chromebook")

	if err := utils.ControlFixture(ctx, s, utils.StationType, utils.StationIndex, utils.ActionPlugin, false); err != nil {
		return errors.Wrap(err, "failed to plug in station to chromebook")
	}

	return nil

}

func dock3UsbChargingStep4(ctx context.Context, s *testing.State) error {

	s.Log("Step 4 - Check chromebook is charging or not")

	if err := utils.VerifyPowerStatus(ctx, utils.IsConnect); err != nil {
		return err
	}

	return nil
}
