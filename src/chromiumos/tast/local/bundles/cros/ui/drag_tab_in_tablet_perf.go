// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/ui/perfutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/lacros/lacrosfixt"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/pointer"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/power"
	"chromiumos/tast/local/ui"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type pointerTypeParam int

const (
	pointerMouse pointerTypeParam = iota
	pointerTouch
)

type pointerTestParam struct {
	pt pointerTypeParam
	bt browser.Type
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         DragTabInTabletPerf,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Measures the presentation time of dragging a tab in tablet mode",
		Contacts:     []string{"yichenz@chromium.org", "chromeos-wmp@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Timeout:      8 * time.Minute,
		Params: []testing.Param{{
			Val: pointerTestParam{pt: pointerMouse, bt: browser.TypeAsh},
		}, {
			Name: "touch",
			Val:  pointerTestParam{pt: pointerTouch, bt: browser.TypeAsh},
		}, {
			Name:              "lacros",
			ExtraSoftwareDeps: []string{"lacros"},
			Val:               pointerTestParam{pt: pointerMouse, bt: browser.TypeLacros},
		}, {
			Name:              "touch_lacros",
			ExtraSoftwareDeps: []string{"lacros"},
			Val:               pointerTestParam{pt: pointerTouch, bt: browser.TypeLacros},
		}},
	})
}

func DragTabInTabletPerf(ctx context.Context, s *testing.State) {
	// Reserve a few seconds for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(cleanupCtx, 10*time.Second)
	defer cancel()

	// Ensure display on to record ui performance correctly.
	if err := power.TurnOnDisplay(ctx); err != nil {
		s.Fatal("Failed to turn on display: ", err)
	}

	// Set up the browser. Open the first window now, then the second one later.
	const numWindows = 2
	const url = ui.PerftestURL
	bt := s.Param().(pointerTestParam).bt
	conn, cr, br, closeBrowser, err := browserfixt.SetUpWithNewChromeAtURL(ctx, bt, url, lacrosfixt.NewConfig(),
		chrome.EnableFeatures("WebUITabStrip", "WebUITabStripTabDragIntegration"))
	if err != nil {
		s.Fatal("Failed to open the browser: ", err)
	}
	if err := conn.Close(); err != nil {
		s.Fatal("Failed to close the connection to 0-th tab: ", err)
	}
	defer cr.Close(cleanupCtx)
	defer closeBrowser(cleanupCtx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, true)
	if err != nil {
		s.Fatal("Failed to ensure in tablet mode: ", err)
	}
	defer cleanup(cleanupCtx)

	// Open the rest of the windows.
	for i := 1; i < numWindows; i++ {
		conn, err := br.NewConn(ctx, url)
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
	pt := s.Param().(pointerTestParam).pt
	if pt == pointerTouch {
		pc, err = pointer.NewTouch(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to set up the touch context: ", err)
		}
	} else {
		pc = pointer.NewMouse(tconn)
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
	pv := perfutil.RunMultiple(ctx, s, cr.Browser(), perfutil.RunAndWaitAll(tconn, func(ctx context.Context) error {
		if err := uiauto.Combine("drag and move a tab",
			// Drag the first tab in the tab strip around work area, then snap back to the tab strip.
			pc.Drag(firstTabLocation.CenterPoint(),
				uiauto.Sleep(time.Second),
				pc.DragTo(snapRightPoint, 3*time.Second),
				pc.DragTo(snapLeftPoint, 3*time.Second),
				pc.DragTo(workAreaCenterPoint, 3*time.Second),
				pc.DragTo(tabListLocation.CenterPoint(), 3*time.Second)),
			// Sleep to ensure that the next run performs correctly.
			uiauto.Sleep(time.Second),
		)(ctx); err != nil {
			return errors.Wrap(err, "failed to drag the tab")
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
