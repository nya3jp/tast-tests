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
	"strings"
	"time"

	"chromiumos/tast/common/perf"
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
	"chromiumos/tast/local/power"
	"chromiumos/tast/testing"
)

type testType string
type chromeType string

const (
	// Simple test of performance with a maximized window opening various web content.
	// This is useful for tracking the performance w.r.t hardware overlay forwarding of video or WebGL content.
	testTypeMaximized testType = "maximized"
	// Test of performance while showing the three-dot context menu. This is intended to track the
	// performance impact of potential double composition of the context menu and hardware overlay usage.
	testTypeThreeDot testType = "threedot"
	// Test of performance during a drag-resize operation.
	testTypeResize testType = "resize"
	// Test of performance of gradual occlusion via drag-move of web content. This is useful for tracking impact
	// of hardware overlay forwarding and clipping (due to occlusion) of tiles optimisations.
	testTypeMoveOcclusion testType = "moveocclusion"
	// Test similar to testTypeMoveOcclusion but always occludes using a ChromeOS chrome window.
	testTypeMoveOcclusionWithCrosWindow testType = "moveocclusion_withcroswindow"

	// testDuration indicates how long histograms should be sampled for during performance tests.
	testDuration time.Duration = 20 * time.Second
	// dragMoveOffsetDP indicates the offset from the top-left of a Chrome window to drag to ensure we can drag move it.
	dragMoveOffsetDP int = 5
	// insetSlopDP indicates how much to inset the work area (display area) to avoid window snapping to the
	// edges of the screen intefering with drag-move and drag-resize of windows.
	insetSlopDP int = 40

	// chromeTypeChromeOS indicates we are using the ChromeOS system's Chrome browser
	chromeTypeChromeOS = "chromeos"
	// chromeTypeLacros indicates we are using Linux Chrome
	chromeTypeLacros = "lacros"
)

type page struct {
	name string
	url  string // TODO: Replace this with WPR.
}

var pageSet = []page{
	{
		name: "aquarium", // WebGL Aquarium
		url:  "https://webglsamples.org/aquarium/aquarium.html",
	},
	{
		name: "poster", // Poster Circle
		url:  "https://webkit.org/blog-files/3d-transforms/poster-circle.html",
	},
	{
		name: "maps", // Google Maps
		url:  "https://www.google.com/maps/@35.652772,139.6605155,14z",
	},
}

