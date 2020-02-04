// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/local/ui"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         HotseatAnimation,
		Desc:         "Measures the framerate of the hotseat animation in tablet mode",
		Contacts:     []string{"newcomer@chromium.org", "manucornet@chromium.org", "enter-actual-team-name@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome", "tablet_mode"},
		Pre:          chrome.LoggedIn(),
	})
}

func HotseatAnimation(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)

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
	if err = tsw.SetRotation(-orientation.Angle); err != nil {
		s.Fatal("Failed to set rotation: ", err)
	}

	stw, err := tsw.NewSingleTouchWriter()
	if err != nil {
		s.Fatal("Failed to create single touch writer: ", err)
	}
	defer stw.Close()

	histograms, err := metrics.Run(ctx, cr, func() error {
		// Before going in-app, wait for things to calm down.
		if err := cpu.WaitUntilIdle(ctx); err != nil {
			s.Fatal("Failed waiting for CPU to become idle: ", err)
		}

		// Open a window to hide the launcher and animate the hotseat to Hidden.
		const numWindows = 1
		conns, err := ash.CreateWindows(ctx, cr, ui.PerftestURL, numWindows)
		if err != nil {
			s.Fatal("Failed to open browser windows: ", err)
		}
		defer conns.Close()

		// Wait for the animation to complete.
		if err := cpu.WaitUntilIdle(ctx); err != nil {
			s.Fatal("Failed waiting for CPU to become idle: ", err)
		}

		// Open overview to animate the hotseat to Extended.
		startx := input.TouchCoord(tsw.Width() / 2)
		starty := input.TouchCoord(tsw.Height() - 1)

		endx := startx
		endy := input.TouchCoord(tsw.Height() / 2)

		s.Log("Dragging from the bottom slowly to open overview")
		if err := stw.Swipe(ctx, startx, starty, endx, endy, 500*time.Millisecond); err != nil {
			return errors.Wrap(err, "failed to tap")
		}
		// Wait for overview to trigger.
		testing.Sleep(ctx, time.Second*1)
		if err := stw.End(); err != nil {
			return errors.Wrap(err, "failed to finish the tap gesture")
		}

		if err := cpu.WaitUntilIdle(ctx); err != nil {
			s.Fatal("Failed waiting for CPU to become idle: ", err)
		}

		s.Log("Tapping an empty space in overview to open the launcher")
		// Open the launcher to animate the hotseat to Shown.
		pressX := input.TouchCoord(tsw.Width() * 5 / 6)
		pressY := input.TouchCoord(tsw.Height() / 2)
		if err := stw.Swipe(ctx, pressX, pressY, pressX+5, pressY-5, 200*time.Millisecond); err != nil {
			return errors.Wrap(err, "failed to tap")
		}
		if err := stw.End(); err != nil {
			return errors.Wrap(err, "failed to finish the tap gesture")
		}

		if err := cpu.WaitUntilIdle(ctx); err != nil {
			s.Fatal("Failed waiting for CPU to become idle: ", err)
		}

		// Open overview to animate the hotseat to Extended.
		startx = input.TouchCoord(tsw.Width() / 2)
		starty = input.TouchCoord(tsw.Height() - 1)

		endx = startx
		endy = input.TouchCoord(tsw.Height() / 2)
		s.Log("Dragging up slowly to go to overview")
		if err := stw.Swipe(ctx, startx, starty, endx, endy, 500*time.Millisecond); err != nil {
			return errors.Wrap(err, "failed to tap")
		}
		// Wait for overview to trigger.
		testing.Sleep(ctx, time.Second*1)
		if err := stw.End(); err != nil {
			return errors.Wrap(err, "failed to finish the tap gesture")
		}

		if err := cpu.WaitUntilIdle(ctx); err != nil {
			s.Fatal("Failed waiting for CPU to become idle: ", err)
		}

		s.Log("Tap the item to go back to in-app")
		x := input.TouchCoord(tsw.Width() / 3)
		y := input.TouchCoord(tsw.Height() / 3)

		if err := stw.Swipe(ctx, x, y, x+5, y-5, 200*time.Millisecond); err != nil {
			return errors.Wrap(err, "failed to tap")
		}
		if err := stw.End(); err != nil {
			return errors.Wrap(err, "failed to finish the tap gesture")
		}

		if err := cpu.WaitUntilIdle(ctx); err != nil {
			s.Fatal("Failed waiting for CPU to become idle: ", err)
		}

		s.Log("Go to home launcher by swiping up")
		startx = input.TouchCoord(tsw.Width() / 2)
		starty = input.TouchCoord(tsw.Height() - 1)

		endx = startx
		endy = input.TouchCoord(tsw.Height() / 2)

		if err := stw.Swipe(ctx, startx, starty, endx, endy, 200*time.Millisecond); err != nil {
			return errors.Wrap(err, "failed to tap")
		}

		if err := stw.End(); err != nil {
			return errors.Wrap(err, "failed to finish the tap gesture")
		}

		return nil
	},
		"Ash.HotseatTransition.AnimationSmoothness.TransitionToHiddenHotseat",
		"Ash.HotseatTransition.AnimationSmoothness.TransitionToShownHotseat",
		"Ash.HotseatTransition.AnimationSmoothness.TransitionToExtendedHotseat")
	if err != nil {
		s.Fatal("Failed to swipe or get histogram: ", err)
	}

	pv := perf.NewValues()
	for _, h := range histograms {
		mean, err := h.Mean()
		if err != nil {
			s.Fatalf("Failed to get mean for histogram %s: %v", h.Name, err)
		}

		pv.Set(perf.Metric{
			Name:      h.Name,
			Unit:      "ms",
			Direction: perf.SmallerIsBetter,
		}, mean)
		s.Log(mean)
	}

	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}
