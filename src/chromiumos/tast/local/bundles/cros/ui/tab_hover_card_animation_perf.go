// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/ui/perfutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
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
		Timeout:      4 * time.Minute,
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
		s.Fatalf("Expected 2 tabs, only found %v tab(s)", len(tabs))
	}

	runner := perfutil.NewRunner(cr)
	for _, data := range []struct {
		tab    *chromeui.Node
		suffix string
	}{
		{tabs[0], "inactive"},
		{tabs[1], "active"},
	} {
		s.Run(ctx, data.suffix, func(ctx context.Context, s *testing.State) {
			// Stabilize CPU usage.
			if err := cpu.WaitUntilIdle(ctx); err != nil {
				s.Error("Failed to wait for system UI to be stabilized: ", err)
			}

			runner.RunMultiple(ctx, s, data.suffix, perfutil.RunAndWaitAll(tconn, func() error {
				if err := mouse.Move(ctx, tconn, center, 0); err != nil {
					return errors.Wrap(err, "failed to put the mouse to the center")
				}
				if err := mouse.Move(ctx, tconn, data.tab.Location.CenterPoint(), 500*time.Millisecond); err != nil {
					return errors.Wrapf(err, "failed to move the mouse to the %s tab", data.suffix)
				}
				// Hover on the tab.
				if err := testing.Sleep(ctx, 4*time.Second); err != nil {
					return errors.Wrap(err, "failed to sleep for 5 seconds")
				}
				if err := mouse.Move(ctx, tconn, center, 500*time.Millisecond); err != nil {
					return errors.Wrap(err, "failed to move the mouse back to the center")
				}
				return nil
			},
				"Chrome.Tabs.AnimationSmoothness.HoverCard.FadeIn",
				"Chrome.Tabs.AnimationSmoothness.HoverCard.FadeOut"),
				perfutil.StoreSmoothness)
		})
	}
	if err := runner.Values().Save(s.OutDir()); err != nil {
		s.Fatal("Failed to save perf data: ", err)
	}
}
