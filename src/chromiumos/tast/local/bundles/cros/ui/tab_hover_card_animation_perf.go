// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	uiperf "chromiumos/tast/local/bundles/cros/ui/perf"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/perfutil"
	"chromiumos/tast/local/power"
	"chromiumos/tast/local/ui"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         TabHoverCardAnimationPerf,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Measures the animation smoothness of tab hover card animation",
		Contacts:     []string{"yichenz@chromium.org", "chromeos-wmp@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Params: []testing.Param{{
			Fixture: "chromeLoggedIn",
			Val:     browser.TypeAsh,
		}, {
			Name:              "lacros",
			Fixture:           "lacros",
			ExtraSoftwareDeps: []string{"lacros"},
			Val:               browser.TypeLacros,
		}},
		Timeout: 4 * time.Minute,
	})
}

func TabHoverCardAnimationPerf(ctx context.Context, s *testing.State) {
	// Reserve a few seconds for various cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	// Ensure display on to record ui performance correctly.
	if err := power.TurnOnDisplay(ctx); err != nil {
		s.Fatal("Failed to turn on display: ", err)
	}

	cr := s.FixtValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		s.Fatal("Failed to ensure in clamshell mode: ", err)
	}
	defer cleanup(ctx)

	// Open two browser windows.
	var conn *browser.Conn
	var br *browser.Browser
	var closeBrowser func(ctx context.Context) error
	for i := 0; i < 2; i++ {
		if i == 0 {
			if conn, br, closeBrowser, err = browserfixt.SetUpWithURL(ctx, cr, s.Param().(browser.Type), ui.PerftestURL); err == nil {
				defer closeBrowser(cleanupCtx)
			}
		} else {
			conn, err = br.NewConn(ctx, ui.PerftestURL)
		}
		if err != nil {
			s.Fatalf("Failed to open %d-th tab: %v", i, err)
		}
		if err := conn.Close(); err != nil {
			s.Fatalf("Failed to close the connection to %d-th tab: %v", i, err)
		}
	}

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	ac := uiauto.New(tconn)
	webviewLocation, err := ac.WithTimeout(10*time.Second).Location(ctx, nodewith.Role(role.WebView).ClassName("ContentsWebView"))
	if err != nil {
		s.Fatal("Failed to find the webview location: ", err)
	}
	center := webviewLocation.CenterPoint()

	// Find tabs.
	tabs, err := ac.NodesInfo(ctx, nodewith.Role(role.Tab).ClassName("Tab"))
	if err != nil {
		s.Fatal("Failed to find tabs: ", err)
	}

	// TODO(TBD): For Lacros, passing (`cr` && `tconn`) or (`br` && `bTconn`) here doesn't report any data for the metrics.
	bTconn, err := br.TestAPIConn(ctx)
	if err != nil {
		s.Fatalf("Failed to connect to test API for %v: %v", s.Param().(browser.Type), err)
	}
	runner := perfutil.NewRunner(br)
	for _, data := range []struct {
		tab    uiauto.NodeInfo
		suffix string
	}{
		{tabs[0], "inactive"},
		{tabs[1], "active"},
	} {
		runner.RunMultiple(ctx, data.suffix, uiperf.Run(s, perfutil.RunAndWaitAll(bTconn, func(ctx context.Context) error {
			return uiauto.Combine(
				"hover and exit",
				mouse.Move(tconn, center, 0),
				mouse.Move(tconn, data.tab.Location.CenterPoint(), 500*time.Millisecond),
				// Waiting to make the hover card appear.
				func(ctx context.Context) error { return testing.Sleep(ctx, 4*time.Second) },
				mouse.Move(tconn, center, 500*time.Millisecond),
			)(ctx)
		},
			"Chrome.Tabs.AnimationSmoothness.HoverCard.FadeIn",
			"Chrome.Tabs.AnimationSmoothness.HoverCard.FadeOut")),
			perfutil.StoreSmoothness)
	}
	if err := runner.Values().Save(ctx, s.OutDir()); err != nil {
		s.Fatal("Failed to save perf data: ", err)
	}
}
