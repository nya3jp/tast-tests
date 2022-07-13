// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/ui/perfutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/lacros"
	"chromiumos/tast/local/power"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         OverviewPerf,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Measures animation smoothness of entering/exiting the overview mode",
		Contacts:     []string{"amusbach@chromium.org", "oshima@chromium.org", "chromeos-perf@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Params: []testing.Param{{
			Val:     browser.TypeAsh,
			Fixture: "chromeLoggedIn",
			Timeout: 14 * time.Minute,
		}, {
			Name:    "skia_renderer",
			Val:     browser.TypeAsh,
			Fixture: "chromeLoggedInWith100FakeAppsSkiaRenderer",
			Timeout: 14 * time.Minute,
		}, {
			Name:              "lacros",
			Val:               browser.TypeLacros,
			Fixture:           "lacros",
			ExtraSoftwareDeps: []string{"lacros"},
			Timeout:           20 * time.Minute,
		}},
		Data: []string{"animation.html", "animation.js"},
	})
}

func OverviewPerf(ctx context.Context, s *testing.State) {
	// Reserve five seconds for various cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	cr, l, cs, err := lacros.Setup(ctx, s.FixtValue(), s.Param().(browser.Type))
	if err != nil {
		s.Fatal("Failed to initialize test: ", err)
	}
	defer lacros.CloseLacros(cleanupCtx, l)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	originalTabletMode, err := ash.TabletModeEnabled(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to obtain the tablet mode status: ", err)
	}
	defer ash.SetTabletModeEnabled(cleanupCtx, tconn, originalTabletMode)

	// Run an http server to serve the test contents for accessing from the chrome browsers.
	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()
	url := server.URL + "/animation.html"

	type testParam struct {
		fullDescription     string
		windowsDescription  string
		windows             int
		tablet              bool
		overviewWindowState ash.WindowStateType
		splitView           bool
		histograms          []string
	}

	genParams := func(n int) []testParam {
		params := []testParam{
			{
				fullDescription:     fmt.Sprintf("SingleClamshellMode-%dwindows", n),
				windowsDescription:  fmt.Sprintf("%dwindows", n),
				windows:             n,
				tablet:              false,
				overviewWindowState: ash.WindowStateMaximized,
				splitView:           false,
				histograms: []string{"Ash.Overview.AnimationSmoothness.Enter.SingleClamshellMode",
					"Ash.Overview.AnimationSmoothness.Exit.SingleClamshellMode"},
			}, {
				fullDescription:     fmt.Sprintf("ClamshellMode-%dwindows", n),
				windowsDescription:  fmt.Sprintf("%dwindows", n),
				windows:             n,
				tablet:              false,
				overviewWindowState: ash.WindowStateNormal,
				splitView:           false,
				histograms: []string{"Ash.Overview.AnimationSmoothness.Enter.ClamshellMode",
					"Ash.Overview.AnimationSmoothness.Exit.ClamshellMode"},
			}, {
				fullDescription:     fmt.Sprintf("TabletMode-%dwindows", n),
				windowsDescription:  fmt.Sprintf("%dwindows", n),
				windows:             n,
				tablet:              true,
				overviewWindowState: ash.WindowStateMaximized,
				splitView:           false,
				histograms: []string{"Ash.Overview.AnimationSmoothness.Enter.TabletMode",
					"Ash.Overview.AnimationSmoothness.Exit.TabletMode"},
			}, {
				fullDescription:     fmt.Sprintf("MinimizedTabletMode-%dwindows", n),
				windowsDescription:  fmt.Sprintf("%dwindows", n),
				windows:             n,
				tablet:              true,
				overviewWindowState: ash.WindowStateMinimized,
				splitView:           false,
				histograms: []string{"Ash.Overview.AnimationSmoothness.Enter.MinimizedTabletMode",
					"Ash.Overview.AnimationSmoothness.Exit.MinimizedTabletMode"},
			}}

		if n == 2 {
			params = append(params, testParam{
				fullDescription:     fmt.Sprintf("SplitView-%dwindows", n),
				windowsDescription:  fmt.Sprintf("%dwindows", n),
				windows:             n,
				tablet:              true,
				overviewWindowState: ash.WindowStateMaximized,
				splitView:           true,
				// The overview exit animation does not include the window being activated.
				// Thus, the SplitView-2windows case has no overview exit animation at all.
				histograms: []string{"Ash.Overview.AnimationSmoothness.Enter.SplitView"},
			})
		} else if n > 2 {
			params = append(params, []testParam{
				{
					fullDescription:     fmt.Sprintf("SplitView-%dwindowsincludingmaximizedoverviewwindows", n),
					windowsDescription:  fmt.Sprintf("%dwindowsincludingmaximizedoverviewwindows", n),
					windows:             n,
					tablet:              true,
					overviewWindowState: ash.WindowStateMaximized,
					splitView:           true,
					histograms: []string{"Ash.Overview.AnimationSmoothness.Enter.SplitView",
						"Ash.Overview.AnimationSmoothness.Exit.SplitView"},
				}, {
					fullDescription:     fmt.Sprintf("SplitView-%dwindowsincludingminimizedoverviewwindows", n),
					windowsDescription:  fmt.Sprintf("%dwindowsincludingminimizedoverviewwindows", n),
					windows:             n,
					tablet:              true,
					overviewWindowState: ash.WindowStateMinimized,
					splitView:           true,
					histograms: []string{"Ash.Overview.AnimationSmoothness.Enter.SplitView",
						"Ash.Overview.AnimationSmoothness.Exit.SplitView"},
				},
			}...)
		}
		return params
	}

	// Note the test only create windows so the number of windows must be in
	// increasing order here.
	params := genParams(2)
	params = append(params, genParams(3)...)
	params = append(params, genParams(4)...)
	params = append(params, genParams(8)...)

	defer ash.SetOverviewModeAndWait(cleanupCtx, tconn, false)
	runner := perfutil.NewRunner(cr.Browser())
	currentWindows := 0
	// Run the overview mode enter/exit flow for various situations.
	// - change the number of browser windows, 2 or 8
	// - the window system status; clamshell mode with maximized windows,
	//   clamshell mode with normal windows, tablet mode with maximized
	//   windows, tablet mode with minimized windows (the home screen),
	//   tablet split view with maximized overview windows, or tablet
	//   split view with minimized overview windows.
	for i, test := range params {
		// Ensure display on to record ui performance correctly. It is put inside
		// the loop because the overall duration of the loop has no input events
		// and last longer than default 7 minutes before display is turned off.
		if err := power.TurnOnDisplay(ctx); err != nil {
			s.Fatal("Failed to turn on display: ", err)
		}

		// This assumes that the test scenarios are sorted by
		// number of windows. If not, then this will generate
		// Panic: runtime error: makeslice: cap out of range
		if err := ash.CreateWindows(ctx, tconn, cs, url, test.windows-currentWindows); err != nil {
			s.Fatal("Failed to create browser windows: ", err)
		}

		// This must be done after ash.CreateWindows to avoid terminating lacros-chrome.
		if i == 0 && s.Param().(browser.Type) == browser.TypeLacros {
			if err := l.Browser().CloseWithURL(ctx, chrome.NewTabURL); err != nil {
				s.Fatal("Failed to close about:blank: ", err)
			}
		}

		currentWindows = test.windows

		// Here we try to set tablet mode enabled/disabled,
		// but keep in mind that the tests run on devices
		// that do not support tablet mode. Thus, in case
		// of an error, we skip to the next scenario.
		if err := ash.SetTabletModeEnabled(ctx, tconn, test.tablet); err != nil {
			s.Logf("Skipping the case of %s as it failed to set tablet mode %v: %v", test.fullDescription, test.tablet, err)
			continue
		}

		// Set all windows to the state that
		// we want for the overview windows.
		if err := ash.ForEachWindow(ctx, tconn, func(w *ash.Window) error {
			return ash.SetWindowStateAndWait(ctx, tconn, w.ID, test.overviewWindowState)
		}); err != nil {
			s.Fatalf("Failed to set all windows to state %v: %v", test.overviewWindowState, err)
		}

		// For tablet split view scenarios, snap
		// a window and then exit overview.
		if test.splitView {
			ws, err := ash.GetAllWindows(ctx, tconn)
			if err != nil {
				s.Fatal("Failed to get windows: ", err)
			}

			if len(ws) == 0 {
				s.Fatal("Found no windows")
			}

			if err := ash.SetWindowStateAndWait(ctx, tconn, ws[0].ID, ash.WindowStateLeftSnapped); err != nil {
				s.Fatal("Failed to snap window: ", err)
			}

			if err := ash.SetOverviewModeAndWait(ctx, tconn, false); err != nil {
				s.Fatal("Failed to exit overview: ", err)
			}
		}

		// Wait for 3 seconds to stabilize the result. Note that this doesn't
		// have to be cpu.WaitUntilIdle(). It may wait too much.
		// TODO(mukai): find the way to wait more properly on the idleness of Ash.
		// https://crbug.com/1001314.
		if err := testing.Sleep(ctx, 3*time.Second); err != nil {
			s.Fatal("Failed to wait: ", err)
		}

		runner.RunMultiple(ctx, s, test.fullDescription,
			perfutil.RunAndWaitAll(tconn,
				func(ctx context.Context) error {
					if err := ash.SetOverviewModeAndWait(ctx, tconn, true); err != nil {
						return errors.Wrap(err, "failed to enter into the overview mode")
					}
					if err := ash.SetOverviewModeAndWait(ctx, tconn, false); err != nil {
						return errors.Wrap(err, "failed to exit from the overview mode")
					}
					return nil
				},
				test.histograms...,
			),
			perfutil.StoreAll(perf.BiggerIsBetter, "percent", test.windowsDescription))
	}

	if err := runner.Values().Save(ctx, s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}
