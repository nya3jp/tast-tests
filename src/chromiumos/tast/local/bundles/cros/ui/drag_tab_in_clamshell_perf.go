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
	chromeui "chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/chrome/ui/mouse"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/local/ui"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DragTabInClamshellPerf,
		Desc:         "Measures the presentation time of dragging a tab in clamshell mode",
		Contacts:     []string{"yichenz@chromium.org", "chromeos-wmp@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Pre:          chrome.LoggedIn(),
	})
}

func DragTabInClamshellPerf(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
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

	ws, err := ash.GetAllWindows(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to obtain the window list: ", err)
	}
	id0 := ws[0].ID
	if _, err := ash.SetWindowState(ctx, tconn, id0, ash.WMEventNormal); err != nil {
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
	tabs, err := chromeui.FindAll(ctx, tconn, chromeui.FindParams{Role: chromeui.RoleTypeTab, ClassName: "Tab"})
	if err != nil {
		s.Fatal("Failed to find tabs: ", err)
	}
	defer tabs.Release(ctx)
	if len(tabs) != 2 {
		s.Fatalf("Expected 2 tabs, only found %v tab(s)", len(tabs))
	}
	start := tabs[0].Location.CenterPoint()

	// Stabilize CPU usage.
	if err := cpu.WaitUntilIdle(ctx); err != nil {
		s.Error("Failed to wait for system UI to be stabilized: ", err)
	}

	pv := perfutil.RunMultiple(ctx, s, cr, perfutil.RunAndWaitAll(tconn, func() error {
		if err := mouse.Drag(ctx, tconn, start, end, 2*time.Second); err != nil {
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

		if err := mouse.Drag(ctx, tconn, end, start, 2*time.Second); err != nil {
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
		"Ash.WorkspaceWindowResizer.TabDragging.PresentationTime.ClamshellMode",
		"Ash.WorkspaceWindowResizer.TabDragging.PresentationTime.MaxLatency.ClamshellMode"),
		perfutil.StoreLatency)

	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
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
