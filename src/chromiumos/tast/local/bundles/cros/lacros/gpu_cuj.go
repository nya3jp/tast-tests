// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package lacros tests linux-chrome running on ChromeOS.
package lacros

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/lacros/launcher"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/cdputil"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/local/chrome/ui"
	chromeui "chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/testing"
)

type testType string
type chromeType string

const (
	testTypeMaximized testType = "maximized"
	testTypeThreeDot  testType = "threedot"
	testTypeResize    testType = "resize"

	testDuration time.Duration = 20 * time.Second
	dragInsetDp  int           = 5
	insetSlopDp  int           = 25

	chromeTypeCros   = "cros"
	chromeTypeLacros = "lacros"
)

type gpuCUJTestParams struct {
	url      string
	testType testType
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         GpuCUJ,
		Desc:         "Lacros GPU performance CUJ tests",
		Contacts:     []string{"edcourtney@chromium.org", "hidehiko@chromium.org", "lacros-team@google.com"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      60 * time.Minute,
		Data:         []string{launcher.DataArtifact},
		Params: []testing.Param{{
			Name: "aquarium_composited",
			Val: gpuCUJTestParams{
				url:      "https://webglsamples.org/aquarium/aquarium.html",
				testType: testTypeMaximized,
			},
			Pre: launcher.StartedByDataForceComposition(),
		}, {
			Name: "aquarium_threedot",
			Val: gpuCUJTestParams{
				url:      "https://webglsamples.org/aquarium/aquarium.html",
				testType: testTypeThreeDot,
			},
			Pre: launcher.StartedByData(),
		}, {
			Name: "aquarium_resize",
			Val: gpuCUJTestParams{
				url:      "https://webglsamples.org/aquarium/aquarium.html",
				testType: testTypeResize,
			},
			Pre: launcher.StartedByData(),
		}, {
			Name: "aquarium",
			Val: gpuCUJTestParams{
				url:      "https://webglsamples.org/aquarium/aquarium.html",
				testType: testTypeMaximized,
			},
			Pre: launcher.StartedByData(),
		}, {
			Name: "poster_composited",
			Val: gpuCUJTestParams{
				url:      "https://webkit.org/blog-files/3d-transforms/poster-circle.html",
				testType: testTypeMaximized,
			},
			Pre: launcher.StartedByDataForceComposition(),
		}, {
			Name: "poster_threedot",
			Val: gpuCUJTestParams{
				url:      "https://webkit.org/blog-files/3d-transforms/poster-circle.html",
				testType: testTypeThreeDot,
			},
			Pre: launcher.StartedByData(),
		}, {
			Name: "poster_resize",
			Val: gpuCUJTestParams{
				url:      "https://webkit.org/blog-files/3d-transforms/poster-circle.html",
				testType: testTypeResize,
			},
			Pre: launcher.StartedByData(),
		}, {
			Name: "poster",
			Val: gpuCUJTestParams{
				url:      "https://webkit.org/blog-files/3d-transforms/poster-circle.html",
				testType: testTypeMaximized,
			},
			Pre: launcher.StartedByData(),
		}},
	})
}

// This test deals with both ChromeOS chrome and Linux chrome. In order to reduce confusion,
// we adopt the following naming convention for chrome.TestConn objects:
//   ctconn: chrome.TestConn to ChromeOS chrome.
//   ltconn: chrome.TestConn to Linux chrome.
//   tconn: chrome.TestConn to either ChromeOS or Linux chrome, i.e. both are usable.

var pollOptions = &testing.PollOptions{Timeout: 10 * time.Second}

func waitForWindowState(ctx context.Context, ctconn *chrome.TestConn, windowID int, state ash.WindowStateType) error {
	return ash.WaitForCondition(ctx, ctconn, func(w *ash.Window) bool {
		// Wait for the window given by |w| to be in the given |state| and also not be animating.
		return windowID == w.ID && w.State == state && !w.IsAnimating
	}, pollOptions)
}

