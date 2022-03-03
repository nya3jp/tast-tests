// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// #13 Change Resolution being displayed on external monitor
// Pre-Condition:
// (Please note: Brand / Model number on test result)
// 1. External displays (Single /Dual)
// 2. Docking station /Hub /Dongle
// 3. Connection Type (HDMI/DP/VGA/DVI/USB-C on test result)

// Procedure:
// 1) Boot-up and Sign-In to the device
// 2) Connect ext-display to (Docking station or Hub)
// 3) Connect (Docking station or Hub) to Chromebook
// 4) Open Chrome Browser: www.youtube.com
// 5) Go to "Quick Settings Menu and Setting /Device /Displays
// 6) Select "Extended" (Ext-Display) and change "Resolutions" settings (Low - Medium - Highest)

// Verification:
// 6)  Make sure "Extended" (Ext-Displays) screen resolutions changed

// Package wwcb contains local Tast tests that work with chromebook
package wwcb

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/wwcb/utils"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Dock13ChangeResolution,
		Desc:         "Change Resolution being displayed on external monitor",
		Contacts:     []string{"flin@google.com", "newmanliu19020@allion.corp-partner.google.com"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      4 * time.Minute,
		Vars:         []string{"WWCBIP", "DockingID", "1stExtDispID"},
		Pre:          chrome.LoggedIn(),
	})
}

func Dock13ChangeResolution(ctx context.Context, s *testing.State) {
	dockingID := s.RequiredVar("DockingID")
	extDispID1 := s.RequiredVar("1stExtDispID")

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

	testing.ContextLog(ctx, "Step 1 - Boot-up and Sign-In to the device")

	// Step 2 - Connect ext-display to docking station.
	if err := dock13ChangeResolutionStep2(ctx, extDispID1); err != nil {
		s.Fatal("Failed to execute step 2: ", err)
	}

	// Step 3 - Connect docking station to Chromebook.
	if err := dock13ChangeResolutionStep3(ctx, dockingID); err != nil {
		s.Fatal("Failed to execute step 3: ", err)
	}

	// Step 4 - Change ext-display resolution - low / medium / high.
	if err := dock13ChangeResolutionStep4(ctx, tconn); err != nil {
		s.Fatal("Fatal to execute step 4: ", err)
	}
}

func dock13ChangeResolutionStep2(ctx context.Context, extDispID1 string) error {
	testing.ContextLog(ctx, "Step 2 - Connect ext-display to docking station")
	if err := utils.SwitchFixture(ctx, extDispID1, "on", "0"); err != nil {
		return errors.Wrap(err, "failed to connect external display")
	}
	return nil
}

func dock13ChangeResolutionStep3(ctx context.Context, dockingID string) error {
	testing.ContextLog(ctx, "Step 3 - Connect docking station to Chromebook")
	if err := utils.SwitchFixture(ctx, dockingID, "on", "0"); err != nil {
		return errors.Wrap(err, "failed to connect docking station")
	}
	return nil
}

func dock13ChangeResolutionStep4(ctx context.Context, tconn *chrome.TestConn) error {
	testing.ContextLog(ctx, "Step 4 - Chage ext-display resolution")

	if err := utils.VerifyExternalDisplay(ctx, tconn, true); err != nil {
		return errors.Wrap(err, "failed to verify ext-display is connected")
	}

	infos, err := utils.GetInternalAndExternalDisplays(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get internal & external display info")
	}

	// Check modes are enough for testing
	if len(infos.External.Modes) < 3 {
		return errors.New("external display modes are not enough, must at least 3")
	}

	// Change ext-display resolution low, middle, high.
	for _, param := range []struct {
		resolution  string
		displayMode display.DisplayMode
	}{
		{"low", *infos.External.Modes[0]},
		{"medium", *infos.External.Modes[(len(infos.External.Modes)-1)/2]},
		{"high", *infos.External.Modes[len(infos.External.Modes)-1]},
	} {
		testing.ContextLogf(ctx, "Setting %s resolution", param.resolution)

		p := display.DisplayProperties{DisplayMode: &param.displayMode}
		if err := display.SetDisplayProperties(ctx, tconn, infos.External.ID, p); err != nil {
			return errors.Wrap(err, "failed to set display properties")
		}
	}
	return nil
}
