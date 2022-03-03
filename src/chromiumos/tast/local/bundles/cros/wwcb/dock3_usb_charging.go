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

// Package wwcb contains local Tast tests that work with Chromebook
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
		Contacts:     []string{"flin@google.com", "newmanliu19020@allion.corp-partner.google.com"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      4 * time.Minute,
		Vars:         []string{"WWCBIP", "DockingID", "1stExtDispID"},
		Pre:          chrome.LoggedIn(),
	})
}

func Dock3UsbCharging(ctx context.Context, s *testing.State) {
	dockingID := s.RequiredVar("DockingID")
	extDispID1 := s.RequiredVar("1stExtDispID")

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

	// Step 1 - Connect ext-display to docking station.
	if err := dock3UsbChargingStep1(ctx, extDispID1); err != nil {
		s.Fatal("Failed to execute step 1: ", err)
	}

	// Step 2 - Connect docking station to Chromebook.
	if err := dock3UsbChargingStep2(ctx, dockingID); err != nil {
		s.Fatal("Failed to execute step 2: ", err)
	}

	// Step 3 - Verify Chromebook is charging or not.
	if err := dock3UsbChargingStep3(ctx); err != nil {
		s.Fatal("Failed to execute step 3: ", err)
	}
}

func dock3UsbChargingStep1(ctx context.Context, extDispID1 string) error {
	testing.ContextLog(ctx, "Step 1 - Connect ext-display to docking station")
	if err := utils.SwitchFixture(ctx, extDispID1, "on", "0"); err != nil {
		return errors.Wrap(err, "failed to connect external display")
	}
	return nil
}

func dock3UsbChargingStep2(ctx context.Context, dockingID string) error {
	testing.ContextLog(ctx, "Step 2 - Connect docking station to Chromebook")
	if err := utils.SwitchFixture(ctx, dockingID, "on", "0"); err != nil {
		return errors.Wrap(err, "failed to connect docking station")
	}
	return nil
}

func dock3UsbChargingStep3(ctx context.Context) error {
	testing.ContextLog(ctx, "Step 3 - Verify Chromebook is charging or not")
	if err := utils.VerifyPowerStatus(ctx, true); err != nil {
		return errors.Wrap(err, "failed to verfiy power is charging")
	}
	return nil
}
