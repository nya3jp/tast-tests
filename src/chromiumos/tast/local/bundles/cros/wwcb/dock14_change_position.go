// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// #14 Change position of display relative to Chromebook

// Pre-Condition:
// (Please note: Brand / Model number on test result)
// 1. External displays (Single /Dual)
// 2. Docking station /Hub /Dongle
// 3. Connection Type (HDMI/DP/VGA/DVI/USB-C on test result)

// Procedure:
// 1) Boot-up and Sign-In to the device
// 2) Connect ext-display to (Docking station or Hub)
// 3) Connect (Docking station or Hub) to Chromebook
// 4) Go to "Quick Settings Menu and Setting /Device /Displays
//  By default "Primary" (Built-in displays) show on the Left side of the (Ext-Displays)
// 5) Click+Hold the displays (Primary) or (Extended) ext-displays icon around (i.e. Left/Right/Top/Bottom)
// 6) On "Primary" (Built-in displays) open Chrome browser window and drag the browser window onto (Extended) ext- displays

// Verification:
// 5) Make sure the display screen show "BLUE" highlighted border around the display and able to drag around without any issue
// 6) Make sure able to drag the browser window around to the (Primary or Extended) displays

// Package wwcb contains local Tast tests that work with chromebook
package wwcb

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/wwcb/utils"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Dock14ChangePosition,
		Desc:         "Change position of display relative to Chromebook",
		Contacts:     []string{"allion-sw@allion.com"},
		SoftwareDeps: []string{"arc", "chrome"},
		Timeout:      10 * time.Minute,
		Pre:          arc.Booted(), //1) Boot-up and Sign-In to the device
		Vars:         utils.InputArguments,
	})
}

func Dock14ChangePosition(ctx context.Context, s *testing.State) {

	cr := s.PreValue().(arc.PreData).Chrome
	a := s.PreValue().(arc.PreData).ARC

	// build connection
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	if err := utils.InitFixtures(ctx); err != nil {
		s.Fatal("Failed to initialize fixtures: ", err)
	}

	s.Log("Step 1 - Boot-up and Sign-In to the device ")

	// step 2 - connect ext-display to docking station
	if err := dock14ChangePositionStep2(ctx, s, tconn); err != nil {
		s.Fatal("Failed to execute 2 : ", err)
	}

	// step 3 - connect docking station to chromebook
	if err := dock14ChangePositionStep3(ctx, s); err != nil {
		s.Fatal("Failed to execute step 3: ", err)
	}

	// step 4, 5 - change display relative position (top/bottom/left/right)
	if err := dock14ChangePositionStep4To5(ctx, s, tconn, a); err != nil {
		s.Fatal("Failed to execute step 4, 5: ", err)
	}

	// step 6 - drag window to ext-display and check window bounds on ext-display
	if err := dock14ChangePositionStep6(ctx, s, cr, tconn, a); err != nil {
		s.Fatal("Failed to execute step 6: ", err)
	}

}

func dock14ChangePositionStep2(ctx context.Context, s *testing.State, tconn *chrome.TestConn) error {

	s.Log("Step 2 - Connect ext-display to docking station")

	if err := utils.ControlFixture(ctx, s, utils.ExtDisp1Type, utils.ExtDisp1Index, utils.ActionPlugin, false); err != nil {
		return errors.Wrap(err, "failed to connect ext-display to docking station")
	}

	return nil
}

func dock14ChangePositionStep3(ctx context.Context, s *testing.State) error {

	s.Log("Step 3 - Connect docking staion to chromebook")

	if err := utils.ControlFixture(ctx, s, utils.StationType, utils.StationIndex, utils.ActionPlugin, false); err != nil {
		return errors.Wrap(err, "failed to connect docking station to chromebook")
	}

	return nil
}

