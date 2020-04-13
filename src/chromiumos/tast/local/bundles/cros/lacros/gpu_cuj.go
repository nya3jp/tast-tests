// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package lacros tests linux-chrome running on ChromeOS.
package lacros

import (
	"context"
	"fmt"
	"sort"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/lacros/launcher"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/cdputil"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/local/chrome/ui"
	chromeui "chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/testing"
)

type gpuCUJTestParams struct {
	url      string
	threedot bool
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
				threedot: false,
			},
			Pre: launcher.StartedByDataForceComposition(),
		}, {
			Name: "aquarium_threedot",
			Val: gpuCUJTestParams{
				url:      "https://webglsamples.org/aquarium/aquarium.html",
				threedot: true,
			},
			Pre: launcher.StartedByData(),
		}, {
			Name: "aquarium",
			Val: gpuCUJTestParams{
				url:      "https://webglsamples.org/aquarium/aquarium.html",
				threedot: false,
			},
			Pre: launcher.StartedByData(),
		}, {
			Name: "poster_composited",
			Val: gpuCUJTestParams{
				url:      "https://webkit.org/blog-files/3d-transforms/poster-circle.html",
				threedot: false,
			},
			Pre: launcher.StartedByDataForceComposition(),
		}, {
			Name: "poster_threedot",
			Val: gpuCUJTestParams{
				url:      "https://webkit.org/blog-files/3d-transforms/poster-circle.html",
				threedot: true,
			},
			Pre: launcher.StartedByData(),
		}, {
			Name: "poster",
			Val: gpuCUJTestParams{
				url:      "https://webkit.org/blog-files/3d-transforms/poster-circle.html",
				threedot: false,
			},
			Pre: launcher.StartedByData(),
		}},
	})
}

var pollOptions = &testing.PollOptions{Timeout: 10 * time.Second}

func waitForWindowState(ctx context.Context, ctconn *chrome.TestConn, w *ash.Window, state ash.WindowStateType) error {
	return ash.WaitForCondition(ctx, ctconn, func(cw *ash.Window) bool {
		// Wait for the window given by w to be in the given state and also not be animating.
		return w.ID == cw.ID && cw.State == state && !cw.IsAnimating
	}, pollOptions)
}

func leftClickLacros(ctx context.Context, ctconn *chrome.TestConn, n *ui.Node) error {
	if err := n.Update(ctx); err != nil {
		return errors.Wrap(err, "failed to update the node's location")
	}
	if n.Location.Empty() {
		return errors.New("this node doesn't have a location on the screen and can't be clicked")
	}
	w, err := findFirstWindow(ctx, ctconn)
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

func maximizeFirstWindow(ctx context.Context, ctconn *chrome.TestConn) error {
	w, err := findFirstWindow(ctx, ctconn)
	if err != nil {
		return err
	}
	if _, err := ash.SetWindowState(ctx, ctconn, w.ID, ash.WMEventMaximize); err != nil {
		return err
	}
	return waitForWindowState(ctx, ctconn, w, ash.WindowStateMaximized)
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
	"Graphics.Smoothness.PercentDroppedFrames.CompositorThread.AllSequences": {
		unit:      "percent",
		direction: perf.SmallerIsBetter,
	},
	"Graphics.Smoothness.PercentDroppedFrames.MainThread.Universal": {
		unit:      "percent",
		direction: perf.SmallerIsBetter,
	},
	"Graphics.Smoothness.PercentDroppedFrames.MainThread.AllSequences": {
		unit:      "percent",
		direction: perf.SmallerIsBetter,
	},
	"Graphics.Smoothness.PercentDroppedFrames.SlowerThread.Universal": {
		unit:      "percent",
		direction: perf.SmallerIsBetter,
	},
	"Graphics.Smoothness.PercentDroppedFrames.SlowerThread.AllSequences": {
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

func runHistogram(ctx context.Context, tconn *chrome.TestConn, pv *perf.Values, testType string) error {
	var keys []string
	for k := range histogramMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	histograms, err := metrics.Run(ctx, tconn, func() error {
		testing.Sleep(ctx, 20.0*time.Second)
		return nil
	}, keys...)
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

func runTestLacros(ctx context.Context, pd launcher.PreData, pv *perf.Values, p gpuCUJTestParams) error {
	ctconn, err := pd.Chrome.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to connect to test API")
	}

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

	// Maximize linux-chrome window.
	if err := maximizeFirstWindow(ctx, ctconn); err != nil {
		return errors.Wrap(err, "failed to maximize linux-chrome")
	}

	// Open the threedot menu if indicated.
	// TODO(edcourtney): Sometimes the accessibility isn't populated for linux chrome, which causes this code to fail.
	if p.threedot {
		clickFn := func(n *ui.Node) error { return leftClickLacros(ctx, ctconn, n) }
		if err := toggleThreeDotMenu(ctx, ltconn, clickFn); err != nil {
			return errors.Wrap(err, "failed to open three dot menu")
		}
		defer toggleThreeDotMenu(ctx, ltconn, clickFn)
	}

	if err := runHistogram(ctx, ltconn, pv, "lacros"); err != nil {
		return err
	}

	return nil
}

func runTestChromeOS(ctx context.Context, pd launcher.PreData, pv *perf.Values, p gpuCUJTestParams) error {
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

	// Maximize chrome window.
	if err := maximizeFirstWindow(ctx, ctconn); err != nil {
		return errors.Wrap(err, "failed to maximize chrome")
	}

	// Open the threedot menu if indicated.
	if p.threedot {
		clickFn := func(n *ui.Node) error { return n.LeftClick(ctx) }
		if err := toggleThreeDotMenu(ctx, ctconn, clickFn); err != nil {
			return errors.Wrap(err, "failed to open three dot menu")
		}
		defer toggleThreeDotMenu(ctx, ctconn, clickFn)
	}

	if err := runHistogram(ctx, ctconn, pv, "cros"); err != nil {
		return err
	}

	return nil
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
	if err := runTestLacros(ctx, s.PreValue().(launcher.PreData), pv, p); err != nil {
		s.Fatal("Failed to run test: ", err)
	}

	if err := runTestChromeOS(ctx, s.PreValue().(launcher.PreData), pv, p); err != nil {
		s.Fatal("Failed to run test: ", err)
	}

	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Cannot save perf data: ", err)
	}
}