func leftClickLacros(ctx context.Context, ctconn *chrome.TestConn, windowID int, n *ui.Node) error {
	if err := n.Update(ctx); err != nil {
		return errors.Wrap(err, "failed to update the node's location")
	}
	if n.Location.Empty() {
		return errors.New("this node doesn't have a location on the screen and can't be clicked")
	}
	w, err := ash.GetWindow(ctx, ctconn, windowID)
	if err != nil {
		return err
	}
	// Compute the node coordinates in cros-chrome root window coordinate space by
	// adding the top left coordinate of the linux-chrome window in cros-chrome root window coorindates.
	return ash.MouseClick(ctx, ctconn, w.BoundsInRoot.TopLeft().Add(n.Location.CenterPoint()), ash.LeftButton)
}

func toggleThreeDotMenu(ctx context.Context, tconn *chrome.TestConn, clickFn func(*ui.Node) error) error {
	// Find and click the three dot menu via UI.
	params := ui.FindParams{
		Role:      ui.RoleTypePopUpButton,
		ClassName: "BrowserAppMenuButton",
	}
	menu, err := chromeui.FindWithTimeout(ctx, tconn, params, 10*time.Second)
	if err != nil {
		return errors.Wrap(err, "failed to find the three dot menu")
	}
	defer menu.Release(ctx)

	if err := clickFn(menu); err != nil {
		return errors.Wrap(err, "failed to click three dot menu")
	}
	return nil
}

func toggleTraySetting(ctx context.Context, tconn *chrome.TestConn, name string) error {
	// Find and click the StatusArea via UI. Clicking it opens the Ubertray.
	params := ui.FindParams{
		ClassName: "ash/StatusAreaWidgetDelegate",
	}
	statusArea, err := chromeui.FindWithTimeout(ctx, tconn, params, 10*time.Second)
	if err != nil {
		return errors.Wrap(err, "failed to find the status area (time, battery, etc.)")
	}
	defer statusArea.Release(ctx)

	if err := statusArea.LeftClick(ctx); err != nil {
		return errors.Wrap(err, "failed to click status area")
	}

	// Find and click button in the Ubertray via UI.
	params = ui.FindParams{
		Name:      name,
		ClassName: "FeaturePodIconButton",
	}
	nbtn, err := chromeui.FindWithTimeout(ctx, tconn, params, 10*time.Second)
	if err != nil {
		return errors.Wrap(err, "failed to find button")
	}
	defer nbtn.Release(ctx)

	if err := nbtn.LeftClick(ctx); err != nil {
		return errors.Wrap(err, "failed to click button")
	}

	// Close StatusArea.
	if err := statusArea.LeftClick(ctx); err != nil {
		return errors.Wrap(err, "failed to click status area")
	}
	return nil
}

func findFirstWindow(ctx context.Context, ctconn *chrome.TestConn) (*ash.Window, error) {
	return ash.FindWindow(ctx, ctconn, func(w *ash.Window) bool {
		return true
	})
}

func setWindowState(ctx context.Context, ctconn *chrome.TestConn, windowID int, state ash.WindowStateType) error {
	windowEventMap := map[ash.WindowStateType]ash.WMEventType{
		ash.WindowStateNormal:     ash.WMEventNormal,
		ash.WindowStateMaximized:  ash.WMEventMaximize,
		ash.WindowStateMinimized:  ash.WMEventMinimize,
		ash.WindowStateFullscreen: ash.WMEventFullscreen,
	}
	wmEvent, ok := windowEventMap[state]
	if !ok {
		return errors.Errorf("didn't find the event for window state: %q", state)
	}
	if _, err := ash.SetWindowState(ctx, ctconn, windowID, wmEvent); err != nil {
		return err
	}
	return waitForWindowState(ctx, ctconn, windowID, state)
}

func setWindowBounds(ctx context.Context, ctconn *chrome.TestConn, windowID int, to coords.Rect) error {
	w, err := ash.GetWindow(ctx, ctconn, windowID)
	if err != nil {
		return err
	}

	info, err := display.GetInternalInfo(ctx, ctconn)
	if err != nil {
		return err
	}

	b, d, err := ash.SetWindowBounds(ctx, ctconn, w.ID, to, info.ID)
	if err != nil {
		return err
	}
	if b != to {
		return errors.Errorf("unable to set window bounds; got: %v, want: %v", b, to)
	}
	if d != info.ID {
		return errors.Errorf("unable to set window display; got: %v, want: %v", d, info.ID)
	}
	return nil
}

