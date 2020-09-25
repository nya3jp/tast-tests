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
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
	})
}

func DesksTrackpadSwipePerf(ctx context.Context, s *testing.State) {
	// TODO(sammiequon): When the feature is fully launched, use chrome.LoggedIn() precondition.
	cr, err := chrome.New(ctx, chrome.ExtraArgs("--enable-features=EnhancedDeskAnimations"))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	// Add three desks for a total of four. Remove them at the end of the test.
	const numDesks = 3
	for i := 0; i < numDesks; i++ {
		if err = ash.CreateNewDesk(ctx, tconn); err != nil {
			s.Error("Failed to create a new desk: ", err)
		}
		defer func() {
			if err = ash.RemoveActiveDesk(ctx, tconn); err != nil {
				s.Error("Failed to remove the active desk: ", err)
			}
		}()
	}

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

	// Performs four finger horizontal scrolling on the trackpad. The vertical location is always vertically
	// centered on the trackpad. The fingers are spaced horizontally on the trackpad by 1/16th of the trackpad
	// width.
	fingerSpacing := tpw.Width() / 16
	doTrackpadFourFingerSwipeScroll := func(ctx context.Context, x0, x1 input.TouchCoord) error {
		y := tpw.Height() / 2
		const t = time.Second * 2
		return tw.Swipe(ctx, x0, y, x1, y, fingerSpacing, 4, t)
	}

	// The amount of trackpad units taken up by placing all 4 fingers on the trackpad. Used to ensure the units
	// passed to doTrackpadFourFingerSwipeScroll stay on the trackpad.
	fingerDistance := fingerSpacing * 4

	pv := perfutil.RunMultiple(ctx, s, cr, perfutil.RunAndWaitAll(tconn, func(ctx context.Context) error {
		// Do a big swipe going left. This will continuous update us to the last desk.
		if err := doTrackpadFourFingerSwipeScroll(ctx, tpw.Width()-fingerDistance, 0); err != nil {
			return errors.Wrap(err, "failed to perform four finger scroll")
		}

		if err := tw.End(); err != nil {
			return errors.Wrap(err, "failed to finish trackpad scroll")
		}

		// TODO(sammiequon): Find a way to properly wait for the end animation to finish.
		if err = testing.Sleep(ctx, 500*time.Millisecond); err != nil {
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
		perfutil.StoreAllWithHeuristics)

	if err := pv.Save(ctx, s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}
