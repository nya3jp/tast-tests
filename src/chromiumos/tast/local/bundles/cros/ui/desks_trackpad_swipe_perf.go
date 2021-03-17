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
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DesksTrackpadSwipePerf,
		Desc:         "Measures the performance of using the trackpad to change desks",
		Contacts:     []string{"afakhry@chromium.org", "sammiequon@chromium.org", "chromeos-wmp@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(
			hwdep.InternalDisplay(),
			// Due to the varying sizes of touchpads on different models, it is hard to have one good swipe
			// motion reliably pass all models. Since samus is an older model, just skip running it.
			hwdep.SkipOnModel("samus"),
		),
		Fixture: "chromeLoggedIn",
	})
}

func DesksTrackpadSwipePerf(ctx context.Context, s *testing.State) {
	// Ensure display on to record ui performance correctly.
	if err := perfutil.EnsureDisplayOn(ctx); err != nil {
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
	defer func(ctx context.Context) {
		if err = ash.RemoveActiveDesk(ctx, tconn); err != nil {
			s.Error("Failed to remove the active desk: ", err)
		}
	}(ctx)

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
		const t = time.Second
		return tw.Swipe(ctx, x0, y, x1, y, fingerSpacing, 4, t)
	}

	// The amount of trackpad units taken up by placing all 4 fingers on the trackpad. Used to ensure the units
	// passed to doTrackpadFourFingerSwipeScroll stay on the trackpad.
	fingerDistance := fingerSpacing * 4

	pv := perfutil.RunMultiple(ctx, s, cr, perfutil.RunAndWaitAll(tconn, func(ctx context.Context) error {
		// Do a big swipe going right. This will continuously shift to the next desk on the right.
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
		"Ash.Desks.PresentationTime.UpdateGesture.MaxLatency"),
		perfutil.StoreAllWithHeuristics(""))

	if err := pv.Save(ctx, s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}
