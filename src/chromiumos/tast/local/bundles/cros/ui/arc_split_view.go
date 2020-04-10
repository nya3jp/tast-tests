// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/ui"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ArcSplitView,
		Desc:         "Tests starting split view with an ARC window",
		Contacts:     []string{"amusbach@chromium.org", "chromeos-wmp@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"android_all", "chrome"},
		Timeout:      time.Minute,
		Params: []testing.Param{
			{
				Name: "clamshell_mode",
				Val:  false,
			},
			{
				Name:              "tablet_mode",
				ExtraSoftwareDeps: []string{"tablet_mode"},
				Val:               true,
			},
		},
	})
}

func ArcSplitView(ctx context.Context, s *testing.State) {
	// Enables DragToSnapInClamshellMode when testing clamshell split view.
	// TODO: When the feature is fully launched, use s.PreValue().(arc.PreData).
	tabletMode := s.Param().(bool)
	var cr *chrome.Chrome
	var err error
	if tabletMode {
		cr, err = chrome.New(ctx, chrome.ARCEnabled())
	} else {
		cr, err = chrome.New(ctx, chrome.ARCEnabled(), chrome.ExtraArgs("--enable-features=DragToSnapInClamshellMode"))
	}
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)
	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, tabletMode)
	if err != nil {
		if tabletMode {
			s.Fatal("Failed to ensure in tablet mode: ", err)
		} else {
			s.Fatal("Failed to ensure in clamshell mode: ", err)
		}
	}
	defer cleanup(ctx)

	tew, err := input.Touchscreen(ctx)
	if err != nil {
		s.Fatal("Failed to access to the touch screen: ", err)
	}
	defer tew.Close()

	// Ensures landscape orientation so this test can assume that windows snap on
	// the left and right. Windows snap on the top and bottom in portrait-oriented
	// tablet mode. They snap on the left and right in portrait-oriented clamshell
	// mode, but there are active (although contentious) proposals to change that.
	orientation, err := display.GetOrientation(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to obtain the orientation info: ", err)
	}
	rotation := -orientation.Angle
	if orientation.Type == display.OrientationPortraitPrimary {
		info, err := display.GetInternalInfo(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to obtain internal display info: ", err)
		}
		if err = display.SetDisplayRotationSync(ctx, tconn, info.ID, display.Rotate90); err != nil {
			s.Fatal("Failed to rotate display: ", err)
		}
		defer display.SetDisplayRotationSync(ctx, tconn, info.ID, display.Rotate0)
		rotation += 90
	}
	tew.SetRotation(rotation)

	info, err := display.GetInternalInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get the internal display info: ", err)
	}
	tcc := tew.NewTouchCoordConverter(info.Bounds.Size())

	stw, err := tew.NewSingleTouchWriter()
	if err != nil {
		s.Fatal("Failed to create a single touch writer: ", err)
	}

	conns, err := ash.CreateWindows(ctx, cr, ui.PerftestURL, 1)
	if err != nil {
		s.Fatal("Failed to open a non-ARC window: ", err)
	}
	conns.Close()

	act, err := arc.NewActivity(a, "com.android.settings", ".Settings")
	if err != nil {
		s.Fatal("Failed to create an ARC activity: ", err)
	}
	defer act.Close()
	if err := act.Start(ctx, tconn); err != nil {
		s.Fatal("Failed to start the Settings activity: ", err)
	}
	defer act.Stop(ctx)

	if err := ash.SetOverviewModeAndWait(ctx, tconn, true); err != nil {
		s.Fatal("Failed to enter overview: ", err)
	}
	arcWin, err := ash.FindFirstWindowInOverview(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to find the ARC window in the overview grid: ", err)
	}
	centerX, centerY := tcc.ConvertLocation(arcWin.OverviewInfo.Bounds.CenterPoint())
	if tabletMode {
		if err := stw.LongPressAt(ctx, centerX, centerY); err != nil {
			s.Fatal("Failed to long-press to start dragging the ARC window: ", err)
		}
	}
	if err := stw.Swipe(ctx, centerX, centerY, 0, tew.Height()/2, time.Second); err != nil {
		s.Fatal("Failed to swipe for snapping the ARC window: ", err)
	}
	if err := stw.End(); err != nil {
		s.Fatal("Failed to end the swipe: ", err)
	}
	if err := ash.WaitForCondition(ctx, tconn, func(w *ash.Window) bool {
		return w.ID == arcWin.ID && !w.IsAnimating && w.State == ash.WindowStateLeftSnapped
	}, &testing.PollOptions{Timeout: 15 * time.Second}); err != nil {
		s.Fatal("Failed to wait for the ARC window to be left-snapped: ", err)
	}

	// Check for https://b.corp.google.com/issues/143499564.
	if _, err := ash.FindFirstWindowInOverview(ctx, tconn); err != nil {
		s.Fatal("Failed to find the non-ARC window in the overview grid: ", err)
	}
}
