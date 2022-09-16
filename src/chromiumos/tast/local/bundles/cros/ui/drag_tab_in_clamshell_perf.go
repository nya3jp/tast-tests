// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/errors"
	uiperf "chromiumos/tast/local/bundles/cros/ui/perf"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/perfutil"
	"chromiumos/tast/local/power"
	"chromiumos/tast/local/ui"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DragTabInClamshellPerf,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Measures the presentation time of dragging a tab in clamshell mode",
		Contacts:     []string{"yichenz@chromium.org", "chromeos-wmp@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Fixture:      "chromeLoggedIn",
		Timeout:      5 * time.Minute,
	})
}

func DragTabInClamshellPerf(ctx context.Context, s *testing.State) {
	// Ensure display on to record ui performance correctly.
	if err := power.TurnOnDisplay(ctx); err != nil {
		s.Fatal("Failed to turn on display: ", err)
	}

	cr := s.FixtValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		s.Fatal("Failed to ensure in clamshell mode: ", err)
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

	pv := perfutil.RunMultiple(ctx, cr.Browser(), uiperf.Run(s, perfutil.RunAndWaitAll(tconn, func(ctx context.Context) error {
		return uiauto.Combine("drag and move a tab",
			mouse.Drag(tconn, start, end, time.Second),
			ac.Retry(10, checkWindowsNum(ctx, tconn, 2)),
			uiauto.Sleep(time.Second),
			mouse.Drag(tconn, end, start, time.Second),
			ac.Retry(10, checkWindowsNum(ctx, tconn, 1)),
			uiauto.Sleep(time.Second),
		)(ctx)
	},
		"Ash.TabDrag.PresentationTime.ClamshellMode",
		"Ash.TabDrag.PresentationTime.MaxLatency.ClamshellMode")),
		perfutil.StoreLatency)
	if err := pv.Save(ctx, s.OutDir()); err != nil {
		s.Error("Failed to save perf data: ", err)
	}
}

func checkWindowsNum(ctx context.Context, tconn *chrome.TestConn, num int) action.Action {
	return func(ctx context.Context) error {
		ws, err := ash.GetAllWindows(ctx, tconn)
		if err != nil {
			return errors.Wrap(err, "failed to obtain the window list")
		}
		if num != len(ws) {
			return errors.Wrapf(err, "failed to verify the number of windows, got %v, want %v", num, len(ws))
		}
		return nil
	}
}
