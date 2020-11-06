// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package gpucuj tests GPU CUJ tests on lacros Chrome and Chrome OS Chrome.
package gpucuj

import (
	"context"
	"math"
	"path/filepath"
	"time"

	"android.googlesource.com/platform/external/perfetto/protos/perfetto/trace"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/cdputil"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/mouse"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/lacros"
	"chromiumos/tast/local/lacros/launcher"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/local/power"
	"chromiumos/tast/testing"
)

// TestType describes the type of GpuCUJ test to run.
type TestType string

// TestParams holds parameters describing how to run a GpuCUJ test.
type TestParams struct {
	TestType TestType
	Rot90    bool // Whether to rotate the screen 90 or not.
}

const (
	// TestTypeMaximized is a simple test of performance with a maximized window opening various web content.
	// This is useful for tracking the performance w.r.t hardware overlay forwarding of video or WebGL content.
	TestTypeMaximized TestType = "maximized"
	// TestTypeThreeDot is a test of performance while showing the three-dot context menu. This is intended to track the
	// performance impact of potential double composition of the context menu and hardware overlay usage.
	TestTypeThreeDot TestType = "threedot"
	// TestTypeResize is a test of performance during a drag-resize operation.
	TestTypeResize TestType = "resize"
	// TestTypeMoveOcclusion is a test of performance of gradual occlusion via drag-move of web content. This is useful for tracking impact
	// of hardware overlay forwarding and clipping (due to occlusion) of tiles optimisations.
	TestTypeMoveOcclusion TestType = "moveocclusion"
	// TestTypeMoveOcclusionWithCrosWindow is a test similar to TestTypeMoveOcclusion but always occludes using a ChromeOS chrome window.
	TestTypeMoveOcclusionWithCrosWindow TestType = "moveocclusion_withcroswindow"

	// testDuration indicates how long histograms should be sampled for during performance tests.
	testDuration time.Duration = 20 * time.Second
	// dragMoveOffsetDP indicates the offset from the top-left of a Chrome window to drag to ensure we can drag move it.
	dragMoveOffsetDP int = 5
	// insetSlopDP indicates how much to inset the work area (display area) to avoid window snapping to the
	// edges of the screen interfering with drag-move and drag-resize of windows.
	insetSlopDP int = 40

	// youtubeVideoDx and youtubeVideoDy are found manually by checking where the Youtube video appears relative
	// to the top left corner of the browser window. These coordinates let us click on the Youtube video area
	// if the video does not appear. This makes sure the video plays.
	// See crbug.com/1085355.
	youtubeVideoDx = 200.0
	youtubeVideoDy = 250.0
)

func ensureYoutubeVideo(ctx context.Context, ctconn *chrome.TestConn, conn *chrome.Conn) error {
	if err := webutil.WaitForYoutubeVideo(ctx, conn, 10*time.Second); err != nil {
		// TODO(crbug.com/1085355): Sometimes, lacros-chrome does not autoplay the Youtube video. Programmatic methods
		// for forcing the video to load and play don't seem to work, so manually click on the video via Ash.
		w, err := lacros.FindFirstNonBlankWindow(ctx, ctconn)
		if err != nil {
			return err
		}
		loc := w.BoundsInRoot.TopLeft().Add(coords.NewPoint(youtubeVideoDx, youtubeVideoDy))
		if err := mouse.Click(ctx, ctconn, loc, mouse.LeftButton); err != nil {
			return err
		}

		return webutil.WaitForYoutubeVideo(ctx, conn, 100*time.Second)
	}

	return nil
}

type page struct {
	name     string
	url      string
	finalize func(ctx context.Context, ctconn *chrome.TestConn, conn *chrome.Conn) error
}

var pageSet = []page{
	{
		name: "aquarium", // WebGL Aquarium. This page is for testing WebGL.
		url:  "https://webglsamples.org/aquarium/aquarium.html",
	},
	{
		name: "poster", // Poster Circle. This page is for testing compositor performance.
		url:  "https://webkit.org/blog-files/3d-transforms/poster-circle.html",
	},
	{
		name: "maps", // Google Maps. This page is for testing WebGL.
		url:  "https://www.google.com/maps/@35.652772,139.6605155,14z",
	},
	{
		name:     "youtube", // YouTube. This page is for testing video playback.
		url:      "https://www.youtube.com/watch?v=aqz-KE-bpKQ?autoplay=1",
		finalize: ensureYoutubeVideo,
	},
	{
		name: "wikipedia", // Wikipedia. This page is for testing conventional web-pages.
		url:  "https://en.wikipedia.org/wiki/Cat",
	},
}