func dock14ChangePositionStep4To5(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC) error {

	s.Log("Step 4, 5 - Change display relative position")

	// install testing app
	if err := a.Install(ctx, arc.APKPath(utils.TestappApk)); err != nil {
		s.Fatal("Failed installing app: ", err)
	}

	externalARCDisplayID, err := arc.FirstDisplayIDByType(ctx, a, arc.ExternalDisplay)
	if err != nil {
		return err
	}

	infos, err := display.GetInfo(ctx, tconn)
	if err != nil {
		return err
	}

	var internalDisplayInfo, externalDisplayInfo display.Info
	for _, info := range infos {
		if info.IsInternal {
			internalDisplayInfo = info
		} else if externalDisplayInfo.ID == "" {
			// Get the first external display info.
			externalDisplayInfo = info
		}
	}

	// Start settings Activity on internal display.
	settingsAct, err := arc.NewActivity(a, utils.SettingsPkg, utils.SettingsAct)
	if err != nil {
		return err
	}
	defer settingsAct.Close()

	if err := settingsAct.Start(ctx, tconn); err != nil {
		return err
	}
	defer settingsAct.Stop(ctx, tconn)
	if err := ash.WaitForVisible(ctx, tconn, utils.SettingsPkg); err != nil {
		return err
	}

	// Start wm Activity on external display.
	wmAct, err := arc.NewActivityOnDisplay(a, utils.TestappPkg, utils.TestappAct, externalARCDisplayID)
	if err != nil {
		return err
	}
	defer wmAct.Close()

	if err := wmAct.Start(ctx, tconn); err != nil {
		return err
	}
	defer wmAct.Stop(ctx, tconn)
	if err := ash.WaitForVisible(ctx, tconn, utils.TestappPkg); err != nil {
		return err
	}

	for _, test := range []struct {
		name        string
		windowState ash.WindowStateType
	}{
		{"Windows are normal", ash.WindowStateNormal},
		{"Windows are maximized", ash.WindowStateMaximized},
	} {
		utils.RunOrFatal(ctx, s, test.name, func(ctx context.Context, s *testing.State) error {
			if err := utils.EnsureSetWindowState(ctx, tconn, utils.SettingsPkg, test.windowState); err != nil {
				return err
			}
			settingsWindowInfo, err := ash.GetARCAppWindowInfo(ctx, tconn, utils.SettingsPkg)
			if err != nil {
				return err
			}

			if err := utils.EnsureSetWindowState(ctx, tconn, utils.TestappPkg, test.windowState); err != nil {
				return err
			}
			wmWindowInfo, err := ash.GetARCAppWindowInfo(ctx, tconn, utils.TestappPkg)
			if err != nil {
				return err
			}

			// Relayout external display and make sure the windows will not move their positions or show black background.
			for _, relayout := range []struct {
				name   string
				offset coords.Point
			}{
				{"Relayout external display on top of internal display", coords.NewPoint(0, -externalDisplayInfo.Bounds.Height)},
				{"Relayout external display on bottom of internal display", coords.NewPoint(0, internalDisplayInfo.Bounds.Height)},
				{"Relayout external display to the left side of internal display", coords.NewPoint(-externalDisplayInfo.Bounds.Width, 0)},
				{"Relayout external display to the right side of internal display", coords.NewPoint(internalDisplayInfo.Bounds.Width, 0)},
			} {
				utils.RunOrFatal(ctx, s, relayout.name, func(ctx context.Context, s *testing.State) error {
					p := display.DisplayProperties{BoundsOriginX: &relayout.offset.X, BoundsOriginY: &relayout.offset.Y}
					if err := display.SetDisplayProperties(ctx, tconn, externalDisplayInfo.ID, p); err != nil {
						return err
					}
					if err := utils.EnsureWindowStable(ctx, tconn, utils.SettingsPkg, settingsWindowInfo); err != nil {
						return err
					}
					return utils.EnsureWindowStable(ctx, tconn, utils.TestappPkg, wmWindowInfo)
				})
			}

			return nil
		})
	}

	return nil
}

func dock14ChangePositionStep6(ctx context.Context, s *testing.State, cr *chrome.Chrome, tconn *chrome.TestConn, a *arc.ARC) error {

	s.Log("Step 6 - On Primary (Built-in displays) open Chrome browser window and drag the browser window onto (Extended) ext- displays")

	// start activity on internal display
	act, err := arc.NewActivityOnDisplay(a, utils.SettingsPkg, utils.SettingsAct, 0)
	if err != nil {
		return errors.Wrap(err, "failed to start activity on internal display")
	}

	// start activity
	if err := act.Start(ctx, tconn); err != nil {
		return err
	}

	// get setting's window
	win, err := ash.GetARCAppWindowInfo(ctx, tconn, utils.SettingsPkg)
	if err != nil {
		return errors.Wrap(err, "failed to get setting window info")
	}

	// set window state to normal
	if _, err := ash.SetWindowState(ctx, tconn, win.ID, ash.WMEventNormal, true); err != nil {
		s.Fatal("Failed to set window state to normal: ", err)
	}

	// retry in 30s
	if err := testing.Poll(ctx, func(c context.Context) error {

		testing.Sleep(ctx, 1*time.Second)

		// get infos
		infos, err := display.GetInfo(ctx, tconn)
		if err != nil {
			return errors.Wrap(err, "failed to get display info")
		}

		if len(infos) < 2 {
			return errors.Errorf("failed to get right num of display, got %d, at least 2: ", len(infos))
		}

		// get setting's window
		win, err := ash.GetARCAppWindowInfo(ctx, tconn, utils.SettingsPkg)
		if err != nil {
			return errors.Wrap(err, "failed to get setting window info")
		}

		// move setting's window to external
		if err := utils.MoveWindowToDisplay(ctx, tconn, win, &infos[1]); err != nil {
			return errors.Wrap(err, "failed to move window to external display")
		}

		return nil
	}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
		return err
	}

	return nil
}