func closeAboutBlank(ctx context.Context, ds *cdputil.Session) error {
	targets, err := ds.FindTargets(ctx, chrome.MatchTargetURL(chrome.BlankURL))
	if err != nil {
		return errors.Wrap(err, "failed to query for about:blank pages")
	}
	for _, info := range targets {
		ds.CloseTarget(ctx, info.TargetID)
	}
	return nil
}

var histogramMap = map[string]struct {
	unit      string
	direction perf.Direction
}{
	"Graphics.Smoothness.PercentDroppedFrames.CompositorThread.Universal": {
		unit:      "percent",
		direction: perf.SmallerIsBetter,
	},
	"Graphics.Smoothness.PercentDroppedFrames.MainThread.Universal": {
		unit:      "percent",
		direction: perf.SmallerIsBetter,
	},
	"Graphics.Smoothness.PercentDroppedFrames.SlowerThread.Universal": {
		unit:      "percent",
		direction: perf.SmallerIsBetter,
	},
	"Graphics.Smoothness.PercentDroppedFrames.AllSequences": {
		unit:      "percent",
		direction: perf.SmallerIsBetter,
	},
	"Compositing.Display.DrawToSwapUs": {
		unit:      "ms",
		direction: perf.SmallerIsBetter,
	},
}

func runHistogram(ctx context.Context, tconn *chrome.TestConn, pv *perf.Values, perfFn func() error, testType string) error {
	var keys []string
	for k := range histogramMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	histograms, err := metrics.Run(ctx, tconn, perfFn, keys...)
	if err != nil {
		return errors.Wrap(err, "failed to get histograms")
	}

	for _, h := range histograms {
		testing.ContextLog(ctx, "Histogram: ", h)
		hinfo, ok := histogramMap[h.Name]
		if !ok {
			return errors.Wrapf(err, "failed to lookup histogram info: %s", h.Name)
		}

		if h.TotalCount() != 0 {
			mean, err := h.Mean()
			if err != nil {
				return errors.Wrapf(err, "failed to get mean for histogram: %s", h.Name)
			}
			testing.ContextLog(ctx, "Mean: ", mean)

			pv.Set(perf.Metric{
				Name:      fmt.Sprintf("%s.%s", h.Name, testType),
				Unit:      hinfo.unit,
				Direction: hinfo.direction,
			}, mean)
		}
	}
	return nil
}

// runTest runs the common part of the GpuCUJ performance test - that is, shared between ChromeOS chrome and Linux chrome.
// tconn is a test connection to the current browser being used (either ChromeOS or Linux chrome).
func runTest(ctx context.Context, pd launcher.PreData, pv *perf.Values, p gpuCUJTestParams, crt chromeType, tconn *chrome.TestConn) error {
	ctconn, err := pd.Chrome.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to connect to test API")
	}

	w, err := findFirstWindow(ctx, ctconn)
	if err != nil {
		return err
	}

	perfFn := func() error {
		testing.Sleep(ctx, testDuration)
		return nil
	}
	if p.testType == testTypeResize {
		// Restore window:
		if err := setWindowState(ctx, ctconn, w.ID, ash.WindowStateNormal); err != nil {
			return errors.Wrap(err, "failed to restore window")
		}

		info, err := display.GetInternalInfo(ctx, ctconn)
		if err != nil {
			return err
		}
		// Create a landscape rectangle. Avoid snapping by insetting by insetSlopDp.
		ms := math.Min(float64(info.WorkArea.Width), float64(info.WorkArea.Height))
		sb := coords.NewRect(info.WorkArea.Left, info.WorkArea.Top, int(ms), int(ms*0.6)).WithInset(insetSlopDp, insetSlopDp)
		if err := setWindowBounds(ctx, ctconn, w.ID, sb); err != nil {
			return errors.Wrap(err, "failed to set window initial bounds")
		}

		perfFn = func() error {
			// End bounds are just flipping the rectangle.
			// TODO(crbug.com/1067535): Subtract -1 to ensure drag-resize occurs for now.
			start := coords.NewPoint(sb.Left+sb.Width-1, sb.Top+sb.Height-1)
			end := coords.NewPoint(sb.Left+sb.Height, sb.Top+sb.Width)
			if err := ash.MouseDrag(ctx, ctconn, start, end, testDuration); err != nil {
				return errors.Wrap(err, "failed to drag resize")
			}
			return nil
		}
	} else {
		// Maximize window.
		if err := setWindowState(ctx, ctconn, w.ID, ash.WindowStateMaximized); err != nil {
			return errors.Wrap(err, "failed to maximize window")
		}
	}

	// Open the threedot menu if indicated.
	// TODO(edcourtney): Sometimes the accessibility tree isn't populated for linux chrome, which causes this code to fail.
	if p.testType == testTypeThreeDot {
		clickFn := func(n *ui.Node) error { return n.LeftClick(ctx) }
		if crt == chromeTypeLacros {
			clickFn = func(n *ui.Node) error { return leftClickLacros(ctx, ctconn, w.ID, n) }
		}
		if err := toggleThreeDotMenu(ctx, tconn, clickFn); err != nil {
			return errors.Wrap(err, "failed to open three dot menu")
		}
		defer toggleThreeDotMenu(ctx, tconn, clickFn)
	}

	testName := string(p.testType) + "." + string(crt)
	return runHistogram(ctx, tconn, pv, perfFn, testName)
}

