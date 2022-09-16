// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	uiperf "chromiumos/tast/local/bundles/cros/ui/perf"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/perfutil"
	"chromiumos/tast/local/power"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DesksTrackpadSwipePerf,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Measures the performance of using the trackpad to change desks",
		Contacts:     []string{"afakhry@chromium.org", "sammiequon@chromium.org", "chromeos-wmp@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(
			hwdep.InternalDisplay(),
			// Due to the varying sizes of touchpads on different models, it is hard to have one good swipe
			// motion reliably pass all models. Since this test is for collecting performance metrics, having it work on most boards is good enough.
			hwdep.SkipOnModel("kohaku", "morphius", "samus"),
		),
		Fixture: "chromeLoggedIn",
	})
}

func DesksTrackpadSwipePerf(ctx context.Context, s *testing.State) {
	// Reserve five seconds for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	// Ensure display on to record ui performance correctly.
	if err := power.TurnOnDisplay(ctx); err != nil {
		s.Fatal("Failed to turn on display: ", err)
	}

	cr := s.FixtValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	// Add an extra desk and remove it at the end of the test.
	if err = ash.CreateNewDesk(ctx, tconn); err != nil {
		s.Fatal("Failed to create a new desk: ", err)
	}
	defer ash.CleanUpDesks(cleanupCtx, tconn)

	// Create a virtual trackpad.
	tpw, err := input.Trackpad(ctx)
	if err != nil {
		s.Fatal("Failed to create a trackpad device: ", err)
	}
	defer tpw.Close()

	tw, err := tpw.NewMultiTouchWriter(4)
	if err != nil {
		s.Fatal("Failed to create a multi touch writer: ", err)
	}
	defer tw.Close()

	// Performs a four finger horizontal scroll on the trackpad. The vertical location is always vertically
	// centered on the trackpad. The fingers are spaced horizontally on the trackpad by 1/16th of the trackpad
	// width.
	fingerSpacing := tpw.Width() / 16
	doTrackpadFourFingerSwipeScroll := func(ctx context.Context, x0, x1 input.TouchCoord) error {
		y := tpw.Height() / 2
		return tw.Swipe(ctx, x0, y, x1, y, fingerSpacing, 0, 4, time.Second)
	}

	// This is supposedly the distance (in trackpad units) spanned by the 4 fingers on the trackpad. It should
	// be fingerSpacing * 3 because if there are 4 fingers then there are only 3 spaces between fingers. We
	// still use fingerSpacing * 4, just to keep new performance data consistent with old performance data.
	fingerDistance := fingerSpacing * 4

	pv := perfutil.RunMultiple(ctx, cr.Browser(), uiperf.Run(s, perfutil.RunAndWaitAll(tconn, func(ctx context.Context) error {
		// Do a big swipe going right. This will continuously shift to the next desk on the right.
		// Note: The swipe should end at tpw.Width()-1-fingerDistance because a finger at tpw.Width() is
		// off the trackpad. However, fingerDistance is already more than it should be, so this works out.
		if err := doTrackpadFourFingerSwipeScroll(ctx, 0, tpw.Width()-fingerDistance); err != nil {
			return errors.Wrap(err, "failed to perform four finger scroll")
		}

		if err := tw.End(); err != nil {
			return errors.Wrap(err, "failed to finish trackpad scroll")
		}

		// TODO(sammiequon): Add an API to properly wait for the end animation to finish.
		if err = testing.Sleep(ctx, time.Second); err != nil {
			s.Fatal("Failed to wait: ", err)
		}

		// Activate the desk at index 0 for the next run.
		if err = ash.ActivateDeskAtIndex(ctx, tconn, 0); err != nil {
			return errors.Wrap(err, "failed to activate the first desk")
		}
		return nil
	},
		"Ash.Desks.AnimationSmoothness.DeskEndGesture",
		"Ash.Desks.PresentationTime.UpdateGesture",
		"Ash.Desks.PresentationTime.UpdateGesture.MaxLatency")),
		perfutil.StoreAllWithHeuristics(""))

	if err := pv.Save(ctx, s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}
