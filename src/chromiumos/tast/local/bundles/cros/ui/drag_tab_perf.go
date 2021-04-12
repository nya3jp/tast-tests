// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/ui/perfutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/power"
	"chromiumos/tast/local/ui"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DragTabPerf,
		Desc:         "Measures the presentation time of dragging a tab",
		Contacts:     []string{"yichenz@chromium.org", "chromeos-wmp@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Params: []testing.Param{
			{
				Name:    "clamshell_mode",
				Val:     false,
				Fixture: "chromeLoggedIn",
			},
			{
				Name:              "tablet_mode",
				Val:               true,
				ExtraSoftwareDeps: []string{"tablet_mode"},
			},
		},
		Timeout: 5 * time.Minute,
	})
}

func DragTabPerf(ctx context.Context, s *testing.State) {
	// Ensure display on to record ui performance correctly.
	if err := power.TurnOnDisplay(ctx); err != nil {
		s.Fatal("Failed to turn on display: ", err)
	}

	inTabletMode := s.Param().(bool)

	var cr *chrome.Chrome
	if inTabletMode {
		var err error
		if cr, err = chrome.New(ctx, chrome.EnableFeatures("WebUITabStrip", "WebUITabStripTabDragIntegration")); err != nil {
			s.Fatal("Failed to init: ", err)
		}
		defer cr.Close(ctx)
	} else {
		cr = s.FixtValue().(*chrome.Chrome)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, inTabletMode)
	if err != nil {
		s.Fatal("Failed to ensure in clamshell/tablet mode: ", err)
	}
	defer cleanup(ctx)

	for i := 0; i < 2; i++ {
		conn, err := cr.NewConn(ctx, ui.PerftestURL)
		if err != nil {
			s.Fatalf("Failed to open %d-th tab: %v", i, err)
		}
		if err := conn.Close(); err != nil {
			s.Fatalf("Failed to close the connection to %d-th tab: %v", i, err)
		}
	}

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	ac := uiauto.New(tconn)

	if inTabletMode {
		info, err := display.GetPrimaryInfo(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to get the primary display info: ", err)
		}
		snapRightPoint := coords.NewPoint(info.WorkArea.Right()-1, info.WorkArea.CenterY())
		snapLeftPoint := coords.NewPoint(info.WorkArea.Left+1, info.WorkArea.CenterY())
		workAreaCenterPoint := info.WorkArea.CenterPoint()

		if err := ac.LeftClick(nodewith.Role(role.Button).ClassName("WebUITabCounterButton").First())(ctx); err != nil {
			s.Fatal("Failed to click the tab strip button")
		}

		pv := perfutil.RunMultiple(ctx, s, cr, perfutil.RunAndWaitAll(tconn, func(ctx context.Context) error {
			if err := uiauto.Combine("Drag tab",
				// Drag the first tab in the tab strip.
				ac.MouseMoveTo(nodewith.Role(role.Tab).First(), 0),
				// Drag in tablet mode starts with a long press.
				mouse.Press(tconn, mouse.LeftButton),
				ac.Sleep(time.Second),
				// Drag tab around work area.
				mouse.Move(tconn, snapRightPoint, 3*time.Second),
				mouse.Move(tconn, snapLeftPoint, 3*time.Second),
				mouse.Move(tconn, workAreaCenterPoint, 3*time.Second),
				// Snap the tab back to the tab strip.
				ac.MouseMoveTo(nodewith.Role(role.TabList).First(), 3*time.Second),
				mouse.Release(tconn, mouse.LeftButton),
				// Sleep to ensure that the next run performs correctly.
				ac.Sleep(time.Second),
			)(ctx); err != nil {
				return errors.Wrap(err, "failed to drag the tab")
			}
			return nil
		},
			"Ash.TabDrag.PresentationTime.TabletMode",
			"Ash.TabDrag.PresentationTime.MaxLatency.TabletMode"),
			perfutil.StoreLatency)

		if err := pv.Save(ctx, s.OutDir()); err != nil {
			s.Error("Failed saving perf data: ", err)
		}
	} else {
		ws, err := ash.GetAllWindows(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to obtain the window list: ", err)
		}
		id0 := ws[0].ID
		if err := ash.SetWindowStateAndWait(ctx, tconn, id0, ash.WindowStateNormal); err != nil {
			s.Fatal("Failed to set the window state to normal: ", err)
		}
		if err := ash.WaitWindowFinishAnimating(ctx, tconn, id0); err != nil {
			s.Fatal("Failed to wait for top window animation: ", err)
		}
		w0, err := ash.GetWindow(ctx, tconn, id0)
		if err != nil {
			s.Fatal("Failed to get the window: ", err)
		}
		if w0.State != ash.WindowStateNormal {
			s.Fatalf("Wrong window state: expected Normal, got %s", w0.State)
		}
		bounds := w0.BoundsInRoot
		end := bounds.CenterPoint()

		// Find tabs.
		tabParam := nodewith.Role(role.Tab).ClassName("Tab")
		tabs, err := ac.NodesInfo(ctx, tabParam)
		if err != nil {
			s.Fatal("Failed to find tabs: ", err)
		}
		if len(tabs) != 2 {
			s.Fatalf("Expected 2 tabs, only found %v tab(s)", len(tabs))
		}
		tabRect, err := ac.Location(ctx, tabParam.First())
		if err != nil {
			s.Fatal("Failed to get the location of the tab: ", err)
		}
		start := tabRect.CenterPoint()

		pv := perfutil.RunMultiple(ctx, s, cr, perfutil.RunAndWaitAll(tconn, func(ctx context.Context) error {
			if err := mouse.Drag(tconn, start, end, time.Second)(ctx); err != nil {
				return errors.Wrap(err, "failed to drag the end of point")
			}
			if err := testing.Poll(ctx, func(ctx context.Context) error {
				// Expecting 2 windows.
				return checkWindowsNum(ctx, tconn, 2)
			}, &testing.PollOptions{Timeout: 10 * time.Second, Interval: time.Second}); err != nil {
				return errors.Wrap(err, "failed to get expected windows")
			}

			// Sleep to ensure post drag finishes so that the window is ready for the next drag.
			if err := testing.Sleep(ctx, time.Second); err != nil {
				return errors.Wrap(err, "failed to sleep")
			}

			if err := mouse.Drag(tconn, end, start, time.Second)(ctx); err != nil {
				return errors.Wrap(err, "failed to drag back to the start point")
			}
			if err := testing.Poll(ctx, func(ctx context.Context) error {
				// Expecting 1 window.
				return checkWindowsNum(ctx, tconn, 1)
			}, &testing.PollOptions{Timeout: 10 * time.Second, Interval: time.Second}); err != nil {
				return errors.Wrap(err, "failed to get expected windows")
			}
			// Sleep to ensure that the next run performs correctly.
			return testing.Sleep(ctx, time.Second)
		},
			"Ash.TabDrag.PresentationTime.ClamshellMode",
			"Ash.TabDrag.PresentationTime.MaxLatency.ClamshellMode"),
			perfutil.StoreLatency)

		if err := pv.Save(ctx, s.OutDir()); err != nil {
			s.Error("Failed saving perf data: ", err)
		}
	}
}

func checkWindowsNum(ctx context.Context, tconn *chrome.TestConn, num int) error {
	ws, err := ash.GetAllWindows(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to obtain the window list")
	}
	if num != len(ws) {
		return errors.Wrapf(err, "expected %v windows, got %v windows", num, len(ws))
	}
	return nil
}
