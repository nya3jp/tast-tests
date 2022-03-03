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
		Contacts:     []string{"allion-sw@allion.com"},
		SoftwareDeps: []string{"chrome"},
		Pre:          chrome.LoggedIn(), // 1) Boot-up and Sign-In to the device
		Vars:         utils.InputArguments,
	})
}

func Dock13ChangeResolution(ctx context.Context, s *testing.State) {

	cr := s.PreValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to open connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	if err := utils.InitFixtures(ctx); err != nil {
		s.Fatal("Failed to initialize fixtures: ", err)
	}

	s.Log("Step 1 - Boot-up and Sign-In to the device ")

	// step 2 - connect ext-display
	if err := dock13ChangeResolutionStep2(ctx, s); err != nil {
		s.Fatal("Failed to execute step 2: ", err)
	}

	// step 3 - connect docking station
	if err := dock13ChangeResolutionStep3(ctx, s); err != nil {
		s.Fatal("Failed to execute step 3: ", err)
	}

	// step 4 - change resolution - low / medium / high
	if err := dock13ChangeResolutionStep4(ctx, s, tconn); err != nil {
		s.Fatal("Fatal to execute step 4: ", err)
	}
}

func dock13ChangeResolutionStep2(ctx context.Context, s *testing.State) error {

	s.Log("Step 2 - Connect ext-display to docking station")

	if err := utils.ControlFixture(ctx, s, utils.ExtDisp1Type, utils.ExtDisp1Index, utils.ActionPlugin, false); err != nil {
		return errors.Wrap(err, "failed to connect ext-display to docking station")
	}

	return nil
}

func dock13ChangeResolutionStep3(ctx context.Context, s *testing.State) error {

	s.Log("Step 3 - Connect docking station to chromebook")

	if err := utils.ControlFixture(ctx, s, utils.StationType, utils.StationIndex, utils.ActionPlugin, false); err != nil {
		return errors.Wrap(err, "failed to connect docking station")
	}

	return nil
}

func dock13ChangeResolutionStep4(ctx context.Context, s *testing.State, tconn *chrome.TestConn) error {

	if err := testing.Poll(ctx, func(c context.Context) error {
		// get external display info
		extDispInfo, err := utils.ExternalDisplay(ctx, tconn)
		if err != nil {
			return errors.Wrap(err, "failed to get external display info")
		}

		s.Log("external info : ", extDispInfo)

		s.Logf("length of ext-display's mode is %d", len(extDispInfo.Modes))

		low := extDispInfo.Modes[0]

		medium := extDispInfo.Modes[(len(extDispInfo.Modes)-1)/2]

		high := extDispInfo.Modes[len(extDispInfo.Modes)-1]

		// change resolution - (low, medium, highest), then check
		// 	using mode to change
		for _, param := range []struct {
			displayMode display.DisplayMode
		}{
			{*low}, {*medium}, {*high},
		} {

			mode := param.displayMode

			s.Log("Setting display properties: mode = ", mode)

			p := display.DisplayProperties{DisplayMode: &mode}
			if err := display.SetDisplayProperties(ctx, tconn, extDispInfo.ID, p); err != nil {
				return errors.Wrap(err, "failed to set display properties")
			}

			testing.Sleep(ctx, 5*time.Second)

			// get external display info
			info, err := utils.ExternalDisplay(ctx, tconn)
			if err != nil {
				return errors.Wrap(err, "failed to get external display info")
			}

			// check external display info resolution
			if info.Bounds.Width != mode.Width || info.Bounds.Height != mode.Height {
				return errors.Wrap(err, "failed to check width and height")
			}

		}

		return nil
	}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
		return err
	}

	return nil

}