func runLacrosTest(ctx context.Context, pd launcher.PreData, pv *perf.Values, p gpuCUJTestParams) error {
	// Launch linux-chrome with about:blank loaded first - we don't want to include startup cost.
	l, err := launcher.LaunchLinuxChrome(ctx, pd)
	if err != nil {
		return errors.Wrap(err, "failed to launch linux-chrome")
	}
	defer l.Close(ctx)

	ltconn, err := l.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to connect to test API")
	}

	// Wait for quiescent state.
	if err := cpu.WaitUntilIdle(ctx); err != nil {
		return errors.Wrap(err, "failed waiting for CPU to become idle")
	}

	conn, err := l.NewConn(ctx, p.url)
	if err != nil {
		return errors.Wrap(err, "failed to open new tab")
	}
	defer conn.Close()

	// Close the initial "about:blank" tab present at startup.
	if err := closeAboutBlank(ctx, l.Devsess); err != nil {
		return errors.Wrap(err, "failed to close about:blank tab")
	}

	return runTest(ctx, pd, pv, p, chromeTypeLacros, ltconn)
}

func runCrosTest(ctx context.Context, pd launcher.PreData, pv *perf.Values, p gpuCUJTestParams) error {
	ctconn, err := pd.Chrome.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to connect to test API")
	}

	// Wait for quiescent state.
	if err := cpu.WaitUntilIdle(ctx); err != nil {
		return errors.Wrap(err, "failed waiting for CPU to become idle")
	}

	conn, err := pd.Chrome.NewConn(ctx, p.url)
	if err != nil {
		return errors.Wrap(err, "failed to open new tab")
	}
	defer conn.Close()

	return runTest(ctx, pd, pv, p, chromeTypeCros, ctconn)
}

func GpuCUJ(ctx context.Context, s *testing.State) {
	tconn, err := s.PreValue().(launcher.PreData).Chrome.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	if err := toggleTraySetting(ctx, tconn, "Toggle Do not disturb. Do not disturb is off."); err != nil {
		s.Fatal("Failed to disable notifications: ", err)
	}
	defer func() {
		if err := toggleTraySetting(ctx, tconn, "Toggle Do not disturb. Do not disturb is on."); err != nil {
			s.Fatal("Failed to re-enable notifications: ", err)
		}
	}()

	p := s.Param().(gpuCUJTestParams)
	pv := perf.NewValues()
	if err := runLacrosTest(ctx, s.PreValue().(launcher.PreData), pv, p); err != nil {
		s.Fatal("Failed to run lacros test: ", err)
	}

	if err := runCrosTest(ctx, s.PreValue().(launcher.PreData), pv, p); err != nil {
		s.Fatal("Failed to run cros test: ", err)
	}

	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Cannot save perf data: ", err)
	}
}