// This test deals with both ChromeOS chrome and lacros chrome. In order to reduce confusion,
// we adopt the following naming convention for chrome.TestConn objects:
//   ctconn: chrome.TestConn to ChromeOS chrome.
//   ltconn: chrome.TestConn to lacros chrome.
//   tconn: chrome.TestConn to either ChromeOS or lacros chrome, i.e. both are usable.

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
	// adding the top left coordinate of the lacros-chrome window in cros-chrome root window coorindates.
	return mouse.Click(ctx, ctconn, w.BoundsInRoot.TopLeft().Add(n.Location.CenterPoint()), mouse.LeftButton)
}

func toggleThreeDotMenu(ctx context.Context, tconn *chrome.TestConn, clickFn func(*ui.Node) error) error {
	// Find and click the three dot menu via UI.
	params := ui.FindParams{
		Role:      ui.RoleTypePopUpButton,
		ClassName: "BrowserAppMenuButton",
	}
	menu, err := ui.FindWithTimeout(ctx, tconn, params, 10*time.Second)
	if err != nil {
		return errors.Wrap(err, "failed to find the three dot menu")
	}
	defer menu.Release(ctx)

	if err := clickFn(menu); err != nil {
		return errors.Wrap(err, "failed to click three dot menu")
	}
	return nil
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

// testInvocation describes a particular test run. A test run involves running a particular scenario
// (e.g. moveocclusion) with a particular type of Chrome (ChromeOS or Lacros) on a particular page.
// This structure holds the necessary data to do this.
type testInvocation struct {
	pv       *perf.Values
	scenario TestType
	page     page
	crt      lacros.ChromeType
	metrics  *metricsRecorder
	traceDir string
}

type traceable interface {
	StartTracing(ctx context.Context, categories []string) error
	StopTracing(ctx context.Context) (*trace.Trace, error)
}

// runTest runs the common part of the GpuCUJ performance test - that is, shared between ChromeOS chrome and lacros chrome.
// tconn is a test connection to the current browser being used (either ChromeOS or lacros chrome).
func runTest(ctx context.Context, tconn *chrome.TestConn, pd launcher.PreData, tracer traceable, invoc *testInvocation) error {
	w, err := lacros.FindFirstNonBlankWindow(ctx, pd.TestAPIConn)
	if err != nil {
		return err
	}

	info, err := display.GetInternalInfo(ctx, pd.TestAPIConn)
	if err != nil {
		return err
	}

	perfFn := func(ctx context.Context) error {
		return testing.Sleep(ctx, testDuration)
	}
	if invoc.scenario == TestTypeResize {
		// Restore window.
		if err := ash.SetWindowStateAndWait(ctx, pd.TestAPIConn, w.ID, ash.WindowStateNormal); err != nil {
			return errors.Wrap(err, "failed to restore non-blank window")
		}

		// Create a landscape rectangle. Avoid snapping by insetting by insetSlopDP.
		ms := math.Min(float64(info.WorkArea.Width), float64(info.WorkArea.Height))
		sb := coords.NewRect(info.WorkArea.Left, info.WorkArea.Top, int(ms), int(ms*0.6)).WithInset(insetSlopDP, insetSlopDP)
		if err := setWindowBounds(ctx, pd.TestAPIConn, w.ID, sb); err != nil {
			return errors.Wrap(err, "failed to set window initial bounds")
		}

		perfFn = func(ctx context.Context) error {
			// End bounds are just flipping the rectangle.
			// TODO(crbug.com/1067535): Subtract -1 to ensure drag-resize occurs for now.
			start := coords.NewPoint(sb.Left+sb.Width-1, sb.Top+sb.Height-1)
			end := coords.NewPoint(sb.Left+sb.Height, sb.Top+sb.Width)
			if err := mouse.Drag(ctx, pd.TestAPIConn, start, end, testDuration); err != nil {
				return errors.Wrap(err, "failed to drag resize")
			}
			return nil
		}
	} else if invoc.scenario == TestTypeMoveOcclusion || invoc.scenario == TestTypeMoveOcclusionWithCrosWindow {
		wb, err := lacros.FindFirstBlankWindow(ctx, pd.TestAPIConn)
		if err != nil {
			return err
		}

		// Restore windows.
		if err := ash.SetWindowStateAndWait(ctx, pd.TestAPIConn, w.ID, ash.WindowStateNormal); err != nil {
			return errors.Wrap(err, "failed to restore non-blank window")
		}

		if err := ash.SetWindowStateAndWait(ctx, pd.TestAPIConn, wb.ID, ash.WindowStateNormal); err != nil {
			return errors.Wrap(err, "failed to restore blank window")
		}

		// Set content window to take up the left half of the screen in landscape, or top half in portrait.
		isp := info.WorkArea.Width < info.WorkArea.Height
		sbl := coords.NewRect(info.WorkArea.Left, info.WorkArea.Top, info.WorkArea.Width/2, info.WorkArea.Height)
		if isp {
			sbl = coords.NewRect(info.WorkArea.Left, info.WorkArea.Top, info.WorkArea.Width, info.WorkArea.Height/2)
		}
		sbl = sbl.WithInset(insetSlopDP, insetSlopDP)
		if err := setWindowBounds(ctx, pd.TestAPIConn, w.ID, sbl); err != nil {
			return errors.Wrap(err, "failed to set non-blank window initial bounds")
		}

		// Set the occluding window to take up the right side of the screen in landscape, or bottom half in portrait.
		sbr := sbl.WithOffset(sbl.Width, 0)
		if isp {
			sbr = sbl.WithOffset(0, sbl.Height)
		}
		if err := setWindowBounds(ctx, pd.TestAPIConn, wb.ID, sbr); err != nil {
			return errors.Wrap(err, "failed to set blank window initial bounds")
		}
		perfFn = func(ctx context.Context) error {
			// Drag from not occluding to completely occluding.
			start := coords.NewPoint(sbr.Left+dragMoveOffsetDP, sbr.Top+dragMoveOffsetDP)
			end := coords.NewPoint(sbl.Left+dragMoveOffsetDP, sbl.Top+dragMoveOffsetDP)
			if err := mouse.Drag(ctx, pd.TestAPIConn, start, end, testDuration); err != nil {
				return errors.Wrap(err, "failed to drag move")
			}
			return nil
		}
	} else {
		// Maximize window.
		if err := ash.SetWindowStateAndWait(ctx, pd.TestAPIConn, w.ID, ash.WindowStateMaximized); err != nil {
			return errors.Wrap(err, "failed to maximize window")
		}
	}

	// Open the threedot menu if indicated.
	// TODO(edcourtney): Sometimes the accessibility tree isn't populated for lacros chrome, which causes this code to fail.
	if invoc.scenario == TestTypeThreeDot {
		clickFn := func(n *ui.Node) error { return n.LeftClick(ctx) }
		if invoc.crt == lacros.ChromeTypeLacros {
			clickFn = func(n *ui.Node) error { return leftClickLacros(ctx, pd.TestAPIConn, w.ID, n) }
		}
		if err := toggleThreeDotMenu(ctx, tconn, clickFn); err != nil {
			return errors.Wrap(err, "failed to open three dot menu")
		}
		defer toggleThreeDotMenu(ctx, tconn, clickFn)
	}

	if invoc.traceDir != "" {
		oldPerfFn := perfFn
		perfFn = func(ctx context.Context) error {
			if err := tracer.StartTracing(ctx, tracingCategories); err != nil {
				return err
			}
			if err := oldPerfFn(ctx); err != nil {
				if _, err := tracer.StopTracing(ctx); err != nil {
					testing.ContextLog(ctx, "Failed to stop tracing after encountering other error: ", err)
				}
				return err
			}
			tr, err := tracer.StopTracing(ctx)
			if err != nil {
				return err
			}
			filename := filepath.Join(invoc.traceDir, string(invoc.crt)+"-"+invoc.page.name+"-trace.data")
			if err := chrome.SaveTraceToFile(ctx, tr, filename); err != nil {
				return err
			}
			return nil
		}
	}

	return runHistogram(ctx, tconn, invoc, perfFn)
}

func waitForStableEnvironment(ctx context.Context) error {
	// Wait for CPU to cool down.
	if _, err := power.WaitUntilCPUCoolDown(ctx, power.CoolDownPreserveUI); err != nil {
		return errors.Wrap(err, "failed to wait for CPU to cool down")
	}

	// Wait for quiescent state.
	if err := cpu.WaitUntilIdle(ctx); err != nil {
		return errors.Wrap(err, "failed waiting for CPU to become idle")
	}
	return nil
}

func runLacrosTest(ctx context.Context, pd launcher.PreData, invoc *testInvocation) error {
	conn, ltconn, l, cleanup, err := lacros.SetupLacrosTestWithPage(ctx, pd, invoc.page.url)
	if err != nil {
		return errors.Wrap(err, "failed to setup cros-chrome test page")
	}
	defer cleanup(ctx)

	if invoc.page.finalize != nil {
		if err := invoc.page.finalize(ctx, pd.TestAPIConn, conn); err != nil {
			return err
		}
	}

	// Setup extra window for multi-window tests.
	if invoc.scenario == TestTypeMoveOcclusion {
		connBlank, err := l.NewConn(ctx, chrome.BlankURL, cdputil.WithNewWindow())
		if err != nil {
			return errors.Wrap(err, "failed to open new tab")
		}
		defer connBlank.Close()
		defer connBlank.CloseTarget(ctx)

	} else if invoc.scenario == TestTypeMoveOcclusionWithCrosWindow {
		connBlank, err := pd.Chrome.NewConn(ctx, chrome.BlankURL, cdputil.WithNewWindow())
		if err != nil {
			return errors.Wrap(err, "failed to open new tab")
		}
		defer connBlank.Close()
		defer connBlank.CloseTarget(ctx)
	}

	return runTest(ctx, ltconn, pd, l, invoc)
}

func runCrosTest(ctx context.Context, pd launcher.PreData, invoc *testInvocation) error {
	conn, cleanup, err := lacros.SetupCrosTestWithPage(ctx, pd, invoc.page.url)
	if err != nil {
		return errors.Wrap(err, "failed to setup cros-chrome test page")
	}
	defer cleanup(ctx)

	if invoc.page.finalize != nil {
		if err := invoc.page.finalize(ctx, pd.TestAPIConn, conn); err != nil {
			return err
		}
	}

	// Setup extra window for multi-window tests.
	if invoc.scenario == TestTypeMoveOcclusion || invoc.scenario == TestTypeMoveOcclusionWithCrosWindow {
		connBlank, err := pd.Chrome.NewConn(ctx, chrome.BlankURL, cdputil.WithNewWindow())
		if err != nil {
			return errors.Wrap(err, "failed to open new tab")
		}
		defer connBlank.Close()
		defer connBlank.CloseTarget(ctx)
	}

	return runTest(ctx, pd.TestAPIConn, pd, pd.Chrome, invoc)
}

// RunGpuCUJ runs a GpuCUJ test according to the given parameters.
func RunGpuCUJ(ctx context.Context, pd launcher.PreData, params TestParams) (*perf.Values, lacros.CleanupCallback, error) {
	cleanup, err := lacros.SetupPerfTest(ctx, pd.TestAPIConn, "lacros.GpuCUJ")
	if err != nil {
		return nil, cleanup, errors.Wrap(err, "failed to setup GpuCUJ test")
	}

	if params.Rot90 {
		infos, err := display.GetInfo(ctx, pd.TestAPIConn)
		if err != nil {
			return nil, cleanup, errors.Wrap(err, "failed to get display info")
		}

		if len(infos) != 1 {
			return nil, cleanup, errors.New("failed to find unique display")
		}

		rot := 90
		if err := display.SetDisplayProperties(ctx, pd.TestAPIConn, infos[0].ID, display.DisplayProperties{Rotation: &rot}); err != nil {
			return nil, cleanup, errors.Wrap(err, "failed to rotate display")
		}
		// Restore the initial rotation.
		cleanup = lacros.CombineCleanup(ctx, cleanup, func(ctx context.Context) error {
			return display.SetDisplayProperties(ctx, pd.TestAPIConn, infos[0].ID, display.DisplayProperties{Rotation: &infos[0].Rotation})
		}, "failed to restore the initial display rotation")
	}

	pv := perf.NewValues()
	m := metricsRecorder{buckets: make(map[statBucketKey][]float64)}
	for _, page := range pageSet {
		if err := runLacrosTest(ctx, pd, &testInvocation{
			pv:       pv,
			scenario: params.TestType,
			page:     page,
			crt:      lacros.ChromeTypeLacros,
			metrics:  &m,
		}); err != nil {
			return nil, cleanup, errors.Wrap(err, "failed to run lacros test")
		}

		if err := runCrosTest(ctx, pd, &testInvocation{
			pv:       pv,
			scenario: params.TestType,
			page:     page,
			crt:      lacros.ChromeTypeChromeOS,
			metrics:  &m,
		}); err != nil {
			return nil, cleanup, errors.Wrap(err, "failed to run cros test")
		}
	}

	if err := m.computeStatistics(ctx, pv); err != nil {
		return nil, cleanup, errors.Wrap(err, "could not compute derived statistics")
	}

	return pv, cleanup, nil
}