type gpuCUJTestParams struct {
	testType testType
	rot90    bool // Whether to rotate the screen 90 or not.
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
			Name: "maximized",
			Val: gpuCUJTestParams{
				testType: testTypeMaximized,
				rot90:    false,
			},
			Pre: launcher.StartedByData(),
		}, {
			Name: "maximized_rot90",
			Val: gpuCUJTestParams{
				testType: testTypeMaximized,
				rot90:    true,
			},
			Pre: launcher.StartedByData(),
		}, {
			Name: "maximized_composited",
			Val: gpuCUJTestParams{
				testType: testTypeMaximized,
				rot90:    false,
			},
			Pre: launcher.StartedByDataForceComposition(),
		}, {
			Name: "threedot",
			Val: gpuCUJTestParams{
				testType: testTypeThreeDot,
				rot90:    false,
			},
			Pre: launcher.StartedByData(),
		}, {
			Name: "threedot_rot90",
			Val: gpuCUJTestParams{
				testType: testTypeThreeDot,
				rot90:    true,
			},
			Pre: launcher.StartedByData(),
		}, {
			Name: "threedot_composited",
			Val: gpuCUJTestParams{
				testType: testTypeThreeDot,
				rot90:    false,
			},
			Pre: launcher.StartedByDataForceComposition(),
		}, {
			Name: "resize",
			Val: gpuCUJTestParams{
				testType: testTypeResize,
				rot90:    false,
			},
			Pre: launcher.StartedByData(),
		}, {
			Name: "resize_rot90",
			Val: gpuCUJTestParams{
				testType: testTypeResize,
				rot90:    true,
			},
			Pre: launcher.StartedByData(),
		}, {
			Name: "resize_composited",
			Val: gpuCUJTestParams{
				testType: testTypeResize,
				rot90:    false,
			},
			Pre: launcher.StartedByDataForceComposition(),
		}, {
			Name: "moveocclusion",
			Val: gpuCUJTestParams{
				testType: testTypeMoveOcclusion,
				rot90:    false,
			},
			Pre: launcher.StartedByData(),
		}, {
			Name: "moveocclusion_rot90",
			Val: gpuCUJTestParams{
				testType: testTypeMoveOcclusion,
				rot90:    true,
			},
			Pre: launcher.StartedByData(),
		}, {
			Name: "moveocclusion_composited",
			Val: gpuCUJTestParams{
				testType: testTypeMoveOcclusion,
				rot90:    false,
			},
			Pre: launcher.StartedByDataForceComposition(),
		}, {
			Name: "moveocclusion_withcroswindow",
			Val: gpuCUJTestParams{
				testType: testTypeMoveOcclusionWithCrosWindow,
				rot90:    false,
			},
			Pre: launcher.StartedByData(),
		}, {
			Name: "moveocclusion_withcroswindow_rot90",
			Val: gpuCUJTestParams{
				testType: testTypeMoveOcclusionWithCrosWindow,
				rot90:    true,
			},
			Pre: launcher.StartedByData(),
		}, {
			Name: "moveocclusion_withcroswindow_composited",
			Val: gpuCUJTestParams{
				testType: testTypeMoveOcclusionWithCrosWindow,
				rot90:    false,
			},
			Pre: launcher.StartedByDataForceComposition(),
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
		// Wait for the window given by w to be in the given state and also not be animating.
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

func waitForWindowWithPredicate(ctx context.Context, ctconn *chrome.TestConn, p func(*ash.Window) bool) (*ash.Window, error) {
	if err := ash.WaitForCondition(ctx, ctconn, p, pollOptions); err != nil {
		return nil, err
	}
	return ash.FindWindow(ctx, ctconn, p)
}

func findFirstBlankWindow(ctx context.Context, ctconn *chrome.TestConn) (*ash.Window, error) {
	return waitForWindowWithPredicate(ctx, ctconn, func(w *ash.Window) bool {
		return strings.Contains(w.Title, "about:blank")
	})
}

func findFirstNonBlankWindow(ctx context.Context, ctconn *chrome.TestConn) (*ash.Window, error) {
	return waitForWindowWithPredicate(ctx, ctconn, func(w *ash.Window) bool {
		return !strings.Contains(w.Title, "about:blank")
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

var metricMap = map[string]struct {
	unit      string
	direction perf.Direction
	uma       bool
}{
	"Graphics.Smoothness.PercentDroppedFrames.CompositorThread.Universal": {
		unit:      "percent",
		direction: perf.SmallerIsBetter,
		uma:       true,
	},
	"Graphics.Smoothness.PercentDroppedFrames.MainThread.Universal": {
		unit:      "percent",
		direction: perf.SmallerIsBetter,
		uma:       true,
	},
	"Graphics.Smoothness.PercentDroppedFrames.SlowerThread.Universal": {
		unit:      "percent",
		direction: perf.SmallerIsBetter,
		uma:       true,
	},
	"Graphics.Smoothness.PercentDroppedFrames.AllSequences": {
		unit:      "percent",
		direction: perf.SmallerIsBetter,
		uma:       true,
	},
	"Compositing.Display.DrawToSwapUs": {
		unit:      "ms",
		direction: perf.SmallerIsBetter,
		uma:       true,
	},
	"total_power": {
		unit:      "joules",
		direction: perf.SmallerIsBetter,
		uma:       false,
	},
	"gpu_power": {
		unit:      "joules",
		direction: perf.SmallerIsBetter,
		uma:       false,
	},
	"nongpu_power": {
		unit:      "joules",
		direction: perf.SmallerIsBetter,
		uma:       false,
	},
}

type statType string

const (
	meanStat  = "mean"
	valueStat = "value"
)

type statBucketKey struct {
	metric string
	stat   statType
	crt    chromeType
}

type metricsRecorder struct {
	buckets map[statBucketKey][]float64
}

func (m *metricsRecorder) record(ctx context.Context, invoc *testInvocation, key statBucketKey, value float64) error {
	minfo, ok := metricMap[key.metric]
	if !ok {
		return errors.Errorf("failed to lookup metric info: %s", key.metric)
	}

	name := fmt.Sprintf("%s.%s.%s.%s", invoc.page.name, key.metric, string(key.stat), string(key.crt))
	testing.ContextLog(ctx, name, ": ", value, " ", minfo.unit)

	invoc.pv.Set(perf.Metric{
		Name:      name,
		Unit:      minfo.unit,
		Direction: minfo.direction,
	}, value)
	m.buckets[key] = append(m.buckets[key], value)
	return nil
}

func (m *metricsRecorder) recordHistogram(ctx context.Context, invoc *testInvocation, h *metrics.Histogram) error {
	// Ignore empty histograms. It's hard to define what the mean should be in this case.
	if h.TotalCount() == 0 {
		return nil
	}

	mean, err := h.Mean()
	if err != nil {
		return errors.Wrapf(err, "failed to get mean for histogram: %s", h.Name)
	}

	testing.ContextLog(ctx, h)

	return m.record(ctx, invoc, statBucketKey{
		metric: h.Name,
		stat:   meanStat,
		crt:    invoc.crt,
	}, mean)
}

func (m *metricsRecorder) recordValue(ctx context.Context, invoc *testInvocation, name string, value float64) error {
	return m.record(ctx, invoc, statBucketKey{
		metric: name,
		stat:   valueStat,
		crt:    invoc.crt,
	}, value)
}

func (m *metricsRecorder) computeStatistics(ctx context.Context, pv *perf.Values) error {
	// Collect means and standard deviations for each bucket. Each bucket contains results from several different pages.
	// We define the population as the set of all pages (another option would be to define the population as the
	// metric itself). For histograms (meanStat), we take a single sample which contains the means for each page.
	// For single values (valueStat), we take as single sample which just consists of those values.
	// We estimate the following quantities:
	// page_mean:
	//   Meaning: The mean for all pages. (e.g. mean of histogram means)
	//   Estimator: sample mean
	// page_stddev:
	//   Meaning: Variance over all pages. (e.g. variance of histogram means)
	//   Estimator: unbiased sample variance
	// N.B. we report standard deviation not variance so even though we use Bessel's correction the standard deviation
	// is still biased.
	// TODO: Consider extending this to also provide data where the population is the metric itself.
	//   e.g. metric_stddev, metric_mean - statistics on the metric overall not per-page.
	var logs []string
	for k, bucket := range m.buckets {
		minfo, ok := metricMap[k.metric]
		if !ok {
			return errors.Errorf("failed to lookup metric info: %s", k.metric)
		}

		var sum float64
		for _, value := range bucket {
			sum += value
		}
		n := float64(len(bucket))
		mean := sum / n
		var variance float64
		for _, value := range bucket {
			variance += (value - mean) * (value - mean)
		}
		variance /= float64(len(bucket) - 1) // Bessel's correction.
		stddev := math.Sqrt(variance)

		m := perf.Metric{
			Name:      fmt.Sprintf("all.%s.%s.%s", k.metric, "page_mean", string(k.crt)),
			Unit:      minfo.unit,
			Direction: minfo.direction,
		}
		s := perf.Metric{
			Name:      fmt.Sprintf("all.%s.%s.%s", k.metric, "page_stddev", string(k.crt)),
			Unit:      minfo.unit,
			Direction: perf.SmallerIsBetter, // In general, it's better if standard deviation is less.
		}
		logs = append(logs, fmt.Sprint(m.Name, ": ", mean, " ", m.Unit), fmt.Sprint(s.Name, ": ", stddev, " ", s.Unit))
		pv.Set(m, mean)
		pv.Set(s, stddev)
	}

	// Print logs in order.
	sort.Strings(logs)
	for _, log := range logs {
		testing.ContextLog(ctx, log)
	}
	return nil
}

func runHistogram(ctx context.Context, tconn *chrome.TestConn, invoc *testInvocation, perfFn func() error) error {
	var keys []string
	for k, v := range metricMap {
		if v.uma {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)

	rapl, err := power.NewRAPLSnapshot()
	if err != nil {
		return errors.Wrap(err, "failed to get RAPL snapshot")
	}

	histograms, err := metrics.Run(ctx, tconn, perfFn, keys...)
	if err != nil {
		return errors.Wrap(err, "failed to get histograms")
	}

	raplv, err := rapl.DiffWithCurrentRAPL()
	if err != nil {
		return errors.Wrap(err, "failed to compute RAPL diffs")
	}

	// Store metrics in the form: Scenario.PageSet.UMA metric name.statistic.{chromeos, lacros}.
	// For example, maximized.Compositing.Display.DrawToSwapUs.mean.chromeos. In crosbolt, for each
	// scenario (e.g. three-dot menu), we can then easily compare between chromeos and lacros
	// for the same metric, in the same scenario.
	for _, h := range histograms {
		if err := invoc.metrics.recordHistogram(ctx, invoc, h); err != nil {
			return err
		}
	}

	nongpuPower := raplv.Total() - raplv.Uncore()
	if err := invoc.metrics.recordValue(ctx, invoc, "total_power", raplv.Total()); err != nil {
		return err
	}
	if err := invoc.metrics.recordValue(ctx, invoc, "nongpu_power", nongpuPower); err != nil {
		return err
	}
	if err := invoc.metrics.recordValue(ctx, invoc, "gpu_power", raplv.Uncore()); err != nil {
		return err
	}
	return nil
}

// testInvocation describes a particular test run. A test run involves running a particular scenario
// (e.g. moveocclusion) with a particular type of Chrome (ChromeOS or Lacros) on a particular page.
// This structure holds the necessary data to do this.
type testInvocation struct {
	pv       *perf.Values
	scenario testType
	page     page
	crt      chromeType
	metrics  *metricsRecorder
}

// runTest runs the common part of the GpuCUJ performance test - that is, shared between ChromeOS chrome and Linux chrome.
// tconn is a test connection to the current browser being used (either ChromeOS or Linux chrome).
func runTest(ctx context.Context, tconn *chrome.TestConn, pd launcher.PreData, invoc *testInvocation) error {
	ctconn, err := pd.Chrome.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to connect to test API")
	}

	w, err := findFirstNonBlankWindow(ctx, ctconn)
	if err != nil {
		return err
	}

	info, err := display.GetInternalInfo(ctx, ctconn)
	if err != nil {
		return err
	}

	perfFn := func() error {
		return testing.Sleep(ctx, testDuration)
	}
	if invoc.scenario == testTypeResize {
		// Restore window.
		if err := setWindowState(ctx, ctconn, w.ID, ash.WindowStateNormal); err != nil {
			return errors.Wrap(err, "failed to restore non-blank window")
		}

		// Create a landscape rectangle. Avoid snapping by insetting by insetSlopDP.
		ms := math.Min(float64(info.WorkArea.Width), float64(info.WorkArea.Height))
		sb := coords.NewRect(info.WorkArea.Left, info.WorkArea.Top, int(ms), int(ms*0.6)).WithInset(insetSlopDP, insetSlopDP)
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
	} else if invoc.scenario == testTypeMoveOcclusion || invoc.scenario == testTypeMoveOcclusionWithCrosWindow {
		wb, err := findFirstBlankWindow(ctx, ctconn)
		if err != nil {
			return err
		}

		// Restore windows.
		if err := setWindowState(ctx, ctconn, w.ID, ash.WindowStateNormal); err != nil {
			return errors.Wrap(err, "failed to restore non-blank window")
		}

		if err := setWindowState(ctx, ctconn, wb.ID, ash.WindowStateNormal); err != nil {
			return errors.Wrap(err, "failed to restore blank window")
		}

		// Set content window to take up the left half of the screen.
		sbl := coords.NewRect(info.WorkArea.Left, info.WorkArea.Top, info.WorkArea.Width/2, info.WorkArea.Height).WithInset(insetSlopDP, insetSlopDP)
		if err := setWindowBounds(ctx, ctconn, w.ID, sbl); err != nil {
			return errors.Wrap(err, "failed to set non-blank window initial bounds")
		}

		// Set the occluding window to take up the right side of the screen.
		sbr := sbl.WithOffset(sbl.Width, 0)
		if err := setWindowBounds(ctx, ctconn, wb.ID, sbr); err != nil {
			return errors.Wrap(err, "failed to set blank window initial bounds")
		}
		perfFn = func() error {
			// Drag from not occluding to completely occluding.
			start := coords.NewPoint(sbr.Left+dragMoveOffsetDP, sbr.Top+dragMoveOffsetDP)
			end := coords.NewPoint(sbl.Left, sbr.Top+1)
			if err := ash.MouseDrag(ctx, ctconn, start, end, testDuration); err != nil {
				return errors.Wrap(err, "failed to drag move")
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
	if invoc.scenario == testTypeThreeDot {
		clickFn := func(n *ui.Node) error { return n.LeftClick(ctx) }
		if invoc.crt == chromeTypeLacros {
			clickFn = func(n *ui.Node) error { return leftClickLacros(ctx, ctconn, w.ID, n) }
		}
		if err := toggleThreeDotMenu(ctx, tconn, clickFn); err != nil {
			return errors.Wrap(err, "failed to open three dot menu")
		}
		defer toggleThreeDotMenu(ctx, tconn, clickFn)
	}

	return runHistogram(ctx, tconn, invoc, perfFn)
}

func runLacrosTest(ctx context.Context, pd launcher.PreData, invoc *testInvocation) error {
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

	connURL, err := l.NewConn(ctx, invoc.page.url)
	if err != nil {
		return errors.Wrap(err, "failed to open new tab")
	}
	defer connURL.Close()
	defer connURL.CloseTarget(ctx)

	// Close the initial "about:blank" tab present at startup.
	if err := closeAboutBlank(ctx, l.Devsess); err != nil {
		return errors.Wrap(err, "failed to close about:blank tab")
	}

	// Setup extra window for multi-window tests.
	if invoc.scenario == testTypeMoveOcclusion {
		connBlank, err := l.NewConn(ctx, chrome.BlankURL, cdputil.WithNewWindow())
		if err != nil {
			return errors.Wrap(err, "failed to open new tab")
		}
		defer connBlank.Close()
		defer connBlank.CloseTarget(ctx)

	} else if invoc.scenario == testTypeMoveOcclusionWithCrosWindow {
		connBlank, err := pd.Chrome.NewConn(ctx, chrome.BlankURL, cdputil.WithNewWindow())
		if err != nil {
			return errors.Wrap(err, "failed to open new tab")
		}
		defer connBlank.Close()
		defer connBlank.CloseTarget(ctx)
	}

	return runTest(ctx, ltconn, pd, invoc)
}

func runCrosTest(ctx context.Context, pd launcher.PreData, invoc *testInvocation) error {
	ctconn, err := pd.Chrome.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to connect to test API")
	}

	// Wait for quiescent state.
	if err := cpu.WaitUntilIdle(ctx); err != nil {
		return errors.Wrap(err, "failed waiting for CPU to become idle")
	}

	connURL, err := pd.Chrome.NewConn(ctx, invoc.page.url)
	if err != nil {
		return errors.Wrap(err, "failed to open new tab")
	}
	defer connURL.Close()
	defer connURL.CloseTarget(ctx)

	// Setup extra window for multi-window tests.
	if invoc.scenario == testTypeMoveOcclusion || invoc.scenario == testTypeMoveOcclusionWithCrosWindow {
		connBlank, err := pd.Chrome.NewConn(ctx, chrome.BlankURL, cdputil.WithNewWindow())
		if err != nil {
			return errors.Wrap(err, "failed to open new tab")
		}
		defer connBlank.Close()
		defer connBlank.CloseTarget(ctx)
	}

	return runTest(ctx, ctconn, pd, invoc)
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

	params := s.Param().(gpuCUJTestParams)

	if params.rot90 {
		infos, err := display.GetInfo(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to get display info: ", err)
		}

		if len(infos) != 1 {
			s.Fatal("Failed to find unique display")
		}

		rot := 90
		if err := display.SetDisplayProperties(ctx, tconn, infos[0].ID, display.DisplayProperties{Rotation: &rot}); err != nil {
			s.Fatal("Failed to rotate display: ", err)
		}
		// Restore the initial rotation.
		defer func() {
			if err := display.SetDisplayProperties(ctx, tconn, infos[0].ID, display.DisplayProperties{Rotation: &infos[0].Rotation}); err != nil {
				s.Fatal("Failed to restore the initial display rotation: ", err)
			}
		}()
	}

	pv := perf.NewValues()
	m := metricsRecorder{buckets: make(map[statBucketKey][]float64)}
	for _, page := range pageSet {
		if err := runLacrosTest(ctx, s.PreValue().(launcher.PreData), &testInvocation{
			pv:       pv,
			scenario: params.testType,
			page:     page,
			crt:      chromeTypeLacros,
			metrics:  &m,
		}); err != nil {
			s.Fatal("Failed to run lacros test: ", err)
		}

		if err := runCrosTest(ctx, s.PreValue().(launcher.PreData), &testInvocation{
			pv:       pv,
			scenario: params.testType,
			page:     page,
			crt:      chromeTypeChromeOS,
			metrics:  &m,
		}); err != nil {
			s.Fatal("Failed to run cros test: ", err)
		}
	}

	if err := m.computeStatistics(ctx, pv); err != nil {
		s.Fatal("Could not compute derived statistics: ", err)
	}

	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Cannot save perf data: ", err)
	}
}
