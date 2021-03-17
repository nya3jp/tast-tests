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
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/ui/perfutil"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/lacros"
	"chromiumos/tast/local/lacros/launcher"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         OverviewPerf,
		Desc:         "Measures animation smoothness of entering/exiting the overview mode",
		Contacts:     []string{"mukai@chromium.org", "oshima@chromium.org", "chromeos-wmp@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Timeout:      8 * time.Minute,
		Params: []testing.Param{{
			Val:     lacros.ChromeTypeChromeOS,
			Fixture: "chromeLoggedIn",
		}, {
			Name:    "skia_renderer",
			Val:     lacros.ChromeTypeChromeOS,
			Fixture: "chromeLoggedInWith100FakeAppsSkiaRenderer",
		}, {
			Name:              "lacros",
			Val:               lacros.ChromeTypeLacros,
			Fixture:           "lacrosStartedByData",
			ExtraData:         []string{launcher.DataArtifact},
			ExtraSoftwareDeps: []string{"lacros"},
		}},
		Data: []string{"animation.html", "animation.js"},
	})
}

func OverviewPerf(ctx context.Context, s *testing.State) {
	// Ensure display on to record ui performance correctly.
	if err := perfutil.EnsureDisplayOn(ctx); err != nil {
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
	defer lacros.CloseLacrosChrome(ctx, l)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	originalTabletMode, err := ash.TabletModeEnabled(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to obtain the tablet mode status: ", err)
	}
	defer ash.SetTabletModeEnabled(ctx, tconn, originalTabletMode)

	// overviewAnimationType specifies the type of the animation of entering to or
	// exiting from the overview mode.
	type overviewAnimationType int
	const (
		// animationTypeMaximized is the animation when there are maximized windows
		// in the clamshell mode.
		animationTypeMaximized overviewAnimationType = iota
		// animationTypeNormalWindow is the animation for normal windows in the
		// clamshell mode.
		animationTypeNormalWindow
		// animationTypeTabletMode is the animation for windows in the tablet mode.
		animationTypeTabletMode
		// animationTypeTabletMode is the animation for windows in the tablet mode
		// when they are all minimized
		animationTypeMinimizedTabletMode
	)

	// Run an http server to serve the test contents for accessing from the chrome browsers.
	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()
	url := server.URL + "/animation.html"

	r := perfutil.NewRunner(cr)
	currentWindows := 0
	// Run the overview mode enter/exit flow for various situations.
	// - change the number of browser windows, 2 or 8
	// - the window system status; clamshell mode with maximized windows or
	//   tablet mode.
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

		for _, state := range []overviewAnimationType{animationTypeMaximized, animationTypeNormalWindow, animationTypeTabletMode, animationTypeMinimizedTabletMode} {
			inTabletMode := (state == animationTypeTabletMode || state == animationTypeMinimizedTabletMode)
			if err = ash.SetTabletModeEnabled(ctx, tconn, inTabletMode); err != nil {
				s.Fatalf("Failed to set tablet mode %v: %v", inTabletMode, err)
			}

			windowState := ash.WindowStateNormal
			if state == animationTypeMaximized || state == animationTypeTabletMode {
				windowState = ash.WindowStateMaximized
			} else if state == animationTypeMinimizedTabletMode {
				windowState = ash.WindowStateMinimized
			}
			if err := ash.ForEachWindow(ctx, tconn, func(w *ash.Window) error {
				return ash.SetWindowStateAndWait(ctx, tconn, w.ID, windowState)
			}); err != nil {
				s.Fatalf("Failed to set all windows to state %v: %v", windowState, err)
			}

			// Wait for 3 seconds to stabilize the result. Note that this doesn't
			// have to be cpu.WaitUntilIdle(). It may wait too much.
			// TODO(mukai): find the way to wait more properly on the idleness of Ash.
			// https://crbug.com/1001314.
			if err = testing.Sleep(ctx, 3*time.Second); err != nil {
				s.Fatal("Failed to wait: ", err)
			}

			var suffix string
			switch state {
			case animationTypeMaximized:
				suffix = "SingleClamshellMode"
			case animationTypeNormalWindow:
				suffix = "ClamshellMode"
			case animationTypeTabletMode:
				suffix = "TabletMode"
			case animationTypeMinimizedTabletMode:
				suffix = "MinimizedTabletMode"
			}

			r.RunMultiple(ctx, s, fmt.Sprintf("%s-%dwindows", suffix, currentWindows), perfutil.RunAndWaitAll(tconn, func(ctx context.Context) error {
				if err = ash.SetOverviewModeAndWait(ctx, tconn, true); err != nil {
					return errors.Wrap(err, "failed to enter into the overview mode")
				}
				if err = ash.SetOverviewModeAndWait(ctx, tconn, false); err != nil {
					return errors.Wrap(err, "failed to exit from the overview mode")
				}
				return nil
			},
				"Ash.Overview.AnimationSmoothness.Enter"+"."+suffix,
				"Ash.Overview.AnimationSmoothness.Exit"+"."+suffix),
				perfutil.StoreAll(perf.BiggerIsBetter, "percent", fmt.Sprintf("%dwindows", currentWindows)))
		}
	}

	if err := r.Values().Save(ctx, s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}
