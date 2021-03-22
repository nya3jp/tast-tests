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
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/input"
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
		Contacts:     []string{"mukai@chromium.org", "oshima@chromium.org", "chromeos-wmp@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Timeout:      15 * time.Minute,
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
	if err := power.TurnOnDisplay(ctx); err != nil {
		s.Fatal("Failed to turn on display: ", err)
	}

	chromeType := s.Param().(lacros.ChromeType)
	isLacros := chromeType == lacros.ChromeTypeLacros
	// TODO(crbug.com/1127165): Remove the artifactPath argument when we can use Data in fixtures.
	var artifactPath string
	if isLacros {
		artifactPath = s.DataPath(launcher.DataArtifact)
	}
	cr, l, cs, err := lacros.Setup(ctx, s.FixtValue(), artifactPath, chromeType)
	if err != nil {
		s.Fatal("Failed to initialize test: ", err)
	}
	defer lacros.CloseLacrosChrome(ctx, l)

	tew, err := input.Touchscreen(ctx)
	if err != nil {
		s.Fatal("Failed to access to the touch screen: ", err)
	}
	defer tew.Close()

	stw, err := tew.NewSingleTouchWriter()
	if err != nil {
		s.Fatal("Failed to create a single touch writer: ", err)
	}
	defer stw.Close()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	originalTabletMode, err := ash.TabletModeEnabled(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to obtain the tablet mode status: ", err)
	}
	defer ash.SetTabletModeEnabled(ctx, tconn, originalTabletMode)

	info, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get the primary display info: ", err)
	}

	tcc := tew.NewTouchCoordConverter(info.Bounds.Size())
	tewW := tew.Width()
	tewH := tew.Height()
	centerX := tewW / 2
	centerY := tewH / 2

	orientation, err := display.GetOrientation(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to obtain the orientation info: ", err)
	}

	var snapX, snapY, dividerEndX, dividerEndY input.TouchCoord
	switch orientation.Type {
	case display.OrientationPortraitPrimary:
		snapX = centerX
		snapY = tewH - 1
		dividerEndX = centerX
		dividerEndY = 0
	case display.OrientationPortraitSecondary:
		snapX = centerX
		snapY = 0
		dividerEndX = centerX
		dividerEndY = tewH - 1
	case display.OrientationLandscapePrimary:
		snapX = 0
		snapY = centerY
		dividerEndX = tewW - 1
		dividerEndY = centerY
	case display.OrientationLandscapeSecondary:
		snapX = tewW - 1
		snapY = centerY
		dividerEndX = 0
		dividerEndY = centerY
	}

	var blankWindowTitle string
	if isLacros {
		blankWindowTitle = "about:blank - Google Chrome"
	} else {
		blankWindowTitle = "Chrome - about:blank"
	}

	isBlank := func(w *ash.Window) bool { return w.Title == blankWindowTitle }

	// Run an http server to serve the test contents for accessing from the chrome browsers.
	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()
	url := server.URL + "/animation.html"

	r := perfutil.NewRunner(cr)
	currentAnimationWindows := 0
	haveBlankWindow := isLacros
	// Run the overview mode enter/exit flow for various situations.
	// - 2 or 8 overview windows
	// - clamshell or tablet
	// - overview windows normal, maximized, or minimized
	// - tablet split view or not
	// - split view window is showing about:blank or animation.html
	for _, overviewWindows := range []int{2, 8} {
		for _, test := range []struct {
			histogramSuffix     string
			extraDescriptionFmt string
			animationWindows    int
			tablet              bool
			overviewWindowState ash.WindowStateType
			splitView           bool
			splitViewUseBlank   bool
		}{
			{
				histogramSuffix:     "SingleClamshellMode",
				extraDescriptionFmt: "%dwindows",
				animationWindows:    overviewWindows,
				tablet:              false,
				overviewWindowState: ash.WindowStateMaximized,
				splitView:           false,
				splitViewUseBlank:   false,
			},
			{
				histogramSuffix:     "ClamshellMode",
				extraDescriptionFmt: "%dwindows",
				animationWindows:    overviewWindows,
				tablet:              false,
				overviewWindowState: ash.WindowStateNormal,
				splitView:           false,
				splitViewUseBlank:   false,
			},
			{
				histogramSuffix:     "TabletMode",
				extraDescriptionFmt: "%dwindows",
				animationWindows:    overviewWindows,
				tablet:              true,
				overviewWindowState: ash.WindowStateMaximized,
				splitView:           false,
				splitViewUseBlank:   false,
			},
			{
				histogramSuffix:     "MinimizedTabletMode",
				extraDescriptionFmt: "%dwindows",
				animationWindows:    overviewWindows,
				tablet:              true,
				overviewWindowState: ash.WindowStateMinimized,
				splitView:           false,
				splitViewUseBlank:   false,
			},
			{
				histogramSuffix:     "SplitView",
				extraDescriptionFmt: "%dmaximizedoverviewwindows-lightsnappedwindow",
				animationWindows:    overviewWindows,
				tablet:              true,
				overviewWindowState: ash.WindowStateMaximized,
				splitView:           true,
				splitViewUseBlank:   true,
			},
			{
				histogramSuffix:     "SplitView",
				extraDescriptionFmt: "%dminimizedoverviewwindows-lightsnappedwindow",
				animationWindows:    overviewWindows,
				tablet:              true,
				overviewWindowState: ash.WindowStateMinimized,
				splitView:           true,
				splitViewUseBlank:   true,
			},
			{
				histogramSuffix:     "SplitView",
				extraDescriptionFmt: "%dmaximizedoverviewwindows-heavysnappedwindow",
				animationWindows:    overviewWindows + 1,
				tablet:              true,
				overviewWindowState: ash.WindowStateMaximized,
				splitView:           true,
				splitViewUseBlank:   false,
			},
			{
				histogramSuffix:     "SplitView",
				extraDescriptionFmt: "%dminimizedoverviewwindows-heavysnappedwindow",
				animationWindows:    overviewWindows + 1,
				tablet:              true,
				overviewWindowState: ash.WindowStateMinimized,
				splitView:           true,
				splitViewUseBlank:   false,
			},
		} {
			if err := ash.CreateWindows(ctx, tconn, cs, url, test.animationWindows-currentAnimationWindows); err != nil {
				s.Fatal("Failed to create browser windows: ", err)
			}
			currentAnimationWindows = test.animationWindows

			if test.splitViewUseBlank {
				if !haveBlankWindow {
					if err := ash.CreateWindows(ctx, tconn, cs, "about:blank", 1); err != nil {
						s.Fatal("Failed to open about:blank: ", err)
					}
					haveBlankWindow = true
				}
			} else if haveBlankWindow {
				w, err := ash.FindWindow(ctx, tconn, isBlank)
				if err != nil {
					s.Fatal("Failed to find window with about:blank: ", err)
				}

				if err := w.CloseWindow(ctx, tconn); err != nil {
					s.Fatal("Failed to close about:blank: ", err)
				}
				haveBlankWindow = false
			}

			if err := ash.SetTabletModeEnabled(ctx, tconn, test.tablet); err != nil {
				s.Fatalf("Failed to set tablet mode to %v: %v", test.tablet, err)
			}

			if err := ash.ForEachWindow(ctx, tconn, func(w *ash.Window) error {
				return ash.SetWindowStateAndWait(ctx, tconn, w.ID, test.overviewWindowState)
			}); err != nil {
				s.Fatalf("Failed to set all windows to state %v: %v", test.overviewWindowState, err)
			}

			if test.splitView {
				if err := ash.SetOverviewModeAndWait(ctx, tconn, true); err != nil {
					s.Fatal("Failed to enter overview: ", err)
				}

				var windowX, windowY input.TouchCoord
				if test.splitViewUseBlank {
					w, err := ash.FindWindow(ctx, tconn, isBlank)
					if err != nil {
						s.Fatal("Failed to find window with about:blank: ", err)
					}

					windowX, windowY = tcc.ConvertLocation(w.OverviewInfo.Bounds.CenterPoint())
					if windowX >= tewW {
						if err := stw.Swipe(ctx, tewW-1, centerY, 0, centerY, time.Second); err != nil {
							s.Fatal("Failed while swiping to scroll the overview grid: ", err)
						}

						if err := stw.End(); err != nil {
							s.Fatal("Failed to end the swipe to scroll the overview grid: ", err)
						}

						w, err := ash.FindWindow(ctx, tconn, isBlank)
						if err != nil {
							s.Fatal("Failed to find window with about:blank: ", err)
						}

						windowX, windowY = tcc.ConvertLocation(w.OverviewInfo.Bounds.CenterPoint())
					}
				} else {
					w, err := ash.FindFirstWindowInOverview(ctx, tconn)
					if err != nil {
						s.Fatal("Failed to find overview window: ", err)
					}

					windowX, windowY = tcc.ConvertLocation(w.OverviewInfo.Bounds.CenterPoint())
				}

				if err := stw.LongPressAt(ctx, windowX, windowY); err != nil {
					s.Fatal("Failed to long-press to initiate window drag from overview: ", err)
				}

				if err := stw.Swipe(ctx, windowX, windowY, snapX, snapY, time.Second); err != nil {
					s.Fatal("Failed while swiping window from overview to snap: ", err)
				}

				if err := stw.End(); err != nil {
					s.Fatal("Failed to end the window swipe from overview to snap: ", err)
				}

				if err := ash.SetOverviewModeAndWait(ctx, tconn, false); err != nil {
					s.Fatal("Failed to exit overview: ", err)
				}
			}

			// Wait for 3 seconds to stabilize the result. Note that this doesn't
			// have to be cpu.WaitUntilIdle(). It may wait too much.
			// TODO(mukai): find the way to wait more properly on the idleness of Ash.
			// https://crbug.com/1001314.
			if err = testing.Sleep(ctx, 3*time.Second); err != nil {
				s.Fatal("Failed to wait: ", err)
			}

			extraDescription := fmt.Sprintf(test.extraDescriptionFmt, overviewWindows)
			r.RunMultiple(ctx, s, fmt.Sprintf("%s-%s", test.histogramSuffix, extraDescription), perfutil.RunAndWaitAll(tconn, func(ctx context.Context) error {
				if err = ash.SetOverviewModeAndWait(ctx, tconn, true); err != nil {
					return errors.Wrap(err, "failed to enter into the overview mode")
				}
				if err = ash.SetOverviewModeAndWait(ctx, tconn, false); err != nil {
					return errors.Wrap(err, "failed to exit from the overview mode")
				}
				return nil
			},
				"Ash.Overview.AnimationSmoothness.Enter"+"."+test.histogramSuffix,
				"Ash.Overview.AnimationSmoothness.Exit"+"."+test.histogramSuffix),
				perfutil.StoreAll(perf.BiggerIsBetter, "percent", extraDescription))

			if test.splitView {
				if err := stw.Swipe(ctx, centerX, centerY, dividerEndX, dividerEndY, time.Second); err != nil {
					s.Fatal("Failed while swiping split view divider to end split view: ", err)
				}

				if err := stw.End(); err != nil {
					s.Fatal("Failed to end the split view divider swipe to end split view: ", err)
				}
			}
		}
	}

	if err := r.Values().Save(ctx, s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}
