// Copyright 2019 The Chromium OS Authors. All rights reserved.
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
	"chromiumos/tast/local/ui"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         TabletTransitionPerf,
		Desc:         "Measures the animation smoothess of animating to and from tablet mode",
		Contacts:     []string{"sammiequon@chromium.org", "chromeos-wmp@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome", "tablet_mode"},
		Fixture:      "chromeLoggedIn",
		Timeout:      3 * time.Minute,
	})
}

func TabletTransitionPerf(ctx context.Context, s *testing.State) {
	// Ensure display on to record ui performance correctly.
	if err := perfutil.EnsureDisplayOn(ctx); err != nil {
		s.Fatal("Failed to turn on display: ", err)
	}

	cr := s.FixtValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	const numWindows = 8
	if err := ash.CreateWindows(ctx, tconn, cr, ui.PerftestURL, numWindows); err != nil {
		s.Fatal("Failed to create windows: ", err)
	}

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		s.Fatal("Failed to ensure in clamshell mode: ", err)
	}
	defer cleanup(ctx)

	// The top window (first window in the list returned by |ash.GetAllWindow|) needs to be normal window state otherwise no animation will occur.
	windows, err := ash.GetAllWindows(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to obtain the window list: ", err)
	}

	if err := ash.SetWindowStateAndWait(ctx, tconn, windows[0].ID, ash.WindowStateNormal); err != nil {
		s.Fatalf("Failed to set the window (%d): %v", windows[0].ID, err)
	}

	pv := perfutil.RunMultiple(ctx, s, cr, perfutil.RunAndWaitAll(tconn, func(ctx context.Context) error {
		if err := ash.SetTabletModeEnabled(ctx, tconn, true); err != nil {
			return errors.Wrap(err, "failed to enable tablet mode")
		}

		// Wait for the top window to finish animating before changing states.
		if err := ash.WaitWindowFinishAnimating(ctx, tconn, windows[0].ID); err != nil {
			return errors.Wrap(err, "failed to wait for top window animation")
		}

		if err := ash.SetTabletModeEnabled(ctx, tconn, false); err != nil {
			return errors.Wrap(err, "failed to disable tablet mode")
		}

		if err := ash.WaitWindowFinishAnimating(ctx, tconn, windows[0].ID); err != nil {
			return errors.Wrap(err, "failed to wait for top window animation")
		}
		return nil
	},
		"Ash.TabletMode.AnimationSmoothness.Enter",
		"Ash.TabletMode.AnimationSmoothness.Exit"),
		perfutil.StoreSmoothness)

	if err := pv.Save(ctx, s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}
