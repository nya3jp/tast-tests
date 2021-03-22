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
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/lacros"
	"chromiumos/tast/local/lacros/launcher"
	"chromiumos/tast/local/power"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         OverviewPerf,
		Desc:         "Measures animation smoothness of entering/exiting the overview mode",
		Contacts:     []string{"mukai@chromium.org", "oshima@chromium.org", "chromeos-perf@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Params: []testing.Param{{
			Val:     lacros.ChromeTypeChromeOS,
			Fixture: "chromeLoggedIn",
			Timeout: 7 * time.Minute,
		}, {
			Name:    "skia_renderer",
			Val:     lacros.ChromeTypeChromeOS,
			Fixture: "chromeLoggedInWith100FakeAppsSkiaRenderer",
			Timeout: 7 * time.Minute,
		}, {
			Name:              "lacros",
			Val:               lacros.ChromeTypeLacros,
			Fixture:           "lacrosStartedByData",
			ExtraData:         []string{launcher.DataArtifact},
			ExtraSoftwareDeps: []string{"lacros"},
			Timeout:           10 * time.Minute,
		}},
		Data: []string{"animation.html", "animation.js"},
	})
}

func OverviewPerf(ctx context.Context, s *testing.State) {
	// Reserve five seconds for various cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	// Ensure display on to record ui performance correctly.
	if err := power.TurnOnDisplay(ctx); err != nil {
		s.Fatal("Failed to turn on display: ", err)
	}

	// TODO(crbug.com/1127165): Remove the artifactPath argument when we can use Data in fixtures.
	var artifactPath string
	if s.Param().(lacros.ChromeType) == lacros.ChromeTypeLacros {
		artifactPath = s.DataPath(launcher.DataArtifact)
	}
	cr, l, cs, err := lacros.Setup(ctx, s.FixtValue(), artifactPath, s.Param().(lacros.ChromeType))
	if err != nil {
		s.Fatal("Failed to initialize test: ", err)
	}
	defer lacros.CloseLacrosChrome(cleanupCtx, l)

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

	runner := perfutil.NewRunner(cr)
	currentWindows := 0
	// Run the overview mode enter/exit flow for various situations.
	// - change the number of browser windows, 2 or 8
	// - the window system status; clamshell mode with maximized windows,
	//   clamshell mode with normal windows, tablet mode with maximized
	//   windows, tablet mode with minimized windows (the home screen),
	//   tablet split view with maximized overview windows, or tablet
	//   split view with minimized overview windows.
	// If these window number values are changed, make sure to check lacros about:blank pages are closed correctly.
	for i, windows := range []int{2, 8} {
		if err := ash.CreateWindows(ctx, tconn, cs, url, windows-currentWindows); err != nil {
			s.Fatal("Failed to create browser windows: ", err)
		}

		// This must be done after ash.CreateWindows to avoid terminating lacros-chrome.
		if i == 0 && s.Param().(lacros.ChromeType) == lacros.ChromeTypeLacros {
			if err := lacros.CloseAboutBlank(ctx, tconn, l.Devsess, 1); err != nil {
				s.Fatal("Failed to close about:blank: ", err)
			}
		}

		currentWindows = windows

		type testCase struct {
			histogramSuffix     string
			extraDescriptionFmt string
			tablet              bool
			overviewWindowState ash.WindowStateType
			splitView           bool
		}
		tests := []testCase{
			{
				histogramSuffix:     "SingleClamshellMode",
				extraDescriptionFmt: "%dwindows",
				tablet:              false,
				overviewWindowState: ash.WindowStateMaximized,
				splitView:           false,
			},
			{
				histogramSuffix:     "ClamshellMode",
				extraDescriptionFmt: "%dwindows",
				tablet:              false,
				overviewWindowState: ash.WindowStateNormal,
				splitView:           false,
			},
			{
				histogramSuffix:     "TabletMode",
				extraDescriptionFmt: "%dwindows",
				tablet:              true,
				overviewWindowState: ash.WindowStateMaximized,
				splitView:           false,
			},
			{
				histogramSuffix:     "MinimizedTabletMode",
				extraDescriptionFmt: "%dwindows",
				tablet:              true,
				overviewWindowState: ash.WindowStateMinimized,
				splitView:           false,
			},
		}
		// In the split view cases, two windows will be snapped. If there
		// are three or more, the rest may be maximized or minimized.
		if windows >= 3 {
			tests = append(tests,
				testCase{
					histogramSuffix:     "SplitView",
					extraDescriptionFmt: "%dwindowsincludingmaximizedoverviewwindows",
					tablet:              true,
					overviewWindowState: ash.WindowStateMaximized,
					splitView:           true,
				},
				testCase{
					histogramSuffix:     "SplitView",
					extraDescriptionFmt: "%dwindowsincludingminimizedoverviewwindows",
					tablet:              true,
					overviewWindowState: ash.WindowStateMinimized,
					splitView:           true,
				},
			)
		} else {
			tests = append(tests,
				testCase{
					histogramSuffix:     "SplitView",
					extraDescriptionFmt: "%dwindows",
					tablet:              true,
					overviewWindowState: ash.WindowStateMaximized,
					splitView:           true,
				},
			)
		}
		for _, test := range tests {
			extraDescription := fmt.Sprintf(test.extraDescriptionFmt, windows)
			fullDescription := fmt.Sprintf("%s-%s", test.histogramSuffix, extraDescription)
			if err := ash.SetTabletModeEnabled(ctx, tconn, test.tablet); err != nil {
				s.Logf("Skipping the case of %s as it failed to set tablet mode %v: %v", fullDescription, test.tablet, err)
				continue
			}

			if err := ash.ForEachWindow(ctx, tconn, func(w *ash.Window) error {
				return ash.SetWindowStateAndWait(ctx, tconn, w.ID, test.overviewWindowState)
			}); err != nil {
				s.Fatalf("Failed to set all windows to state %v: %v", test.overviewWindowState, err)
			}

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

			histograms := []string{"Ash.Overview.AnimationSmoothness.Enter." + test.histogramSuffix}
			// The overview exit animation does not include the window being
			// activated. So Ash.Overview.AnimationSmoothness.Exit.SplitView
			// requires at least three windows: one snapped, one being
			// activated, and one in the overview exit animation.
			if !test.splitView || windows >= 3 {
				histograms = append(histograms, "Ash.Overview.AnimationSmoothness.Exit."+test.histogramSuffix)
			}
			runner.RunMultiple(ctx, s, fullDescription, perfutil.RunAndWaitAll(tconn, func(ctx context.Context) error {
				if err := ash.SetOverviewModeAndWait(ctx, tconn, true); err != nil {
					return errors.Wrap(err, "failed to enter into the overview mode")
				}
				if err := ash.SetOverviewModeAndWait(ctx, tconn, false); err != nil {
					return errors.Wrap(err, "failed to exit from the overview mode")
				}
				return nil
			},
				histograms...),
				perfutil.StoreAll(perf.BiggerIsBetter, "percent", extraDescription))
		}
	}

	if err := runner.Values().Save(ctx, s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}
