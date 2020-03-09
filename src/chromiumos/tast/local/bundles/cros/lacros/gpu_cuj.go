// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package lacros tests linux-chrome running on ChromeOS.
package lacros

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/lacros/launcher"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/cdputil"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         GpuCUJ,
		Desc:         "Lacros GPU performance CUJ tests",
		Contacts:     []string{"edcourtney@chromium.org", "hidehiko@chromium.org", "lacros-team@google.com"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"disabled"},
		Pre:          launcher.StartedByData(),
		Timeout:      60 * time.Minute,
		Data:         []string{launcher.DataArtifact},
		Params: []testing.Param{{
			Name: "aquarium",
			Val:  "https://webglsamples.org/aquarium/aquarium.html",
		}, {
			Name: "poster",
			Val:  "https://webkit.org/blog-files/3d-transforms/poster-circle.html",
		}},
	})
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
	_, err = ash.SetWindowState(ctx, ctconn, w.ID, ash.WMEventMaximize)
	return err
}

func closeAboutBlank(ctx context.Context, ds *cdputil.Session) error {
	targetFilter := func(t *cdputil.Target) bool {
		return t.URL == chrome.BlankURL
	}
	targets, err := ds.FindTargets(ctx, targetFilter)
	if err != nil {
		return errors.Wrap(err, "failed to query for about:blank pages")
	}
	for _, info := range targets {
		ds.CloseTarget(ctx, info.TargetID)
	}
	return nil
}

func runHistogram(ctx context.Context, tconn *chrome.TestConn, pv *perf.Values, testType string) error {
	histograms, err := metrics.Run(ctx, tconn, func() error {
		testing.Sleep(ctx, 20.0*time.Second)
		return nil
	}, "Graphics.Smoothness.PercentDroppedFrames.CompositorThread.Universal",
		"Graphics.Smoothness.PercentDroppedFrames.CompositorThread.AllSequences",
		"Graphics.Smoothness.PercentDroppedFrames.MainThread.Universal",
		"Graphics.Smoothness.PercentDroppedFrames.MainThread.AllSequences",
		"Graphics.Smoothness.PercentDroppedFrames.SlowerThread.Universal",
		"Graphics.Smoothness.PercentDroppedFrames.SlowerThread.AllSequences",
		"Graphics.Smoothness.PercentDroppedFrames.AllSequences",
		"Compositing.Display.DrawToSwapUs",
	)
	if err != nil {
		return errors.Wrap(err, "failed to get histograms")
	}

	for _, h := range histograms {
		testing.ContextLog(ctx, "Histogram: ", h)

		if h.TotalCount() != 0 {
			mean, err := h.Mean()
			if err != nil {
				return errors.Wrapf(err, "failed to get mean for histogram: %s", h.Name)
			}
			testing.ContextLog(ctx, "Mean: ", mean)

			pv.Set(perf.Metric{
				Name:      fmt.Sprintf("%s.%s", h.Name, testType),
				Unit:      "",
				Direction: perf.SmallerIsBetter,
			}, mean)
		}
	}
	return nil
}

func GpuCUJ(ctx context.Context, s *testing.State) {
	ctconn, err := s.PreValue().(launcher.PreData).Chrome.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	// Launch linux-chrome with about:blank loaded first - we don't want to include startup cost.
	l, err := launcher.LaunchLinuxChrome(ctx, s.PreValue().(launcher.PreData))
	if err != nil {
		s.Fatal("Failed to launch linux-chrome: ", err)
	}
	defer l.Close(ctx)

	ltconn, err := l.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	// Wait for quiescent state.
	if err := cpu.WaitUntilIdle(ctx); err != nil {
		s.Fatal("Failed waiting for CPU to become idle: ", err)
	}

	conn, err := l.NewConn(ctx, s.Param().(string))
	if err != nil {
		s.Fatal("Failed to open new tab: ", err)
	}
	defer conn.Close()

	// Close the initial "about:blank" tab present at startup.
	if err = closeAboutBlank(ctx, l.Devsess); err != nil {
		s.Fatal("Failed to close about:blank tab: ", err)
	}

	// Maximize linux-chrome window.
	if err = maximizeFirstWindow(ctx, ctconn); err != nil {
		s.Fatal("Failed to maximize linux-chrome: ", err)
	}

	pv := perf.NewValues()

	err = runHistogram(ctx, ltconn, pv, "lacros")
	if err != nil {
		s.Fatal("Failed to get histograms: ", err)
	}

	if err = pv.Save(s.OutDir()); err != nil {
		s.Error("Cannot save perf data: ", err)
	}
}
