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
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/perfutil"
	"chromiumos/tast/local/power"
	"chromiumos/tast/local/ui"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         OverviewScrollPerf,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Measures the presentation time of scrolling the overview grid in tablet mode",
		Contacts:     []string{"sammiequon@chromium.org", "chromeos-wmp@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Fixture:      "chromeLoggedIn",
	})
}

func OverviewScrollPerf(ctx context.Context, s *testing.State) {
	// Ensure display on to record ui performance correctly.
	if err := power.TurnOnDisplay(ctx); err != nil {
		s.Fatal("Failed to turn on display: ", err)
	}

	cr := s.FixtValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	orientation, err := display.GetOrientation(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to obtain the display rotation: ", err)
	}

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, true)
	if err != nil {
		s.Fatal("Failed to ensure in tablet mode: ", err)
	}
	defer cleanup(ctx)

	// Prepare the touch screen as this test requires touch scroll events.
	tsew, err := input.Touchscreen(ctx)
	if err != nil {
		s.Fatal("Failed to create touch screen event writer: ", err)
	}
	defer tsew.Close()
	if err = tsew.SetRotation(-orientation.Angle); err != nil {
		s.Fatal("Failed to set rotation: ", err)
	}

	stw, err := tsew.NewSingleTouchWriter()
	if err != nil {
		s.Fatal("Failed to create single touch writer: ", err)
	}
	defer stw.Close()

	// Use a total of 16 windows for this test, so that scrolling can happen.
	const numWindows = 16
	if err := ash.CreateWindows(ctx, tconn, cr, ui.PerftestURL, numWindows); err != nil {
		s.Fatal("Failed to open browser windows: ", err)
	}

	if err = ash.SetOverviewModeAndWait(ctx, tconn, true); err != nil {
		s.Fatal("It does not appear to be in the overview mode: ", err)
	}
	defer ash.SetOverviewModeAndWait(ctx, tconn, false)

	pv := perfutil.RunMultiple(ctx, cr.Browser(), uiperf.Run(s, perfutil.RunAndWaitAll(tconn, func(ctx context.Context) error {
		// Scroll from the top right of the screen to the top middle (1/4 of the
		// screen width). The destination position should match with the next swipe
		// to make the same amount of scrolling.
		if err := stw.Swipe(ctx, tsew.Width()-10, 10, tsew.Width()/4, 10, 500*time.Millisecond); err != nil {
			return errors.Wrap(err, "failed to execute a swipe gesture")
		}

		if err := stw.End(); err != nil {
			return errors.Wrap(err, "failed to finish the swipe gesture")
		}

		// Scroll back from the top middle to the top right so that the test returns
		// back to the original status. Note that this can't be starting from the
		// top left, since it can be recognized as another gesture (back gesture).
		if err := stw.Swipe(ctx, tsew.Width()/4, 10, tsew.Width()-10, 10, 500*time.Millisecond); err != nil {
			return errors.Wrap(err, "failed to execute a swipe gesture")
		}
		if err := stw.End(); err != nil {
			return errors.Wrap(err, "failed to finish the swipe gesture")
		}

		return nil
	}, "Ash.Overview.Scroll.PresentationTime.TabletMode")), perfutil.StoreLatency)

	if err = ash.SetOverviewModeAndWait(ctx, tconn, false); err != nil {
		s.Fatal("It does not appear to be in the overview mode: ", err)
	}

	if err := pv.Save(ctx, s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}
