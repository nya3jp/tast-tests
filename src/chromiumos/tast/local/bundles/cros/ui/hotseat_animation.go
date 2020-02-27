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
		Contacts:     []string{"newcomer@chromium.org", "manucornet@chromium.org", "cros-shelf-prod-notifications@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome", "tablet_mode"},
		Pre:          chrome.LoggedIn(),
		Timeout:      8 * time.Minute,
	})
}

// showOverview shows overview by dragging up, pausing for the gesture to be recognized, then ending the gesture.
func showOverview(ctx context.Context, tsw *input.TouchscreenEventWriter, stw *input.SingleTouchEventWriter) error {
	startX := tsw.Width() / 2
	startY := tsw.Height() - 1

	endX := startX
	endY := tsw.Height() / 2

	testing.ContextLog(ctx, "Dragging from the bottom slowly to open overview")
	if err := stw.Swipe(ctx, startX, startY, endX, endY, 500*time.Millisecond); err != nil {
		return errors.Wrap(err, "failed to swipe")
	}
	// Wait with the swipe paused so the overview mode gesture is recognized. Use 1 second because this is roughly the amount of time it takes for the 'swipe up and hold' overview gesture to trigger.
	const pauseDuration = time.Second
	if err := testing.Sleep(ctx, pauseDuration); err != nil {
		return errors.Wrap(err, "failed to sleep while waiting for overview to trigger")
	}
	if err := stw.End(); err != nil {
		return errors.Wrap(err, "failed to finish the swipe gesture")
	}

	// After overview has triggered, wait for animations to complete.
	// TODO(https://crbug.com/1052211): Create an autotest api to check whether the overview mode animation has completed.
	const waitDuration = 2 * time.Second
	if err := testing.Sleep(ctx, waitDuration); err != nil {
		return errors.Wrap(err, "failed to sleep")
	}
	return nil
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
	defer tsw.Close()
	if err := tsw.SetRotation(-orientation.Angle); err != nil {
		s.Fatal("Failed to set rotation: ", err)
	}

	stw, err := tsw.NewSingleTouchWriter()
	if err != nil {
		s.Fatal("Failed to create single touch writer: ", err)
	}
	defer stw.Close()

	histograms, err := metrics.Run(ctx, tconn, func() error {
		// Open a window to hide the launcher and animate the hotseat to Hidden.
		const numWindows = 1
		conns, err := ash.CreateWindows(ctx, cr, ui.PerftestURL, numWindows)
		if err != nil {
			return errors.Wrap(err, "failed to open browser windows: ")
		}
		if err := conns.Close(); err != nil {
			s.Error("Failed to close the connection to a browser window")
		}

		// Wait for the animations to complete and for things to settle down.
		if err := cpu.WaitUntilIdle(ctx); err != nil {
			s.Fatal("Failed waiting for CPU to become idle: ", err)
		}

		if err := showOverview(ctx, tsw, stw); err != nil {
			return errors.Wrap(err, "failed to drag from bottom of the screen to show overview")
		}

		s.Log("Tapping an empty space in overview to open the launcher")
		pressX := tsw.Width() * 5 / 6
		pressY := tsw.Height() / 2
		if err := stw.Swipe(ctx, pressX, pressY, pressX+5, pressY-5, 200*time.Millisecond); err != nil {
			return errors.Wrap(err, "failed to tap")
		}
		if err := stw.End(); err != nil {
			return errors.Wrap(err, "failed to finish the tap gesture")
		}

		// After overview has dismissed, wait for the overview animation to complete.
		// TODO(https://crbug.com/1052211): Create an autotest api to check whether the overview mode animation has completed.
		const overviewAnimationWaitDuration = 2 * time.Second
		if err := testing.Sleep(ctx, overviewAnimationWaitDuration); err != nil {
			return errors.Wrap(err, "failed to sleep")
		}

		if err := showOverview(ctx, tsw, stw); err != nil {
			return errors.Wrap(err, "failed to drag from bottom of the screen to show overview")
		}

		s.Log("Tap the overview item to go back to in-app")
		pressX = tsw.Width() / 3
		pressY = tsw.Height() / 3

		if err := stw.Swipe(ctx, pressX, pressY, pressX+5, pressY-5, 200*time.Millisecond); err != nil {
			return errors.Wrap(err, "failed to tap")
		}
		if err := stw.End(); err != nil {
			return errors.Wrap(err, "failed to finish the tap gesture")
		}

		// After overview has been dismissed, wait for the overview animation to complete.
		// TODO(https://crbug.com/1052211): Create an autotest api to check whether the overview mode animation has completed.
		if err := testing.Sleep(ctx, overviewAnimationWaitDuration); err != nil {
			return errors.Wrap(err, "failed to sleep")
		}

		s.Log("Go to home launcher by swiping up")
		startX := tsw.Width() / 2
		startY := tsw.Height() - 1

		endX := startX
		endY := tsw.Height() / 2

		if err := stw.Swipe(ctx, startX, startY, endX, endY, 200*time.Millisecond); err != nil {
			return errors.Wrap(err, "failed to swipe")
		}

		if err := stw.End(); err != nil {
			return errors.Wrap(err, "failed to finish the swipe gesture")
		}

		// After going to home launcher, wait for the animation to complete.
		// TODO(https://crbug.com/1052215): Create an autotest api to check whether the home launcher animation has completed.
		const homeLauncherAnimationWaitDuration = 2 * time.Second
		if err := testing.Sleep(ctx, homeLauncherAnimationWaitDuration); err != nil {
			return errors.Wrap(err, "failed to sleep")
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
			Unit:      "percent",
			Direction: perf.BiggerIsBetter,
		}, mean)
	}

	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}
