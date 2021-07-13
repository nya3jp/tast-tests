// Copyright 2021 The Chromium OS Authors. All rights reserved.
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
	"chromiumos/tast/local/chrome/uiauto/pointer"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/power"
	"chromiumos/tast/local/ui"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DragTabInTabletPerf,
		Desc:         "Measures the presentation time of dragging a tab in tablet mode",
		Contacts:     []string{"yichenz@chromium.org", "chromeos-wmp@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Timeout:      5 * time.Minute,
		Params: []testing.Param{{
			Val: "mouse",
		}, {
			Name: "touch",
			Val:  "touch",
		}},
	})
}

func DragTabInTabletPerf(ctx context.Context, s *testing.State) {
	// Ensure display on to record ui performance correctly.
	if err := power.TurnOnDisplay(ctx); err != nil {
		s.Fatal("Failed to turn on display: ", err)
	}

	cr, err := chrome.New(ctx, chrome.EnableFeatures("WebUITabStrip", "WebUITabStripTabDragIntegration"))
	if err != nil {
		s.Fatal("Failed to init: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, true)
	if err != nil {
		s.Fatal("Failed to ensure in tablet mode: ", err)
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

	var pc pointer.Context
	pc, err = pointer.NewTouch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to set up the touch context: ", err)
	}
	defer pc.Close()

	info, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get the primary display info: ", err)
	}
	snapRightPoint := coords.NewPoint(info.WorkArea.Right()-1, info.WorkArea.CenterY())
	snapLeftPoint := coords.NewPoint(info.WorkArea.Left+1, info.WorkArea.CenterY())
	workAreaCenterPoint := info.WorkArea.CenterPoint()

	tabStripButton := nodewith.Role(role.Button).ClassName("WebUITabCounterButton").First()
	if err := ac.LeftClick(tabStripButton)(ctx); err != nil {
		s.Fatal("Failed to click the tab strip button")
	}

	firstTab := nodewith.Role(role.Tab).First()
	firstTabLocation, _ := ac.Location(ctx, firstTab)
	tabList := nodewith.Role(role.TabList).First()
	tabListLocation, _ := ac.Location(ctx, tabList)
	pv := perfutil.RunMultiple(ctx, s, cr, perfutil.RunAndWaitAll(tconn, func(ctx context.Context) error {
		pointerType := s.Param().(string)
		switch pointerType {
		case "touch":
			{
				if err := uiauto.Combine("touch drag and move a tab",
					// Drag the first tab in the tab strip around the work area and back to the tab strip.
					pc.Drag(firstTabLocation.CenterPoint(), ac.Sleep(time.Second),
						pc.DragTo(snapRightPoint, 3*time.Second), pc.DragTo(snapLeftPoint, 3*time.Second),
						pc.DragTo(workAreaCenterPoint, 3*time.Second),
						pc.DragTo(tabListLocation.CenterPoint(), 3*time.Second)),
					// Sleep to ensure that the next run performs correctly.
					ac.Sleep(time.Second),
				)(ctx); err != nil {
					return errors.Wrap(err, "failed to touch drag the tab")
				}
			}
		case "mouse":
			{
				if err := uiauto.Combine("mouse drag and move a tab",
					// Drag the first tab in the tab strip.
					ac.MouseMoveTo(firstTab, 0),
					// Drag in tablet mode starts with a long press.
					mouse.Press(tconn, mouse.LeftButton),
					ac.Sleep(time.Second),
					// Drag tab around work area.
					mouse.Move(tconn, snapRightPoint, 3*time.Second),
					mouse.Move(tconn, snapLeftPoint, 3*time.Second),
					mouse.Move(tconn, workAreaCenterPoint, 3*time.Second),
					// Snap the tab back to the tab strip.
					ac.MouseMoveTo(tabList, 3*time.Second),
					mouse.Release(tconn, mouse.LeftButton),
					// Sleep to ensure that the next run performs correctly.
					ac.Sleep(time.Second),
				)(ctx); err != nil {
					return errors.Wrap(err, "failed to mouse drag the tab")
				}
			}
		default:
			return nil
		}

		return nil
	},
		"Ash.TabDrag.PresentationTime.TabletMode",
		"Ash.TabDrag.PresentationTime.MaxLatency.TabletMode"),
		perfutil.StoreLatency)

	if err := pv.Save(ctx, s.OutDir()); err != nil {
		s.Error("Failed to save perf data: ", err)
	}
}
