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
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SystemTrayPerf,
		Desc:         "Measures animation smoothness of system tray animations",
		Contacts:     []string{"amehfooz@chromium.org", "tengs@chromium.org", "chromeos-wmp@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Pre:          chrome.LoggedIn(),
		Timeout:      3 * time.Minute,
	})
}

func SystemTrayPerf(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	defer ui.WaitForLocationChangeCompleted(ctx, tconn)

	if err := cpu.WaitUntilIdle(ctx); err != nil {
		s.Fatal("Failed to wait: ", err)
	}

	// Find and click the StatusArea via UI. Clicking it opens the Ubertray.
	params := ui.FindParams{
		ClassName: "ash/StatusAreaWidgetDelegate",
	}
	statusArea, err := ui.FindWithTimeout(ctx, tconn, params, 10*time.Second)
	if err != nil {
		s.Fatal("Failed to find the status area (time, battery, etc.): ", err)
	}
	defer statusArea.Release(ctx)

	if err := statusArea.LeftClick(ctx); err != nil {
		s.Fatal("Failed to click status area: ", err)
	}

	// Confirm that the system tray is open by checking for the "CollapseButton".
	params = ui.FindParams{
		ClassName: "CollapseButton",
	}
	if err := ui.WaitUntilExists(ctx, tconn, params, 10*time.Second); err != nil {
		s.Fatal("Waiting for system tray open failed: ", err)
	}

	// Find the collapse button view bounds.
	collapseButton, err := ui.Find(ctx, tconn, ui.FindParams{ClassName: "CollapseButton"})
	if err != nil {
		s.Fatal("Failed to find the collapse button: ", err)
	}
	defer collapseButton.Release(ctx)

	// Toggle the collapsed state of the system tray.
	pv := perfutil.RunMultiple(ctx, s, cr, perfutil.RunAndWaitAll(tconn, func() error {
		if err := collapseButton.LeftClick(ctx); err != nil {
			return errors.Wrap(err, "failed to click collapse button")
		}
		if err := ui.WaitForLocationChangeCompleted(ctx, tconn); err != nil {
			return errors.Wrap(err, "failed to wait")
		}
		if err := collapseButton.LeftClick(ctx); err != nil {
			return errors.Wrap(err, "failed to click the collapse button")
		}
		return ui.WaitForLocationChangeCompleted(ctx, tconn)
	},
		"ChromeOS.SystemTray.AnimationSmoothness.TransitionToCollapsed",
		"ChromeOS.SystemTray.AnimationSmoothness.TransitionToExpanded"),
		perfutil.StoreSmoothness)

	if err := pv.Save(s.OutDir()); err != nil {
		s.Fatal("Failed saving perf data: ", err)
	}
}
