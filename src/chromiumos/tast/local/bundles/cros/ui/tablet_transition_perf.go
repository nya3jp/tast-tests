// Copyright 2019 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	uiperf "chromiumos/tast/local/bundles/cros/ui/perf"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/perfutil"
	"chromiumos/tast/local/power"
	"chromiumos/tast/local/ui"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         TabletTransitionPerf,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Measures the animation smoothess of animating to and from tablet mode",
		Contacts:     []string{"sammiequon@chromium.org", "chromeos-wmp@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Timeout:      3 * time.Minute,
		Params: []testing.Param{
			{
				Name: "classic",
				Val:  false,
			},
			{
				Name: "webui",
				Val:  true,
			},
		},
	})
}

func TabletTransitionPerf(ctx context.Context, s *testing.State) {
	// Ensure display on to record ui performance correctly.
	if err := power.TurnOnDisplay(ctx); err != nil {
		s.Fatal("Failed to turn on display: ", err)
	}

	var opt chrome.Option
	if s.Param().(bool) {
		opt = chrome.EnableFeatures("WebUITabStrip")
	} else {
		opt = chrome.DisableFeatures("WebUITabStrip")
	}
	cr, err := chrome.New(ctx, opt)
	if err != nil {
		s.Fatal("Failed to start chrome: ", err)
	}
	defer cr.Close(ctx)

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

	pv := perfutil.RunMultiple(ctx, cr.Browser(), uiperf.Run(s, perfutil.RunAndWaitAll(tconn, func(ctx context.Context) error {
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
		"Ash.TabletMode.AnimationSmoothness.Exit")),
		perfutil.StoreSmoothness)

	if err := pv.Save(ctx, s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}
