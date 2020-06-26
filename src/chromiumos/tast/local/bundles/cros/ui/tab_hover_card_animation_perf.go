// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/metrics"
	chromeui "chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/chrome/ui/mouse"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/local/ui"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         TabHoverCardAnimationPerf,
		Desc:         "Measures the animation smoothness of tab hover card animation",
		Contacts:     []string{"yichenz@chromium.org", "chromeos-wmp@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Pre:          chrome.LoggedIn(),
		Timeout:      3 * time.Minute,
	})
}

func TabHoverCardAnimationPerf(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		s.Fatal("Failed to ensure in clamshell mode: ", err)
	}
	defer cleanup(ctx)

	for i := 0; i < 2; i++ {
		conn, err := cr.NewConn(ctx, ui.PerftestURL)
		if err != nil {
			s.Fatalf("Failed to open %d-th tab: %v", i, err)
		}
		if err := conn.Close(); err != nil {
			s.Fatalf("Failed to close the connection to %d-th tab: %v", i, err)
		}
	}

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	webview, err := chromeui.FindWithTimeout(ctx, tconn, chromeui.FindParams{Role: chromeui.RoleTypeWebView, ClassName: "WebView"}, 10*time.Second)
	if err != nil {
		s.Fatal("Failed to find webview: ", err)
	}
	defer webview.Release(ctx)
	center := webview.Location.CenterPoint()

	// Find tabs.
	tabs, err := chromeui.FindAll(ctx, tconn, chromeui.FindParams{Role: chromeui.RoleTypeTab, ClassName: "Tab"})
	if err != nil {
		s.Fatal("Failed to find tabs: ", err)
	}
	defer tabs.Release(ctx)
	if len(tabs) != 2 {
		s.Fatalf("expected 2 tabs, only found %v tab(s)", len(tabs))
	}

	pv := perf.NewValues()
	for _, data := range []struct {
		tab    *chromeui.Node
		suffix string
	}{
		{tabs[0], "inactive"},
		{tabs[1], "active"},
	} {
		// Stabilize CPU usage.
		if err := cpu.WaitUntilIdle(ctx); err != nil {
			s.Error("Failed to wait for system UI to be stabilized: ", err)
		}

		hists, err := metrics.RunAndWaitAll(ctx, tconn, time.Second, func() error {
			if err := mouse.Move(ctx, tconn, center, 0); err != nil {
				s.Fatalf("Failed to put mouse to the center: ", err)
			}
			if err := mouse.Move(ctx, tconn, data.tab.Location.CenterPoint(), time.Second); err != nil {
				s.Fatalf("Failed to move mouse to the %s tab: ", data.suffix, err)
			}
			// Hover on the tab.
			if err := testing.Sleep(ctx, 5*time.Second); err != nil {
				s.Fatal("Failed to sleep for 5 seconds: ", err)
			}
			if err := mouse.Move(ctx, tconn, center, time.Second); err != nil {
				s.Fatal("Failed to move mouse back to the center: ", err)
			}
			return nil
		},
			"Chrome.Tabs.AnimationSmoothness.HoverCard.FadeIn",
			"Chrome.Tabs.AnimationSmoothness.HoverCard.FadeOut")
		if err != nil {
			s.Fatal("Failed to move mouse or get the histogram: ", err)
		}

		for _, h := range hists {
			mean, err := h.Mean()
			if err != nil {
				s.Fatalf("Failed to get mean for histogram %s (%s tab): %v ", h.Name, data.suffix, err)
			}
			pv.Set(perf.Metric{
				Name:      h.Name + "." + data.suffix,
				Unit:      "percent",
				Direction: perf.BiggerIsBetter,
			}, mean)
		}
	}
	if err := pv.Save(s.OutDir()); err != nil {
		s.Fatal("Failed to save perf data: ", err)
	}
}