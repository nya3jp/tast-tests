// Copyright 2019 The ChromiumOS Authors
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
	uiperf "chromiumos/tast/local/bundles/cros/ui/perf"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/lacros"
	"chromiumos/tast/local/perfutil"
	"chromiumos/tast/local/power"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         OverviewPerf,
		LacrosStatus: testing.LacrosVariantExists,
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
			Name:              "lacros",
			Val:               browser.TypeLacros,
			Fixture:           "lacros",
			ExtraSoftwareDeps: []string{"lacros"},
			Timeout:           20 * time.Minute,
		}, {
			Name:    "passthrough",
			Val:     browser.TypeAsh,
			Fixture: "chromeLoggedInWith100FakeAppsPassthroughCmdDecoder",
			Timeout: 14 * time.Minute,
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

	defer ash.SetOverviewModeAndWait(cleanupCtx, tconn, false)
	runner := perfutil.NewRunner(cr.Browser())
	currentWindows := 0
	// Run the overview mode enter/exit flow for various situations.
	// - change the number of browser windows,
	// - the window system status; clamshell mode with maximized windows,
	//   clamshell mode with normal windows, tablet mode with maximized
	//   windows, tablet mode with minimized windows (the home screen),
	//   tablet split view with maximized overview windows, or tablet
	//   split view with minimized overview windows.
	for i, windows := range []int{2, 3, 4, 8} {
		// This assumes that the test scenarios are sorted by
		// number of windows. If not, then this will generate
		// Panic: runtime error: makeslice: cap out of range
		if err := ash.CreateWindows(ctx, tconn, cs, url, windows-currentWindows); err != nil {
			s.Fatal("Failed to create browser windows: ", err)
		}

		// This must be done after ash.CreateWindows to avoid terminating lacros-chrome.
		if i == 0 && s.Param().(browser.Type) == browser.TypeLacros {
			if err := l.Browser().CloseWithURL(ctx, chrome.NewTabURL); err != nil {
				s.Fatal("Failed to close about:blank: ", err)
			}
		}

		currentWindows = windows

		windowsDescription := fmt.Sprintf("%dwindows", windows)
		for _, test := range []struct {
			fullDescriptionFmt  string
			tablet              bool
			overviewWindowState ash.WindowStateType
			histogramSuffix     string
		}{
			{"SingleClamshellMode-%dwindows", false, ash.WindowStateMaximized, "SingleClamshellMode"},
			{"ClamshellMode-%dwindows", false, ash.WindowStateNormal, "ClamshellMode"},
			{"TabletMode-%dwindows", true, ash.WindowStateMaximized, "TabletMode"},
			{"MinimizedTabletMode-%dwindows", true, ash.WindowStateMinimized, "MinimizedTabletMode"},
		} {
			fullDescription := fmt.Sprintf(test.fullDescriptionFmt, windows)
			if err := doTestCase(
				ctx, s, tconn, runner, fullDescription, windowsDescription, test.tablet, test.overviewWindowState,
				false /*splitview*/, "Ash.Overview.AnimationSmoothness.Enter."+test.histogramSuffix,
				"Ash.Overview.AnimationSmoothness.Exit."+test.histogramSuffix,
			); err != nil {
				s.Fatalf("Test case %q failed: %s", fullDescription, err)
			}
		}

		if windows == 2 {
			// The overview exit animation does not include the window being activated.
			// Thus, the SplitView-2windows case has no overview exit animation at all.
			if err := doTestCase(
				ctx, s, tconn, runner, "SplitView-2windows", "2windows", true /*tablet*/, ash.WindowStateMaximized,
				true /*splitview*/, "Ash.Overview.AnimationSmoothness.Enter.SplitView",
			); err != nil {
				s.Fatal("Test case \"SplitView-2windows\" failed: ", err)
			}
			continue
		}

		for _, test := range []struct {
			fullDescriptionFmt    string
			windowsDescriptionFmt string
			overviewWindowState   ash.WindowStateType
		}{
			{"SplitView-%dwindowsincludingmaximizedoverviewwindows", "%dwindowsincludingmaximizedoverviewwindows", ash.WindowStateMaximized},
			{"SplitView-%dwindowsincludingminimizedoverviewwindows", "%dwindowsincludingminimizedoverviewwindows", ash.WindowStateMinimized},
		} {
			fullDescription := fmt.Sprintf(test.fullDescriptionFmt, windows)
			windowsDescription := fmt.Sprintf(test.windowsDescriptionFmt, windows)
			if err := doTestCase(
				ctx, s, tconn, runner, fullDescription, windowsDescription, true /*tablet*/, test.overviewWindowState,
				true /*splitview*/, "Ash.Overview.AnimationSmoothness.Enter.SplitView",
				"Ash.Overview.AnimationSmoothness.Exit.SplitView",
			); err != nil {
				s.Fatalf("Test case %q failed: %s", fullDescription, err)
			}
		}
	}

	if err := runner.Values().Save(ctx, s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}

// doTestCase runs the overview entering/exiting test using the given arguments.
func doTestCase(
	ctx context.Context,
	s *testing.State,
	tconn *chrome.TestConn,
	runner *perfutil.Runner,
	fullDescription,
	windowsDescription string,
	tablet bool,
	overviewWindowState ash.WindowStateType,
	splitView bool,
	histograms ...string,
) error {
	// Ensure display on to record ui performance correctly.
	if err := power.TurnOnDisplay(ctx); err != nil {
		return errors.Wrap(err, "failed to turn on display")
	}

	// Here we try to set tablet mode enabled/disabled,
	// but keep in mind that the tests run on devices
	// that do not support tablet mode. Thus, in case
	// of an error, we skip to the next scenario.
	if err := ash.SetTabletModeEnabled(ctx, tconn, tablet); err != nil {
		testing.ContextLogf(ctx, "Skipping the case of %s as it failed to set tablet mode %v: %v", fullDescription, tablet, err)
		return nil
	}

	// Set all windows to the state that
	// we want for the overview windows.
	if err := ash.ForEachWindow(ctx, tconn, func(w *ash.Window) error {
		return ash.SetWindowStateAndWait(ctx, tconn, w.ID, overviewWindowState)
	}); err != nil {
		return errors.Wrapf(err, "failed to set all windows to state %v", overviewWindowState)
	}

	// For tablet split view scenarios, snap
	// a window and then exit overview.
	if splitView {
		ws, err := ash.GetAllWindows(ctx, tconn)
		if err != nil {
			return errors.Wrap(err, "failed to get windows")
		}

		if len(ws) == 0 {
			return errors.Wrap(err, "found no windows")
		}

		if err := ash.SetWindowStateAndWait(ctx, tconn, ws[0].ID, ash.WindowStateLeftSnapped); err != nil {
			return errors.Wrap(err, "failed to snap window")
		}

		if err := ash.SetOverviewModeAndWait(ctx, tconn, false); err != nil {
			return errors.Wrap(err, "failed to exit overview")
		}
	}

	// Wait for 3 seconds to stabilize the result. Note that this doesn't
	// have to be cpu.WaitUntilIdle(). It may wait too much.
	// TODO(mukai): find the way to wait more properly on the idleness of Ash.
	// https://crbug.com/1001314.
	if err := testing.Sleep(ctx, 3*time.Second); err != nil {
		return errors.Wrap(err, "failed to wait")
	}

	runner.RunMultiple(ctx, fullDescription, uiperf.Run(s,
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
			histograms...,
		)),
		perfutil.StoreAll(perf.BiggerIsBetter, "percent", windowsDescription))

	return nil
}
