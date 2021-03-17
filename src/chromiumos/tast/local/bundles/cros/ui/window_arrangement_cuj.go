// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/audio/crastestclient"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/bundles/cros/ui/perfutil"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/ui/pointer"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WindowArrangementCUJ,
		Desc:         "Measures the performance of critical user journey for window arrangements",
		Contacts:     []string{"yichenz@chromium.org", "chromeos-wmp@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome", "arc", "chrome_internal"},
		Vars:         []string{"record"},
		Timeout:      10 * time.Minute,
		Data:         []string{"bear-320x240.vp8.webm", "pip.html"},
		Fixture:      "loggedInToCUJUser",
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

	// Ensure display on to record ui performance correctly.
	if err := perfutil.EnsureDisplayOn(ctx); err != nil {
		s.Fatal("Failed to turn on display: ", err)
	}

	// Shorten context a bit to allow for cleanup.
	closeCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 2*time.Second)
	defer cancel()

	tabletMode := s.Param().(bool)

	cr := s.FixtValue().(cuj.FixtureData).Chrome

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to the test API connection: ", err)
	}

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, tabletMode)
	if err != nil {
		s.Fatal("Failed to ensure clamshell/tablet mode: ", err)
	}
	defer cleanup(closeCtx)

	info, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get the primary display info: ", err)
	}

	tabChecker, err := cuj.NewTabCrashChecker(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to create TabCrashChecker: ", err)
	}

	if _, ok := s.Var("record"); ok {
		screenRecorder, err := uiauto.NewScreenRecorder(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to create ScreenRecorder: ", err)
		}
		defer func() {
			screenRecorder.Stop(ctx)
			dir, ok := testing.ContextOutDir(ctx)
			if ok && dir != "" {
				if _, err := os.Stat(dir); err == nil {
					testing.ContextLogf(ctx, "Saving screen record to %s", dir)
					if err := screenRecorder.SaveInBytes(ctx, filepath.Join(dir, "screen_record.webm")); err != nil {
						s.Fatal("Failed to save screen record in bytes: ", err)
					}
				}
			}
			screenRecorder.Release(ctx)
		}()
		screenRecorder.Start(ctx, tconn)
	}

	// Set up the cuj.Recorder: In clamshell mode, this test will measure the combinations of
	// input latency of tab dragging and of window resizing and of split view resizing, and
	// also the percent of dropped frames of video; In tablet mode, this test will measure
	// the combinations of input latency of split view resizing and the percent of dropped frames
	// of video.
	var configs []cuj.MetricConfig
	if !tabletMode {
		configs = []cuj.MetricConfig{
			cuj.NewLatencyMetricConfig("Ash.WorkspaceWindowResizer.TabDragging.PresentationTime.ClamshellMode"),
			cuj.NewLatencyMetricConfig("Ash.InteractiveWindowResize.TimeToPresent"),
			cuj.NewLatencyMetricConfig("Ash.SplitViewResize.PresentationTime.ClamshellMode.SingleWindow"),
			cuj.NewCustomMetricConfig(
				"Graphics.Smoothness.PercentDroppedFrames.CompositorThread.Video",
				"percent", perf.SmallerIsBetter, []int64{50, 80}),
		}
	} else {
		configs = []cuj.MetricConfig{
			cuj.NewLatencyMetricConfig("Ash.SplitViewResize.PresentationTime.TabletMode.MultiWindow"),
			cuj.NewCustomMetricConfig(
				"Graphics.Smoothness.PercentDroppedFrames.CompositorThread.Video",
				"percent", perf.SmallerIsBetter, []int64{50, 80}),
		}
	}

	recorder, err := cuj.NewRecorder(ctx, tconn, configs...)
	if err != nil {
		s.Fatal("Failed to create a recorder: ", err)
	}
	defer recorder.Close(closeCtx)

	if err := crastestclient.Mute(ctx); err != nil {
		s.Fatal("Failed to mute audio: ", err)
	}
	defer crastestclient.Unmute(closeCtx)

	defer faillog.DumpUITreeOnError(closeCtx, s.OutDir(), s.HasError, tconn)

	srv := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer srv.Close()

	connPiP, err := cr.NewConn(ctx, srv.URL+"/pip.html")
	if err != nil {
		s.Fatal("Failed to load pip.html: ", err)
	}
	defer connPiP.Close()
	if err := webutil.WaitForQuiescence(ctx, connPiP, timeout); err != nil {
		s.Fatal("Failed to wait for pip.html to achieve quiescence: ", err)
	}

	connNoPiP, err := cr.NewConn(ctx, srv.URL+"/pip.html")
	if err != nil {
		s.Fatal("Failed to load pip.html: ", err)
	}
	defer connNoPiP.Close()
	if err := webutil.WaitForQuiescence(ctx, connNoPiP, timeout); err != nil {
		s.Fatal("Failed to wait for pip.html to achieve quiescence: ", err)
	}

	ui := uiauto.New(tconn)

	// The second tab enters the system PiP mode.
	webview := nodewith.ClassName("ContentsWebView").Role(role.WebView)
	// Assume that the meeting code is the only textfield in the webpage.
	if err := ui.LeftClick(nodewith.Name("Enter Picture-in-Picture").Role(role.Button).Ancestor(webview))(ctx); err != nil {
		s.Fatal("Failed to click the pip button: ", err)
	}
	if err := webutil.WaitForQuiescence(ctx, connPiP, timeout); err != nil {
		s.Fatal("Failed to wait for quiescence: ", err)
	}

	ws, err := ash.GetAllWindows(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to obtain the window list: ", err)
	}
	id0 := ws[0].ID
	if !tabletMode {
		// In clamshell mode, turn the window into normal state.
		if err := ash.SetWindowStateAndWait(ctx, tconn, id0, ash.WindowStateNormal); err != nil {
			s.Fatal("Failed to set the window state to normal: ", err)
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
	defer pc.Close()

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
			newTabButton := nodewith.Name("New Tab")
			newTabButtonRect, err := ui.Location(ctx, newTabButton)
			if err != nil {
				return errors.Wrap(err, "failed to get the location of the new tab button")
			}
			tabStripGapPt := coords.NewPoint(newTabButtonRect.Right()+10, newTabButtonRect.Top)
			testing.ContextLog(ctx, "Dragging the window")
			if err := pointer.Drag(ctx, pc, tabStripGapPt, middlePt, duration); err != nil {
				return errors.Wrap(err, "failed to drag window from the tab strip point to the middle")
			}
			if err := pointer.Drag(ctx, pc, middlePt, tabStripGapPt, duration); err != nil {
				return errors.Wrap(err, "failed to drag window back from the middle")
			}

			// Maximize window.
			maximizeButton := nodewith.Name("Maximize").ClassName("FrameCaptionButton").Role(role.Button)
			if err := ui.LeftClick(maximizeButton)(ctx); err != nil {
				return errors.Wrap(err, "failed to maximize the window")
			}
			if err := ash.WaitForCondition(ctx, tconn, func(w *ash.Window) bool {
				return w.ID == id0 && w.State == ash.WindowStateMaximized && !w.IsAnimating
			}, &testing.PollOptions{Timeout: timeout}); err != nil {
				return errors.Wrap(err, "failed to wait for window to become maximized")
			}

			// Minimize window.
			minimizeButton := nodewith.Name("Minimize").ClassName("FrameCaptionButton").Role(role.Button)
			if err := ui.LeftClick(minimizeButton)(ctx); err != nil {
				return errors.Wrap(err, "failed to minimize the window")
			}
			if err := ash.WaitForCondition(ctx, tconn, func(w *ash.Window) bool {
				return w.ID == id0 && w.State == ash.WindowStateMinimized && !w.IsAnimating
			}, &testing.PollOptions{Timeout: timeout}); err != nil {
				return errors.Wrap(err, "failed to wait for window to become minimized")
			}

			// Snap the window to the left and drag the second tab to snap to the right.
			if _, err := ash.SetWindowState(ctx, tconn, id0, ash.WMEventNormal); err != nil {
				return errors.Wrap(err, "failed to set the window state to normal")
			}
			if err := ash.WaitForCondition(ctx, tconn, func(w *ash.Window) bool {
				return w.ID == id0 && w.State == ash.WindowStateNormal && !w.IsAnimating
			}, &testing.PollOptions{Timeout: timeout}); err != nil {
				return errors.Wrap(err, "failed to wait for window to become normal")
			}
			testing.ContextLog(ctx, "Snapping the window to the left")
			if err := pointer.Drag(ctx, pc, tabStripGapPt, snapLeftPoint, duration); err != nil {
				return errors.Wrap(err, "failed to snap the window to the left")
			}
			if err := ash.WaitForCondition(ctx, tconn, func(w *ash.Window) bool {
				return w.ID == id0 && w.State == ash.WindowStateLeftSnapped && !w.IsAnimating
			}, &testing.PollOptions{Timeout: timeout}); err != nil {
				return errors.Wrap(err, "failed to wait for window to be left snapped")
			}
			testing.ContextLog(ctx, "Snapping the second tab to the right")
			firstTabRect, err := ui.Location(ctx, nodewith.Role(role.Tab).ClassName("Tab").First())
			if err != nil {
				return errors.Wrap(err, "failed to get the location of the first tab")
			}
			if err := pointer.Drag(ctx, pc, firstTabRect.CenterPoint(), snapRightPoint, duration); err != nil {
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
				}
				return errors.New("windows are not snapped yet")
			}, &testing.PollOptions{Timeout: timeout}); err != nil {
				return errors.Wrap(err, "failed to wait for windows to be snapped correctly")
			}

			// Split view resizing. Some preparations need to be done before dragging the divider in
			// order to collect Ash.SplitViewResize.PresentationTime.SingleWindow. It must have a snapped
			// window and an overview grid to be able to collect the metrics for SplitViewController.
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
			// Snap one of the window to the left from the overview grid.
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
			// Drag the first window from overview grid to snap.
			if err := pointer.Drag(ctx, pc, w.OverviewInfo.Bounds.CenterPoint(), snapLeftPoint, duration); err != nil {
				return errors.Wrap(err, "failed to drag window from overview to snap")
			}
			w, err = ash.FindFirstWindowInOverview(ctx, tconn)
			if err != nil {
				return errors.Wrap(err, "failed to find the window in the overview mode to drag to snap")
			}
			deskMiniViews, err := ui.NodesInfo(ctx, nodewith.ClassName("DeskMiniView"))
			if err != nil {
				return errors.Wrap(err, "failed to get desk mini-views")
			}
			if deskMiniViewCount := len(deskMiniViews); deskMiniViewCount != 2 {
				return errors.Wrapf(err, "expected 2 desk mini-views; found %v", deskMiniViewCount)
			}
			// Drag the second window to another desk to obtain an empty overview grid.
			if err := pointer.Drag(ctx, pc, w.OverviewInfo.Bounds.CenterPoint(), deskMiniViews[1].Location.CenterPoint(), time.Second); err != nil {
				return errors.Wrap(err, "failed to drag window from overview grid to desk mini-view")
			}
			// Wait for 2 seconds for location-change events to be completed.
			if err := testing.Sleep(ctx, 2*time.Second); err != nil {
				return errors.Wrap(err, "failed to wait for location-change events to be completed")
			}

			// Drag divider.
			testing.ContextLog(ctx, "Dragging the divider")
			if err := pc.Press(ctx, splitViewDragPoints[0]); err != nil {
				return errors.Wrap(err, "failed to start divider drag")
			}
			if err := pc.Move(ctx, splitViewDragPoints[0], splitViewDragPoints[1], duration); err != nil {
				return errors.Wrap(err, "failed to drag divider slightly right")
			}
			if err := pc.Move(ctx, splitViewDragPoints[1], splitViewDragPoints[2], duration); err != nil {
				return errors.Wrap(err, "failed to drag divider all the way left")
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
		// tab dragging and split view resizing.
		f = func(ctx context.Context) error {
			// Drag the second tab to snap to the right.
			tabs, err := ui.NodesInfo(ctx, nodewith.Role(role.Tab))
			if err != nil {
				return errors.Wrap(err, "failed to find children nodes of the tabs")
			}
			if len(tabs) != 2 {
				return errors.Errorf("failed to get the second tab, expected 2 tabs, got %v", len(tabs))
			}
			tab2 := tabs[1]
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
				}
				return errors.New("windows are not snapped yet")
			}, &testing.PollOptions{Timeout: timeout}); err != nil {
				return errors.Wrap(err, "failed to wait for windows to be snapped correctly")
			}

			// Split view resizing by dragging the divider.
			testing.ContextLog(ctx, "Dragging the divider")
			if err := pc.Press(ctx, splitViewDragPoints[0]); err != nil {
				return errors.Wrap(err, "failed to start divider drag")
			}
			if err := pc.Move(ctx, splitViewDragPoints[0], splitViewDragPoints[1], duration); err != nil {
				return errors.Wrap(err, "failed to drag divider slightly right")
			}
			if err := pc.Move(ctx, splitViewDragPoints[1], splitViewDragPoints[2], duration); err != nil {
				return errors.Wrap(err, "failed to drag divider all the way left")
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
	if err := recorder.Run(ctx, f); err != nil {
		s.Fatal("Failed to conduct the recorder task: ", err)
	}

	// Check if there is any tab crashed.
	if err := tabChecker.Check(ctx); err != nil {
		s.Fatal("Tab renderer crashed: ", err)
	}

	// Store perf metrics.
	pv := perf.NewValues()
	if err := recorder.Record(ctx, pv); err != nil {
		s.Fatal("Failed to record the data: ", err)
	}
	if err := pv.Save(s.OutDir()); err != nil {
		s.Fatal("Failed to save the perf data: ", err)
	}
}
