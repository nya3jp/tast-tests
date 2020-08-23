// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/audio"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/bundles/cros/ui/pointer"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/ui"
	chromeui "chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WindowArrangementCUJ,
		Desc:         "Measures the performance of critical user journey for window arrangements",
		Contacts:     []string{"yichenz@chromium.org"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome", "arc"},
		Timeout:      10 * time.Minute,
		Vars: []string{
			"ui.cuj_username",
			"ui.cuj_password",
		},
		Pre: cuj.LoggedInToCUJUser(),
		Params: []testing.Param{
			{
				Name: "clamshell_mode",
				Val:  false,
			},
			{
				Name:              "tablet_mode",
				Val:               true,
				ExtraSoftwareDeps: []string{"tablet_mode"},
			},
		},
	})
}

func WindowArrangementCUJ(ctx context.Context, s *testing.State) {
	const (
		timeout  = 10 * time.Second
		duration = 2 * time.Second
	)

	tabletMode := s.Param().(bool)

	cr := s.PreValue().(cuj.PreData).Chrome

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to the test API connection: ", err)
	}

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, tabletMode)
	if err != nil {
		s.Fatal("Failed to ensure clamshell/tablet mode: ", err)
	}
	defer cleanup(ctx)

	info, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get the primary display info: ", err)
	}

	tabChecker, err := cuj.NewTabCrashChecker(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to create TabCrashChecker: ", err)
	}

	if err := audio.Mute(ctx); err != nil {
		s.Fatal("Failed to mute audio: ", err)
	}
	// defer audio.Unmute(ctx)

	// Open two youtube tabs: the second tab will be in pip mode, the first will not.
	connNoPiP, err := cr.NewConn(ctx, "https://www.youtube.com/watch?v=yn3HwLBphW8")
	if err != nil {
		s.Fatal("Failed to open the first youtube tab: ", err)
	}
	defer connNoPiP.Close()
	connPiP, err := cr.NewConn(ctx, "https://www.youtube.com/watch?v=yn3HwLBphW8")
	if err != nil {
		s.Fatal("Failed to open the second youtube tab: ", err)
	}
	defer connPiP.Close()
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// Set the video in the second tab into PiP mode.
	webview, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{Role: ui.RoleTypeWebView, ClassName: "WebView"}, timeout)
	if err != nil {
		s.Fatal("Failed to find webview: ", err)
	}
	defer webview.Release(ctx)
	pipButton, err := webview.DescendantWithTimeout(ctx, ui.FindParams{Role: ui.RoleTypeButton, ClassName: "ytp-miniplayer-button ytp-button"}, timeout)
	if err != nil {
		s.Fatal("Failed to find the pip button ", err)
	}
	defer pipButton.Release(ctx)
	if err := pipButton.LeftClick(ctx); err != nil {
		s.Fatal("Failed to click on the pip button ", err)
	}
	if err := webutil.WaitForQuiescence(ctx, connPiP, timeout); err != nil {
		s.Fatal("Failed to wait for quiescence ", err)
	}

	ws, err := ash.GetAllWindows(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to obtain the window list: ", err)
	}
	id0 := ws[0].ID
	if !tabletMode {
		// In clamshell mode, turn the window into normal state.
		if _, err := ash.SetWindowState(ctx, tconn, id0, ash.WMEventNormal); err != nil {
			s.Fatal("Failed to set the window state to normal: ", err)
		}
		if err := ash.WaitWindowFinishAnimating(ctx, tconn, id0); err != nil {
			s.Fatal("Failed to wait for top window animation: ", err)
		}
	}
	w0, err := ash.GetWindow(ctx, tconn, id0)
	if err != nil {
		s.Fatal("Failed to get the window: ", err)
	}

	var pc pointer.Controller
	if !tabletMode {
		pc = pointer.NewMouseController(tconn)
	} else {
		pc, err = pointer.NewTouchController(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to create a touch controller: ", err)
		}
	}

	// Set up the cuj.Recorder: this test will measure the combinations of
	// input latency for tab dragging and for window resizing and for split
	// view resizing, and also the percent of dropped frames for video.
	configs := []cuj.MetricConfig{
		cuj.NewLatencyMetricConfig("Ash.WorkspaceWindowResizer.TabDragging.PresentationTime.ClamshellMode"),
		cuj.NewLatencyMetricConfig("Ash.InteractiveWindowResize.TimeToPresent"),
		cuj.NewCustomMetricConfig(
			"Graphics.Smoothness.PercentDroppedFrames.CompositorThread.Video",
			"percent", perf.SmallerIsBetter, []int64{50, 80}),
	}
	for _, suffix := range []string{"ClamshellMode.SingleWindow", "TabletMode.SingleWindow", "TabletMode.MultiWindow"} {
		configs = append(configs, cuj.NewLatencyMetricConfig(
			"Ash.SplitViewResize.PresentationTime."+suffix))
	}
	recorder, err := cuj.NewRecorder(ctx, configs...)
	if err != nil {
		s.Fatal("Failed to create a recorder: ", err)
	}

	splitViewDragPoints := []coords.Point{
		info.WorkArea.CenterPoint(),
		coords.NewPoint(info.WorkArea.Left+info.WorkArea.Width-1, info.WorkArea.CenterY()),
		coords.NewPoint(info.WorkArea.Left+info.WorkArea.Width/4, info.WorkArea.CenterY()),
	}
	snapLeftPoint := coords.NewPoint(info.WorkArea.Left+1, info.WorkArea.CenterY())
	snapRightPoint := coords.NewPoint(info.WorkArea.Right()-1, info.WorkArea.CenterY())

	var f func(ctx context.Context) error
	if !tabletMode {
		// In clamshell mode, we test performance for resizing window, dragging window,
		// maximizing window, minimizing window and split view resizing.
		f = func(ctx context.Context) error {

			// Resize window.
			if w0.State != ash.WindowStateNormal {
				return errors.Errorf("Wrong window state: expected Normal, got %s", w0.State)
			}
			bounds := w0.BoundsInRoot
			upperLeftPt := coords.NewPoint(bounds.Left, bounds.Top)
			middlePt := coords.NewPoint(bounds.Left+bounds.Width/2, bounds.Top+bounds.Height/2)
			testing.ContextLog(ctx, "Resizing the window")
			if err := pointer.Drag(ctx, pc, upperLeftPt, middlePt, duration); err != nil {
				return errors.Wrap(err, "failed to resize window from the upper left to the middle")
			}
			if err := pointer.Drag(ctx, pc, middlePt, upperLeftPt, duration); err != nil {
				return errors.Wrap(err, "failed to resize window back from the middle")
			}

			// Drag window.
			tabs, err := chromeui.FindAll(ctx, tconn, chromeui.FindParams{Role: chromeui.RoleTypeTab, ClassName: "Tab"})
			if err != nil {
				return errors.Wrap(err, "failed to find tabs")
			}
			defer tabs.Release(ctx)
			if len(tabs) != 2 {
				return errors.Errorf("expected 2 tabs, only found %v tab(s)", len(tabs))
			}
			tabStripGapPt := coords.NewPoint(tabs[1].Location.CenterX(), (tabs[1].Location.Top+bounds.Top)/2)
			testing.ContextLog(ctx, "Dragging the window")
			if err := pointer.Drag(ctx, pc, tabStripGapPt, middlePt, duration); err != nil {
				return errors.Wrap(err, "failed to drag window from the tab strip point to the middle")
			}
			if err := pointer.Drag(ctx, pc, middlePt, tabStripGapPt, duration); err != nil {
				return errors.Wrap(err, "failed to drag window back from the middle")
			}

			// Maximize window.
			maximizeButton, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{Role: ui.RoleTypeButton, ClassName: "FrameCaptionButton", Name: "Maximize"}, timeout)
			if err != nil {
				return errors.Wrap(err, "failed to find maximize button")
			}
			defer maximizeButton.Release(ctx)
			testing.ContextLog(ctx, "Maximizing the window")
			if err := maximizeButton.LeftClick(ctx); err != nil {
				return errors.Wrap(err, "failed to maximize the window")
			}
			if err := ash.WaitForCondition(ctx, tconn, func(w *ash.Window) bool {
				return w.ID == id0 && w.State == ash.WindowStateMaximized
			}, &testing.PollOptions{Timeout: timeout}); err != nil {
				return errors.Wrap(err, "failed to wait for window to become maximized")
			}

			// Minimize window.
			minimizeButton, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{Role: ui.RoleTypeButton, ClassName: "FrameCaptionButton", Name: "Minimize"}, timeout)
			if err != nil {
				return errors.Wrap(err, "failed to find minimize button")
			}
			defer minimizeButton.Release(ctx)
			testing.ContextLog(ctx, "Minimizing the window")
			if err := minimizeButton.LeftClick(ctx); err != nil {
				return errors.Wrap(err, "failed to minimize the window")
			}
			if err := ash.WaitForCondition(ctx, tconn, func(w *ash.Window) bool {
				return w.ID == id0 && w.State == ash.WindowStateMinimized
			}, &testing.PollOptions{Timeout: timeout}); err != nil {
				return errors.Wrap(err, "failed to wait for window to become minimized")
			}

			// Snap the window to the left and drag the second tab to snap to the right.
			if _, err := ash.SetWindowState(ctx, tconn, w0.ID, ash.WMEventNormal); err != nil {
				return errors.Wrap(err, "failed to set the window state to normal")
			}
			if err := ash.WaitWindowFinishAnimating(ctx, tconn, w0.ID); err != nil {
				return errors.Wrap(err, "failed to wait for top window animation")
			}
			testing.ContextLog(ctx, "Snapping the window to the left")
			if err := pointer.Drag(ctx, pc, tabStripGapPt, snapLeftPoint, duration); err != nil {
				return errors.Wrap(err, "failed to snap the window to the left")
			}

			testing.ContextLog(ctx, "Snapping the second tab to the right")
			tabs, err = chromeui.FindAll(ctx, tconn, chromeui.FindParams{Role: chromeui.RoleTypeTab, ClassName: "Tab"})
			if err != nil {
				return errors.Wrap(err, "failed to find tabs")
			}
			defer tabs.Release(ctx)
			if len(tabs) != 2 {
				return errors.Errorf("expected 2 tabs, only found %v tab(s)", len(tabs))
			}
			if err := pointer.Drag(ctx, pc, tabs[1].Location.CenterPoint(), snapRightPoint, duration); err != nil {
				return errors.Wrap(err, "failed to snap the second tab to the right")
			}

			ws, err := ash.GetAllWindows(ctx, tconn)
			if err != nil {
				return errors.Wrap(err, "failed to obtain the window list")
			}
			if err := testing.Poll(ctx, func(ctx context.Context) error {
				if len(ws) != 2 {
					return errors.Errorf("should be 2 windows, got %v", len(ws))
				}
				if (ws[1].State == ash.WindowStateLeftSnapped && ws[0].State == ash.WindowStateRightSnapped) ||
					(ws[0].State == ash.WindowStateLeftSnapped && ws[1].State == ash.WindowStateRightSnapped) {
					return nil
				} else {
					return errors.New("windows are not snapped yet")
				}
			}, &testing.PollOptions{Timeout: timeout}); err != nil {
				return errors.Wrap(err, "failed to wait for windows to be snapped correctly")
			}

			// Split view resizing. Referring to SplitViewResizerPerf, some preparations need to be done
			// before dragging the divider in order to collect Ash.SplitViewResize.PresentationTime.SingleWindow.
			kw, err := input.Keyboard(ctx)
			if err != nil {
				return errors.Wrap(err, "failed to open the keyboard")
			}
			defer kw.Close()
			// Enter the overview mode.
			topRow, err := input.KeyboardTopRowLayout(ctx, kw)
			if err != nil {
				return errors.Wrap(err, "failed to obtain the top-row layout")
			}
			if err = kw.Accel(ctx, topRow.SelectTask); err != nil {
				return errors.Wrap(err, "failed to enter overview mode")
			}
			// Snap one of the window to the left from the overview mode.
			if err := ash.CreateNewDesk(ctx, tconn); err != nil {
				return errors.Wrap(err, "failed to create a new desk")
			}
			w, err := ash.FindFirstWindowInOverview(ctx, tconn)
			if err != nil {
				return errors.Wrap(err, "failed to find the window in the overview mode")
			}
			// Wait for 2 seconds for location-change events to be completed.
			if err := testing.Sleep(ctx, 2*time.Second); err != nil {
				return errors.Wrap(err, "failed to wait for location-change events to be completed")
			}
			if err := pointer.Drag(ctx, pc, w.OverviewInfo.Bounds.CenterPoint(), snapLeftPoint, duration); err != nil {
				return errors.Wrap(err, "failed to drag window from overview to snap")
			}
			w, err = ash.FindFirstWindowInOverview(ctx, tconn)
			if err != nil {
				return errors.Wrap(err, "failed to find the window in the overview mode to drag to snap")
			}
			deskMiniViews, err := chromeui.FindAll(ctx, tconn, chromeui.FindParams{ClassName: "DeskMiniView"})
			if err != nil {
				return errors.Wrap(err, "failed to get desk mini-views")
			}
			defer deskMiniViews.Release(ctx)
			if deskMiniViewCount := len(deskMiniViews); deskMiniViewCount != 2 {
				return errors.Wrapf(err, "expected 2 desk mini-views; found %v", deskMiniViewCount)
			}
			if err := pointer.Drag(ctx, pc, w.OverviewInfo.Bounds.CenterPoint(), deskMiniViews[1].Location.CenterPoint(), time.Second); err != nil {
				return errors.Wrap(err, "failed to drag window from overview grid to desk mini-view")
			}
			// Wait for 2 seconds for location-change events to be completed.
			if err := testing.Sleep(ctx, 2*time.Second); err != nil {
				return errors.Wrap(err, "failed to wait for location-change events to be completed")
			}
			// Drag divider
			testing.ContextLog(ctx, "Dragging the divider")
			if err := pc.Press(ctx, splitViewDragPoints[0]); err != nil {
				return errors.Wrap(err, "failed to start divider drag")
			}
			if err := pc.Move(ctx, splitViewDragPoints[0], splitViewDragPoints[1], duration); err != nil {
				return errors.Wrap(err, "failed to drag divider slightly left")
			}
			if err := pc.Move(ctx, splitViewDragPoints[1], splitViewDragPoints[2], duration); err != nil {
				return errors.Wrap(err, "failed to drag divider all the way right")
			}
			if err := pc.Move(ctx, splitViewDragPoints[2], splitViewDragPoints[0], duration); err != nil {
				return errors.Wrap(err, "failed to drag divider back to the center")
			}
			if err := pc.Release(ctx); err != nil {
				return errors.Wrap(err, "failed to end divider drag")
			}
			return nil
		}
	} else {
		// In tablet mode, since windows are always maximized, we only test performance for
		// split view resizing.
		f = func(ctx context.Context) error {
			// Snap the window to the left and drag the second tab to snap to the right.
			tabsButton, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{Role: ui.RoleTypeButton, Name: "2 open tabs, press to toggle tab strip"}, timeout)
			if err != nil {
				return errors.Wrap(err, "failed to find the tab list button")
			}
			defer tabsButton.Release(ctx)
			if err := tabsButton.LeftClick(ctx); err != nil {
				return errors.Wrap(err, "failed to click on the tab list button")
			}
			tab2, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{Role: ui.RoleTypeTab, Name: "YouTube - Audio playing"}, timeout)
			if err != nil {
				return errors.Wrap(err, "failed to find the second tab")
			}
			defer tab2.Release(ctx)
			testing.ContextLog(ctx, "Snapping the second tab to the right")
			if err := pc.Press(ctx, tab2.Location.CenterPoint()); err != nil {
				return errors.Wrap(err, "failed to start drag the second tab to snap to the right")
			}
			if err := testing.Sleep(ctx, time.Second); err != nil {
				return errors.Wrap(err, "failed to wait for touch to become long press, for dragging the second tab from the window to snap")
			}
			if err := pc.Move(ctx, tab2.Location.CenterPoint(), snapRightPoint, duration); err != nil {
				return errors.Wrap(err, "failed to drag the second tab to snap")
			}
			if err := pc.Release(ctx); err != nil {
				return errors.Wrap(err, "failed to end tab drag to snap to the right")
			}
			ws, err := ash.GetAllWindows(ctx, tconn)
			if err != nil {
				return errors.Wrap(err, "failed to obtain the window list")
			}
			if err := testing.Poll(ctx, func(ctx context.Context) error {
				if len(ws) != 2 {
					return errors.Errorf("should be 2 windows, got %v", len(ws))
				}
				if (ws[1].State == ash.WindowStateLeftSnapped && ws[0].State == ash.WindowStateRightSnapped) ||
					(ws[0].State == ash.WindowStateLeftSnapped && ws[1].State == ash.WindowStateRightSnapped) {
					return nil
				} else {
					return errors.New("windows are not snapped yet")
				}
			}, &testing.PollOptions{Timeout: timeout}); err != nil {
				return errors.Wrap(err, "failed to wait for windows to be snapped correctly")
			}
			// Split view resizing.
			testing.ContextLog(ctx, "Dragging the divider")
			if err := pc.Press(ctx, splitViewDragPoints[0]); err != nil {
				return errors.Wrap(err, "failed to start divider drag")
			}
			if err := pc.Move(ctx, splitViewDragPoints[0], splitViewDragPoints[1], duration); err != nil {
				return errors.Wrap(err, "failed to drag divider slightly left")
			}
			if err := pc.Move(ctx, splitViewDragPoints[1], splitViewDragPoints[2], duration); err != nil {
				return errors.Wrap(err, "failed to drag divider all the way right")
			}
			if err := pc.Move(ctx, splitViewDragPoints[2], splitViewDragPoints[0], duration); err != nil {
				return errors.Wrap(err, "failed to drag divider back to the center")
			}
			if err := pc.Release(ctx); err != nil {
				return errors.Wrap(err, "failed to end divider drag")
			}
			return nil
		}
	}

	// Run the recorder.
	if err := recorder.Run(ctx, tconn, f); err != nil {
		s.Fatal("Failed to conduct the recorder task: ", err)
	}
	// Store perf metrics.
	pv := perf.NewValues()
	if err := recorder.Record(pv); err != nil {
		s.Fatal("Failed to record the data: ", err)
	}
	if err := pv.Save(s.OutDir()); err != nil {
		s.Fatal("Failed to save the perf data: ", err)
	}

	// Check if there is any tab crashed.
	if err := tabChecker.Check(ctx); err != nil {
		s.Fatal("Tab renderer crashed: ", err)
	}
}
