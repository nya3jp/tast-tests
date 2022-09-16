// Copyright 2020 The ChromiumOS Authors
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
	"chromiumos/tast/local/ui"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         HotseatDrag,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Measures the presentation time of dragging the hotseat in tablet mode",
		Contacts:     []string{"newcomer@chromium.org", "manucornet@chromium.org", "cros-shelf-prod-notifications@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Fixture:      "chromeLoggedIn",
	})
}

func HotseatDrag(ctx context.Context, s *testing.State) {
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
	tsw, err := input.Touchscreen(ctx)
	if err != nil {
		s.Fatal("Failed to create touch screen event writer: ", err)
	}
	defer tsw.Close()

	if err = tsw.SetRotation(-orientation.Angle); err != nil {
		s.Fatal("Failed to set rotation: ", err)
	}

	stw, err := tsw.NewSingleTouchWriter()
	if err != nil {
		s.Fatal("Failed to create single touch writer: ", err)
	}
	defer stw.Close()

	const numWindows = 1
	if err := ash.CreateWindows(ctx, tconn, cr, ui.PerftestURL, numWindows); err != nil {
		s.Fatal("Failed to open browser windows: ", err)
	}

	pv := perfutil.RunMultiple(ctx, cr.Browser(), uiperf.Run(s, perfutil.RunAndWaitAll(tconn, func(ctx context.Context) error {
		ws, err := ash.GetAllWindows(ctx, tconn)
		if err != nil || len(ws) == 0 {
			s.Fatal("Failed to obtain the window list: ", err)
		}

		startX := tsw.Width() / 2
		startY := tsw.Height() - 1

		endX := tsw.Width() / 2
		// Scroll 1/4th of the screen to guarantee the hotseat is dragged the full
		// amount.
		endY := tsw.Height() * 3 / 4

		if err := stw.Swipe(ctx, startX, startY, endX, endY, time.Second); err != nil {
			return errors.Wrap(err, "failed to execute a swipe gesture")
		}

		if err := stw.End(); err != nil {
			return errors.Wrap(err, "failed to finish the swipe gesture")
		}
		if err := ash.WaitWindowFinishAnimating(ctx, tconn, ws[0].ID); err != nil {
			return errors.Wrap(err, "failed to wait")
		}

		return ash.SetOverviewModeAndWait(ctx, tconn, false)
	},
		"Ash.HotseatTransition.Drag.PresentationTime",
		"Ash.HotseatTransition.Drag.PresentationTime.MaxLatency")),
		perfutil.StoreLatency)

	if err := pv.Save(ctx, s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}
