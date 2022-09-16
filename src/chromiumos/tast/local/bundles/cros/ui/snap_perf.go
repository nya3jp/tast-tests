// Copyright 2019 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	uiperf "chromiumos/tast/local/bundles/cros/ui/perf"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/perfutil"
	"chromiumos/tast/local/power"
	"chromiumos/tast/local/ui"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SnapPerf,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Measures the animation smoothess of snapping windows in clamshell mode",
		Contacts:     []string{"sammiequon@chromium.org", "chromeos-wmp@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Params: []testing.Param{{
			Fixture: "chromeLoggedIn",
			Val:     browser.TypeAsh,
		}, {
			Name:              "lacros",
			Fixture:           "lacros",
			ExtraSoftwareDeps: []string{"lacros"},
			Val:               browser.TypeLacros,
		}},
	})
}

func SnapPerf(ctx context.Context, s *testing.State) {
	// Reserve a few seconds for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(cleanupCtx, 10*time.Second)
	defer cancel()

	// Ensure display on to record ui performance correctly.
	if err := power.TurnOnDisplay(ctx); err != nil {
		s.Fatal("Failed to turn on display: ", err)
	}

	// Set up the browser.
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	conn, _, closeBrowser, err := browserfixt.SetUpWithURL(ctx, cr, s.Param().(browser.Type), ui.PerftestURL)
	if err != nil {
		s.Fatal("Failed to open a new connection: ", err)
	}
	defer closeBrowser(cleanupCtx)
	defer conn.Close()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		s.Fatal("Failed to ensure in clamshell mode: ", err)
	}
	defer cleanup(cleanupCtx)

	window, err := ash.FindWindow(ctx, tconn, func(w *ash.Window) bool { return true })
	if err != nil {
		s.Fatal("Failed to obtain the window list: ", err)
	}
	pv := perfutil.RunMultiple(ctx, cr.Browser(), uiperf.Run(s, perfutil.RunAndWaitAll(tconn, func(ctx context.Context) error {
		// Snap the window to the left.
		if err := ash.SetWindowStateAndWait(ctx, tconn, window.ID, ash.WindowStateLeftSnapped); err != nil {
			return err
		}

		// Restore the normal state bounds, as no animation stats will be logged if the window size does not change.
		if err := ash.SetWindowStateAndWait(ctx, tconn, window.ID, ash.WindowStateNormal); err != nil {
			return err
		}

		// Snap the window to the right.
		if err := ash.SetWindowStateAndWait(ctx, tconn, window.ID, ash.WindowStateRightSnapped); err != nil {
			return err
		}

		// Restore the normal state bounds, as no animation stats will be logged if the window size does not change.
		if err := ash.SetWindowStateAndWait(ctx, tconn, window.ID, ash.WindowStateNormal); err != nil {
			return err
		}

		return nil
	},
		"Ash.Window.AnimationSmoothness.Snap")), perfutil.StoreSmoothness)

	if err := pv.Save(ctx, s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}
