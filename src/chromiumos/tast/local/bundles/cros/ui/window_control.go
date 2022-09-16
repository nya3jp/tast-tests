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
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/perfutil"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WindowControl,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Check if the performance around window controlling is good enough; go/cros-ui-perftests-cq#heading=h.fwfk0yg3teo1",
		Contacts: []string{
			"oshima@chromium.org",
			"afakhry@chromium.org",
			"chromeos-wmp@google.com",
			"mukai@chromium.org", // Tast author
		},
		Attr: []string{"group:mainline"},
		// no_qemu: VMs often fail performance expectations.
		SoftwareDeps: []string{"chrome", "no_chrome_dcheck", "no_qemu"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Params: []testing.Param{{
			Fixture: "chromeLoggedIn",
			Val:     browser.TypeAsh,
		}, {
			Name:              "lacros",
			Fixture:           "lacros",
			ExtraAttr:         []string{"informational"},
			ExtraSoftwareDeps: []string{"lacros"},
			Val:               browser.TypeLacros,
		}},
	})
}

func WindowControl(ctx context.Context, s *testing.State) {
	// Reserve a few seconds for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(cleanupCtx, 10*time.Second)
	defer cancel()

	interactiveWindowResizeHistogram := "Ash.InteractiveWindowResize.TimeToPresent"
	if s.Param().(browser.Type) == browser.TypeLacros {
		interactiveWindowResizeHistogram = "Ash.InteractiveWindowResize.Lacros.TimeToPresent"
	}
	expects := perfutil.CreateExpectations(ctx,
		"Ash.Window.AnimationSmoothness.CrossFade",
		"Ash.Window.AnimationSmoothness.CrossFade.DragMaximize",
		"Ash.Window.AnimationSmoothness.CrossFade.DragUnmaximize",
		"Ash.WindowCycleView.AnimationSmoothness.Show",
		"Ash.Overview.AnimationSmoothness.Enter.ClamshellMode",
		"Ash.Overview.AnimationSmoothness.Exit.ClamshellMode",
		interactiveWindowResizeHistogram,
	)
	// When custom expectation value needs to be set, modify expects here.
	// Ash.WindowCycleView.AnimationSmoothness.Show is known bad: https://crbug.com/1111130
	expects["Ash.WindowCycleView.AnimationSmoothness.Show"] = 20
	// DragMaximize/Unmaximize is known bad: https://crbug.com/1170544
	expects["Ash.Window.AnimationSmoothness.CrossFade.DragMaximize"] = 20
	expects["Ash.Window.AnimationSmoothness.CrossFade.DragUnmaximize"] = 20

	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to get the connection to the test API: ", err)
	}
	// Set up the browser, open a first window.
	const numWindows = 8
	const url = chrome.BlankURL
	conn, br, closeBrowser, err := browserfixt.SetUpWithURL(ctx, cr, s.Param().(browser.Type), url)
	if err != nil {
		s.Fatal("Failed to open the browser: ", err)
	}
	defer closeBrowser(cleanupCtx)
	defer conn.Close()

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		s.Fatal("Failed to ensure into clamshell mode: ", err)
	}
	defer cleanup(cleanupCtx)
	// Open the rest of the new windows alongside the one that was already opened above.
	if err := ash.CreateWindows(ctx, tconn, br, url, numWindows-1); err != nil {
		s.Fatal("Failed to create new windows: ", err)
	}
	ws, err := ash.GetAllWindows(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get the windows: ", err)
	}

	r := perfutil.NewRunner(br)
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
	r.RunMultiple(ctx, "window-state", uiperf.Run(s, perfutil.RunAndWaitAll(tconn, func(ctx context.Context) error {
		for i, newState := range states {
			if err := ash.SetWindowStateAndWait(ctx, tconn, ws[0].ID, newState); err != nil {
				return errors.Wrapf(err, "failed to set window state at step %d", i)
			}
		}
		return nil
	}, "Ash.Window.AnimationSmoothness.CrossFade")),
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
	r.RunMultiple(ctx, "drag-maximized-window", uiperf.Run(s, perfutil.RunAndWaitAll(tconn, func(ctx context.Context) error {
		if err := mouse.Move(tconn, points[0], 0)(ctx); err != nil {
			return errors.Wrap(err, "failed to move to the start position")
		}
		if err := mouse.Press(tconn, mouse.LeftButton)(ctx); err != nil {
			return errors.Wrap(err, "failed to release the button")
		}
		defer mouse.Release(tconn, mouse.LeftButton)
		for _, point := range points[1:] {
			if err := mouse.Move(tconn, point, 200*time.Millisecond)(ctx); err != nil {
				return errors.Wrap(err, "failed to move the mouse")
			}
		}
		// Needs to wait a bit before releasing the mouse, otherwise the window
		// may not get back to be maximized.  See https://crbug.com/1158548.
		if err := testing.Sleep(ctx, time.Second); err != nil {
			return errors.Wrap(err, "failed to wait")
		}
		if err := mouse.Release(tconn, mouse.LeftButton)(ctx); err != nil {
			return errors.Wrap(err, "failed to release the button")
		}
		if err := ash.WaitWindowFinishAnimating(ctx, tconn, w.ID); err != nil {
			return errors.Wrap(err, "failed to wait for the top window animation")
		}
		// Validity check to ensure the window is maximized at the end.
		window, err := ash.GetWindow(ctx, tconn, w.ID)
		if err != nil {
			return errors.Wrapf(err, "failed to obtain the window info for %d", w.ID)
		}
		if window.State != ash.WindowStateMaximized {
			return errors.New("window is not maximized")
		}
		return nil
	},
		"Ash.Window.AnimationSmoothness.CrossFade.DragMaximize",
		"Ash.Window.AnimationSmoothness.CrossFade.DragUnmaximize")),
		perfutil.StoreSmoothness)

	s.Log("Step 3: alt-tab to change the active window")
	kw, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get the keyboard: ", err)
	}
	defer kw.Close()
	r.RunMultiple(ctx, "alt-tab", uiperf.Run(s, perfutil.RunAndWaitAll(tconn, func(ctx context.Context) (err error) {
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
	}, "Ash.WindowCycleView.AnimationSmoothness.Show")),
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
	r.RunMultiple(ctx, "overview", uiperf.Run(s, perfutil.RunAndWaitAll(tconn, func(ctx context.Context) error {
		if err := ash.SetOverviewModeAndWait(ctx, tconn, true); err != nil {
			return errors.Wrap(err, "failed to enter into the overview mode")
		}
		if err := ash.SetOverviewModeAndWait(ctx, tconn, false); err != nil {
			return errors.Wrap(err, "failed to exit from the overview mode")
		}
		return nil
	},
		"Ash.Overview.AnimationSmoothness.Enter.ClamshellMode",
		"Ash.Overview.AnimationSmoothness.Exit.ClamshellMode")),
		perfutil.StoreSmoothness)

	s.Log("Step 5: window resizes")
	// Assumes the window is already in normal state for the preparation of the
	// previous step.  Also assumes the ws[0] is the topmost window.
	r.RunMultiple(ctx, "resize", uiperf.Run(s, perfutil.RunAndWaitAll(tconn, func(ctx context.Context) error {
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
		if err := mouse.Move(tconn, tr, 0)(ctx); err != nil {
			return errors.Wrap(err, "failed to move the mouse to the initial location")
		}
		if err := mouse.Press(tconn, mouse.LeftButton)(ctx); err != nil {
			return errors.Wrap(err, "failed to press the left button")
		}
		defer mouse.Release(tconn, mouse.LeftButton)
		for i, point := range []coords.Point{bounds.CenterPoint(), tr} {
			if err := mouse.Move(tconn, point, 500*time.Millisecond)(ctx); err != nil {
				return errors.Wrapf(err, "failed to move the mouse to %v at step %d", point, i)
			}
		}
		return nil
	}, interactiveWindowResizeHistogram)),
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
