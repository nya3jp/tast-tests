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
	"chromiumos/tast/local/chrome/ui/mouse"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: WindowControl,
		Desc: "Check if the performance around window controlling is good enough; go/cros-ui-perftests-cq#heading=h.fwfk0yg3teo1",
		Contacts: []string{
			"oshima@chromium.org",
			"afakhry@chromium.org",
			"chromeos-wmp@google.com",
			"mukai@chromium.org", // Tast author
		},
		Attr:         []string{"group:mainline"},
		Pre:          chrome.LoggedIn(),
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
	})
}

func WindowControl(ctx context.Context, s *testing.State) {
	expects := perfutil.CreateExpectations(ctx,
		"Ash.Window.AnimationSmoothness.CrossFade",
		"Ash.Window.AnimationSmoothness.CrossFade.DragMaximize",
		"Ash.Window.AnimationSmoothness.CrossFade.DragUnmaximize",
		"Ash.WindowCycleView.AnimationSmoothness.Show",
		"Ash.Overview.AnimationSmoothness.Enter.ClamshellMode",
		"Ash.Overview.AnimationSmoothness.Exit.ClamshellMode",
		"Ash.InteractiveWindowResize.TimeToPresent",
	)
	// When custom expectation value needs to be set, modify expects here.
	// Ash.WindowCycleView.AnimationSmoothness.Show is known bad: https://crbug.com/1111130
	expects["Ash.WindowCycleView.AnimationSmoothness.Show"] = 20
	// DragMaximize/Unmaximize is known bad: https://crbug.com/1170544
	expects["Ash.Window.AnimationSmoothness.CrossFade.DragMaximize"] = 20
	expects["Ash.Window.AnimationSmoothness.CrossFade.DragUnmaximize"] = 20

	cr := s.PreValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to get the connection to the test API: ", err)
	}
	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		s.Fatal("Failed to ensure into clamshell mode: ", err)
	}
	defer cleanup(ctx)
	conns, err := ash.CreateWindows(ctx, tconn, cr, "", 8)
	if err != nil {
		s.Fatal("Failed to create new windows: ", err)
	}
	if err := conns.Close(); err != nil {
		s.Fatal("Failed to close the connections: ", err)
	}
	ws, err := ash.GetAllWindows(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get the windows: ", err)
	}

	r := perfutil.NewRunner(cr)
	r.Runs = 5
	r.RunTracing = false

	s.Log("Step 1: window state transition")
	// List of target states.
	// The default window state can be either of maximized or normal, depending on
	// the screen size. When the default state is maximized, switch to normal and
	// then back to maximized state.
	states := []ash.WindowStateType{
		ash.WindowStateNormal,
		ash.WindowStateMaximized}
	// When the default state is normal, switch to maximized and then back to
	// normal state.
	if ws[0].State == ash.WindowStateNormal {
		states = []ash.WindowStateType{
			ash.WindowStateMaximized,
			ash.WindowStateNormal}
	}
	r.RunMultiple(ctx, s, "window-state", perfutil.RunAndWaitAll(tconn, func(ctx context.Context) error {
		for i, newState := range states {
			if err := ash.SetWindowStateAndWait(ctx, tconn, ws[0].ID, newState); err != nil {
				return errors.Wrapf(err, "failed to set window state at step %d", i)
			}
		}
		return nil
	}, "Ash.Window.AnimationSmoothness.CrossFade"),
		perfutil.StoreSmoothness)

	s.Log("Step 2: drag the maximized window")
	if err := ash.SetWindowStateAndWait(ctx, tconn, ws[0].ID, ash.WindowStateMaximized); err != nil {
		s.Fatalf("Failed to maximize %d: %v", ws[0].ID, err)
	}
	w, err := ash.FindWindow(ctx, tconn, func(w *ash.Window) bool { return w.ID == ws[0].ID })
	if err != nil {
		s.Fatal("Failed to find the window: ", err)
	}
	bounds := w.BoundsInRoot
	center := bounds.CenterPoint()
	// Drag points; move across the entire screen.
	points := []coords.Point{
		coords.NewPoint(center.X, bounds.Top),
		coords.NewPoint(bounds.Left, center.Y),
		coords.NewPoint(center.X, bounds.Bottom()),
		coords.NewPoint(bounds.Right(), center.Y),
		coords.NewPoint(center.X, bounds.Top),
	}
	r.RunMultiple(ctx, s, "drag-maximized-window", perfutil.RunAndWaitAll(tconn, func(ctx context.Context) error {
		if err := mouse.Move(ctx, tconn, points[0], 0); err != nil {
			return errors.Wrap(err, "failed to move to the start position")
		}
		if err := mouse.Press(ctx, tconn, mouse.LeftButton); err != nil {
			return errors.Wrap(err, "failed to release the button")
		}
		defer mouse.Release(ctx, tconn, mouse.LeftButton)
		for _, point := range points[1:] {
			if err := mouse.Move(ctx, tconn, point, 200*time.Millisecond); err != nil {
				return errors.Wrap(err, "failed to move the mouse")
			}
		}
		if err := mouse.Release(ctx, tconn, mouse.LeftButton); err != nil {
			return errors.Wrap(err, "failed to release the button")
		}
		if err := ash.WaitWindowFinishAnimating(ctx, tconn, w.ID); err != nil {
			return errors.Wrap(err, "failed to wait for the top window animation")
		}
		return nil
	},
		"Ash.Window.AnimationSmoothness.CrossFade.DragMaximize",
		"Ash.Window.AnimationSmoothness.CrossFade.DragUnmaximize"),
		perfutil.StoreSmoothness)

	s.Log("Step 3: alt-tab to change the active window")
	kw, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get the keyboard: ", err)
	}
	defer kw.Close()
	r.RunMultiple(ctx, s, "alt-tab", perfutil.RunAndWaitAll(tconn, func(ctx context.Context) (err error) {
		pressed := false
		defer func() {
			if pressed {
				if releaseErr := kw.AccelRelease(ctx, "Alt"); releaseErr != nil {
					testing.ContextLog(ctx, "Failed to release the alt key: ", releaseErr)
					if err == nil {
						err = releaseErr
					}
				}
			}
		}()
		if err := kw.AccelPress(ctx, "Alt"); err != nil {
			return errors.Wrap(err, "failed to press the alt key")
		}
		pressed = true
		if err := kw.Accel(ctx, "Tab"); err != nil {
			return errors.Wrap(err, "failed to hit tab")
		}
		// Right now we don't have good events to wait for the alt-tab switching,
		// so simply waiting for 500 msecs.
		if err := testing.Sleep(ctx, 500*time.Millisecond); err != nil {
			return errors.Wrap(err, "failed to wait for the ")
		}
		if err := kw.AccelRelease(ctx, "Alt"); err != nil {
			return errors.Wrap(err, "failed to release the alt key")
		}
		// Right now we don't have good events to wait for the alt-tab switching,
		// so simply waiting for 500 msecs.
		if err := testing.Sleep(ctx, 500*time.Millisecond); err != nil {
			return errors.Wrap(err, "failed to wait for the ")
		}
		pressed = false
		return nil
	}, "Ash.WindowCycleView.AnimationSmoothness.Show"),
		perfutil.StoreSmoothness)

	s.Log("Step 4: overview mode")
	// To prepare the oveview mode, we want to ensure that all windows are in
	// normal state.
	ws, err = ash.GetAllWindows(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get all windows: ", err)
	}
	for _, w := range ws {
		if err := ash.SetWindowStateAndWait(ctx, tconn, w.ID, ash.WindowStateNormal); err != nil {
			s.Fatalf("Failed to turn window %d into normal: %v", w.ID, err)
		}
	}
	r.RunMultiple(ctx, s, "overview", perfutil.RunAndWaitAll(tconn, func(ctx context.Context) error {
		if err := ash.SetOverviewModeAndWait(ctx, tconn, true); err != nil {
			return errors.Wrap(err, "failed to enter into the overview mode")
		}
		if err := ash.SetOverviewModeAndWait(ctx, tconn, false); err != nil {
			return errors.Wrap(err, "failed to exit from the overview mode")
		}
		return nil
	},
		"Ash.Overview.AnimationSmoothness.Enter.ClamshellMode",
		"Ash.Overview.AnimationSmoothness.Exit.ClamshellMode"),
		perfutil.StoreSmoothness)

	s.Log("Step 5: window resizes")
	// Assumes the window is already in normal state for the preparation of the
	// previous step.  Also assumes the ws[0] is the topmost window.
	r.RunMultiple(ctx, s, "resize", perfutil.RunAndWaitAll(tconn, func(ctx context.Context) error {
		var w *ash.Window
		if err := ash.WaitForCondition(ctx, tconn, func(window *ash.Window) bool {
			if ws[0].ID == window.ID && window.State == ash.WindowStateNormal && window.OverviewInfo == nil && !window.IsAnimating {
				w = window
				return true
			}
			return false
		}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
			return errors.Wrapf(err, "failed to find the window %d", w.ID)
		}
		bounds := w.BoundsInRoot
		tr := bounds.TopRight()
		if err := mouse.Move(ctx, tconn, tr, 0); err != nil {
			return errors.Wrap(err, "failed to move the mouse to the initial location")
		}
		if err := mouse.Press(ctx, tconn, mouse.LeftButton); err != nil {
			return errors.Wrap(err, "failed to press the left button")
		}
		defer mouse.Release(ctx, tconn, mouse.LeftButton)
		for i, point := range []coords.Point{bounds.CenterPoint(), tr} {
			if err := mouse.Move(ctx, tconn, point, 500*time.Millisecond); err != nil {
				return errors.Wrapf(err, "failed to move the mouse to %v at step %d", point, i)
			}
		}
		return nil
	}, "Ash.InteractiveWindowResize.TimeToPresent"),
		perfutil.StoreLatency)

	// Check the validity of histogram data.
	for _, err := range r.Values().Verify(ctx, expects) {
		s.Error("Performance expectation missed: ", err)
	}
	// Storing the results for the future analyses.
	if err := r.Values().Save(ctx, s.OutDir()); err != nil {
		s.Error("Failed to save the values: ", err)
	}
}
