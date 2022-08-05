// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"net/http"
	"net/http/httptest"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/lacros"
	"chromiumos/tast/local/chrome/uiauto/pointer"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/ui/cujrecorder"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WindowStateTransitionsCUJ,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Measures the performance of critical user journey for window state transitions",
		Contacts:     []string{"amusbach@chromium.org", "chromeos-perfmetrics-eng@google.com"},
		Attr:         []string{"group:cuj"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{"animation.html", "animation.js", cujrecorder.SystemTraceConfigFile},
		Timeout:      15 * time.Minute,
		Params: []testing.Param{{
			Val:     browser.TypeAsh,
			Fixture: "loggedInToCUJUser",
		}, {
			Name:              "lacros",
			Val:               browser.TypeLacros,
			ExtraSoftwareDeps: []string{"lacros"},
			Fixture:           "loggedInToCUJUserLacros",
		}},
	})
}

func WindowStateTransitionsCUJ(ctx context.Context, s *testing.State) {
	// Reserve ten seconds for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	cr := s.FixtValue().(chrome.HasChrome).Chrome()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		s.Fatal("Failed to ensure clamshell mode: ", err)
	}
	defer cleanup(cleanupCtx)

	info, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get the primary display info: ", err)
	}

	pc := pointer.NewMouse(tconn)
	defer pc.Close()

	srv := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer srv.Close()
	animationURL := srv.URL + "/animation.html"

	var bTconn *chrome.TestConn
	switch s.Param().(browser.Type) {
	case browser.TypeAsh:
		conn, err := cr.NewConn(ctx, animationURL)
		if err != nil {
			s.Fatal("Failed to launch ash chrome: ", err)
		}
		defer conn.Close()

		bTconn = tconn
	case browser.TypeLacros:
		l, err := lacros.LaunchWithURL(ctx, tconn, animationURL)
		if err != nil {
			s.Fatal("Failed to launch lacros: ", err)
		}
		defer l.Close(cleanupCtx)

		if bTconn, err = l.TestAPIConn(ctx); err != nil {
			s.Fatal("Failed to get lacros TestAPIConn: ", err)
		}
	}

	// Verify that there is only one window, and get its ID.
	ws, err := ash.GetAllWindows(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get the windows: ", err)
	}
	if len(ws) != 1 {
		s.Fatalf("Unexpected number of windows: got %d; want 1", len(ws))
	}
	wID := ws[0].ID

	// Set the window bounds for the "Normal" state.
	wState, err := ash.SetWindowState(ctx, tconn, wID, ash.WMEventNormal, true)
	if err != nil {
		s.Fatal("Failed to set window state to \"Normal\": ", err)
	}
	if wState != ash.WindowStateNormal {
		s.Fatalf("Unexpected window state: got %q; want \"Normal\"", wState)
	}
	desiredBounds := info.WorkArea.WithResizeAboutCenter(info.WorkArea.Width/2, info.WorkArea.Height/2)
	bounds, displayID, err := ash.SetWindowBounds(ctx, tconn, wID, desiredBounds, info.ID)
	if err != nil {
		s.Error("Failed to set the window bounds: ", err)
	}
	if displayID != info.ID {
		s.Errorf("Unexpected display ID for window: got %q; want %q", displayID, info.ID)
	}
	if bounds != desiredBounds {
		s.Errorf("Unexpected window bounds: got %v; want %v", bounds, desiredBounds)
	}

	// Maximize the window. The performance measurement will begin
	// with the window maximized.
	if err := ash.SetWindowStateAndWait(ctx, tconn, wID, ash.WindowStateMaximized); err != nil {
		s.Fatal("Failed to set window state to \"Maximized\": ", err)
	}

	// Get the window's maximized bounds, and use them to create an
	// action that drags from the top to unmaximize and maximize.
	w, err := ash.GetWindow(ctx, tconn, wID)
	if err != nil {
		s.Fatal("Failed to get window info: ", err)
	}
	top := w.BoundsInRoot.TopCenter()
	const (
		// dragDuration is the duration of each straight-
		// line part of the drag.
		dragDuration = 250 * time.Millisecond
		// holdDuration is how long the drag pauses after
		// going back to the top, to maximize the window.
		holdDuration = time.Second
	)
	dragUnmaximizeAndMaximize := pc.Drag(
		top,
		pc.DragTo(top.Add(coords.NewPoint(0, 100)), dragDuration),
		pc.DragTo(top, dragDuration),
		action.Sleep(holdDuration),
	)

	// Create and configure the metrics recorder.
	recorder, err := cujrecorder.NewRecorder(ctx, cr, nil, cujrecorder.RecorderOptions{})
	if err != nil {
		s.Fatal("Failed to create the recorder: ", err)
	}
	defer recorder.Close(cleanupCtx)
	if err := recorder.AddCommonMetrics(tconn, bTconn); err != nil {
		s.Fatal("Failed to add CUJ common metrics to the recorder: ", err)
	}
	if err := recorder.AddCollectedMetrics(tconn, browser.TypeAsh,
		cujrecorder.NewSmoothnessMetricConfig("Ash.Window.AnimationSmoothness.CrossFade"),
		cujrecorder.NewSmoothnessMetricConfig("Ash.Window.AnimationSmoothness.CrossFade.DragMaximize"),
		cujrecorder.NewSmoothnessMetricConfig("Ash.Window.AnimationSmoothness.CrossFade.DragUnmaximize"),
		cujrecorder.NewSmoothnessMetricConfig("Ash.Window.AnimationSmoothness.Minimize"),
		cujrecorder.NewSmoothnessMetricConfig("Ash.Window.AnimationSmoothness.Unminimize"),
	); err != nil {
		s.Fatal("Failed to add window animation smoothness metrics to the recorder: ", err)
	}
	recorder.EnableTracing(s.OutDir(), s.DataPath(cujrecorder.SystemTraceConfigFile))

	// Conduct the performance measurement.
	if err := recorder.RunFor(ctx, func(ctx context.Context) error {
		if err := dragUnmaximizeAndMaximize(ctx); err != nil {
			return errors.Wrap(err, "failed to drag to unmaximize and maximize the window")
		}
		if err := ash.WaitWindowFinishAnimating(ctx, tconn, wID); err != nil {
			return errors.Wrap(err, "failed to wait for the window animation")
		}
		for _, wState := range []ash.WindowStateType{
			ash.WindowStateNormal,
			ash.WindowStateMinimized,
			ash.WindowStateNormal,
			ash.WindowStateMaximized,
		} {
			if err := ash.SetWindowStateAndWait(ctx, tconn, wID, wState); err != nil {
				return errors.Wrapf(err, "failed to set window state to %q", wState)
			}
		}
		return nil
	}, 10*time.Second); err != nil {
		s.Fatal("Failed to conduct the performance measurement: ", err)
	}

	// Report the results.
	pv := perf.NewValues()
	if err := recorder.Record(ctx, pv); err != nil {
		s.Fatal("Failed to record the performance data: ", err)
	}
	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Failed to save the performance data: ", err)
	}
}
