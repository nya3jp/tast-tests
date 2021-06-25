// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package graphics

import (
	"context"
	"net/http"
	"net/http/httptest"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/local/lacros"
	"chromiumos/tast/local/lacros/launcher"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

const fastInkAPK = "LowLatencyStylusDemoGPU_20210423.apk"

type fastInkTestParams struct {
	arc              bool
	chromeType       lacros.ChromeType
	tablet           bool
	displayRotations []display.RotationAngle
	wStates          []ash.WindowStateType
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         FastInk,
		Desc:         "Verifies that fast ink is working as evidenced by a hardware overlay",
		Contacts:     []string{"amusbach@chromium.org", "oshima@chromium.org", "chromeos-wmp@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.SupportsNV12Overlays(), hwdep.InternalDisplay(), hwdep.TouchScreen()),
		Params: []testing.Param{{
			Name:      "chrome_clamshell",
			ExtraData: []string{"d-canvas/main.html", "d-canvas/2d.js", "d-canvas/webgl.js"},
			Fixture:   "chromeLoggedIn",
			Val: fastInkTestParams{
				arc:        false,
				chromeType: lacros.ChromeTypeChromeOS,
				tablet:     false,
				displayRotations: []display.RotationAngle{
					display.Rotate0,
					display.Rotate90,
					display.Rotate180,
					display.Rotate270,
				},
				wStates: []ash.WindowStateType{
					ash.WindowStateNormal,
					ash.WindowStateMaximized,
					ash.WindowStateFullscreen,
				}},
		}, {
			Name:              "chrome_tablet",
			ExtraSoftwareDeps: []string{"tablet_mode"},
			ExtraData:         []string{"d-canvas/main.html", "d-canvas/2d.js", "d-canvas/webgl.js"},
			Fixture:           "chromeLoggedIn",
			Val: fastInkTestParams{
				arc:        false,
				chromeType: lacros.ChromeTypeChromeOS,
				tablet:     true,
				displayRotations: []display.RotationAngle{
					display.Rotate0,
					display.Rotate90,
					display.Rotate180,
					display.Rotate270,
				},
				wStates: []ash.WindowStateType{
					ash.WindowStateMaximized,
					ash.WindowStateFullscreen,
				}},
		}, {
			Name:              "chrome_clamshell_lacros",
			ExtraSoftwareDeps: []string{"lacros"},
			ExtraData:         []string{"d-canvas/main.html", "d-canvas/2d.js", "d-canvas/webgl.js", launcher.DataArtifact},
			Fixture:           "lacrosStartedByData",
			Val: fastInkTestParams{
				arc:        false,
				chromeType: lacros.ChromeTypeLacros,
				tablet:     false,
				displayRotations: []display.RotationAngle{
					display.Rotate0,
					display.Rotate90,
					display.Rotate180,
					display.Rotate270,
				},
				wStates: []ash.WindowStateType{
					ash.WindowStateNormal,
					ash.WindowStateMaximized,
					ash.WindowStateFullscreen,
				}},
		}, {
			Name:              "chrome_tablet_lacros",
			ExtraSoftwareDeps: []string{"lacros", "tablet_mode"},
			ExtraData:         []string{"d-canvas/main.html", "d-canvas/2d.js", "d-canvas/webgl.js", launcher.DataArtifact},
			Fixture:           "lacrosStartedByData",
			Val: fastInkTestParams{
				arc:        false,
				chromeType: lacros.ChromeTypeLacros,
				tablet:     true,
				displayRotations: []display.RotationAngle{
					display.Rotate0,
					display.Rotate90,
					display.Rotate180,
					display.Rotate270,
				},
				wStates: []ash.WindowStateType{
					ash.WindowStateMaximized,
					ash.WindowStateFullscreen,
				}},
		}, {
			Name:              "arc_clamshell",
			ExtraSoftwareDeps: []string{"android_p"},
			ExtraData:         []string{fastInkAPK},
			Fixture:           "arcBooted",
			Val: fastInkTestParams{
				arc:        true,
				chromeType: lacros.ChromeTypeChromeOS, // not applicable
				tablet:     false,
				displayRotations: []display.RotationAngle{
					display.Rotate0,
					display.Rotate180,
				},
				wStates: []ash.WindowStateType{
					ash.WindowStateNormal,
					ash.WindowStateLeftSnapped,
					ash.WindowStateRightSnapped,
					ash.WindowStateMaximized,
					ash.WindowStateFullscreen,
				}},
		}, {
			Name:              "arc_tablet",
			ExtraSoftwareDeps: []string{"android_p", "tablet_mode"},
			ExtraData:         []string{fastInkAPK},
			Fixture:           "arcBootedInTabletMode",
			Val: fastInkTestParams{
				arc:        true,
				chromeType: lacros.ChromeTypeChromeOS, // not applicable
				tablet:     true,
				displayRotations: []display.RotationAngle{
					display.Rotate0,
					display.Rotate180,
				},
				wStates: []ash.WindowStateType{
					ash.WindowStateMaximized,
					ash.WindowStateLeftSnapped,
					ash.WindowStateRightSnapped,
					ash.WindowStateFullscreen,
				}},
		}, {
			Name:              "arc_clamshell_vm",
			ExtraSoftwareDeps: []string{"android_vm"},
			ExtraData:         []string{fastInkAPK},
			Fixture:           "arcBooted",
			Val: fastInkTestParams{
				arc:        true,
				chromeType: lacros.ChromeTypeChromeOS, // not applicable
				tablet:     false,
				displayRotations: []display.RotationAngle{
					display.Rotate0,
					display.Rotate180,
				},
				wStates: []ash.WindowStateType{
					ash.WindowStateNormal,
					ash.WindowStateLeftSnapped,
					ash.WindowStateRightSnapped,
					ash.WindowStateMaximized,
					ash.WindowStateFullscreen,
				}},
		}, {
			Name:              "arc_tablet_vm",
			ExtraSoftwareDeps: []string{"android_vm", "tablet_mode"},
			ExtraData:         []string{fastInkAPK},
			Fixture:           "arcBootedInTabletMode",
			Val: fastInkTestParams{
				arc:        true,
				chromeType: lacros.ChromeTypeChromeOS, // not applicable
				tablet:     true,
				displayRotations: []display.RotationAngle{
					display.Rotate0,
					display.Rotate180,
				},
				wStates: []ash.WindowStateType{
					ash.WindowStateMaximized,
					ash.WindowStateLeftSnapped,
					ash.WindowStateRightSnapped,
					ash.WindowStateFullscreen,
				}},
		}},
	})
}

func FastInk(ctx context.Context, s *testing.State) {
	// Reserve ten seconds for various cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	params := s.Param().(fastInkTestParams)
	var cr *chrome.Chrome
	var l *launcher.LacrosChrome
	if params.arc {
		cr = s.FixtValue().(*arc.PreData).Chrome
	} else {
		// TODO(crbug.com/1127165): Remove the artifactPath argument when we can use Data in fixtures.
		var artifactPath string
		if params.chromeType == lacros.ChromeTypeLacros {
			artifactPath = s.DataPath(launcher.DataArtifact)
		}
		var err error
		cr, l, _, err = lacros.Setup(ctx, s.FixtValue(), artifactPath, params.chromeType)
		if err != nil {
			s.Fatal("Failed to initialize test: ", err)
		}
		defer lacros.CloseLacrosChrome(cleanupCtx, l)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, params.tablet)
	if err != nil {
		s.Fatal("Failed to ensure clamshell/tablet mode: ", err)
	}
	defer cleanup(cleanupCtx)

	internalInfo, err := display.GetInternalInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get the internal display info: ", err)
	}

	if params.arc {
		a := s.FixtValue().(*arc.PreData).ARC
		if err := a.Install(ctx, s.DataPath(fastInkAPK)); err != nil {
			s.Fatal("Failed installing app: ", err)
		}

		act, err := arc.NewActivity(a, "dev.chromeos.lowlatencystylusdemogpu", ".MainActivity")
		if err != nil {
			s.Fatal("Failed to create new activity: ", err)
		}
		defer act.Close()

		if err := act.Start(ctx, tconn); err != nil {
			s.Fatal("Failed to start activity: ", err)
		}
		defer act.Stop(cleanupCtx, tconn)
	} else {
		if !params.tablet {
			// Move the mouse to the center of the work area. Otherwise,
			// on some devices, when the window is fullscreen, the mouse
			// position will make the shelf visible, causing test failure.
			if err := mouse.Move(tconn, internalInfo.WorkArea.CenterPoint(), 0)(ctx); err != nil {
				s.Fatal("Failed to move mouse: ", err)
			}
		}

		srv := httptest.NewServer(http.FileServer(s.DataFileSystem()))
		defer srv.Close()

		conn, err := cr.NewConn(ctx, srv.URL+"/d-canvas/main.html")
		if err != nil {
			s.Fatal("Failed to load d-canvas/main.html: ", err)
		}
		defer conn.Close()

		if params.chromeType == lacros.ChromeTypeLacros {
			if err := lacros.CloseAboutBlank(ctx, tconn, l.Devsess, 1); err != nil {
				s.Fatal("Failed to close about:blank: ", err)
			}
		}

		if err := webutil.WaitForQuiescence(ctx, conn, 10*time.Second); err != nil {
			s.Fatal("Failed to wait for d-canvas/main.html to achieve quiescence: ", err)
		}
	}

	ws, err := ash.GetAllWindows(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get windows: ", err)
	}

	// Verify that there is one and only one window.
	if wsCount := len(ws); wsCount != 1 {
		s.Fatal("Expected 1 window; found ", wsCount)
	}

	internalDisplayID := internalInfo.ID
	wID := ws[0].ID
	defer display.SetDisplayRotationSync(cleanupCtx, tconn, internalDisplayID, display.Rotate0)
	for _, displayRotation := range params.displayRotations {
		s.Run(ctx, string(displayRotation), func(ctx context.Context, s *testing.State) {
			if err := display.SetDisplayRotationSync(ctx, tconn, internalDisplayID, displayRotation); err != nil {
				s.Fatal("Failed to rotate display: ", err)
			}

			for _, wState := range params.wStates {
				s.Run(ctx, string(wState), func(ctx context.Context, s *testing.State) {
					if err := ash.SetWindowStateAndWait(ctx, tconn, wID, wState); err != nil {
						s.Fatalf("Failed to set window state to %v: %v", wState, err)
					}

					if wState == ash.WindowStateNormal {
						internalInfo, err := display.GetInternalInfo(ctx, tconn)
						if err != nil {
							s.Fatal("Failed to get the internal display info: ", err)
						}

						// Make the window reasonably large, with no part behind the shelf.
						desiredBounds := internalInfo.WorkArea.WithInset(10, 10)
						bounds, displayID, err := ash.SetWindowBounds(ctx, tconn, wID, desiredBounds, internalInfo.ID)
						if err != nil {
							s.Fatal("Failed to set the window bounds: ", err)
						}
						if displayID != internalInfo.ID {
							info, err := display.FindInfo(ctx, tconn, func(info *display.Info) bool { return info.ID == displayID })
							if err != nil {
								s.Fatal("Window not on internal display after setting bounds (also failed to get info on the display where it is)")
							}
							s.Fatalf("Window on %s; tried to put it on %s", info.Name, internalInfo.Name)
						}
						if bounds != desiredBounds {
							s.Fatalf("Window bounds are %v; tried to set them to %v", bounds, desiredBounds)
						}
					}

					if err := testing.Sleep(ctx, time.Second); err != nil {
						s.Fatal("Failed to wait a second: ", err)
					}

					hists, err := metrics.Run(ctx, tconn, func(ctx context.Context) error {
						if err := testing.Sleep(ctx, time.Second); err != nil {
							return errors.Wrap(err, "failed to wait a second")
						}
						return nil
					}, "Viz.DisplayCompositor.OverlayStrategy")
					if err != nil {
						s.Fatal("Error while recording histogram Viz.DisplayCompositor.OverlayStrategy: ", err)
					}

					hist := hists[0]
					if len(hist.Buckets) == 0 {
						s.Fatal("Got no overlay strategy data")
					}

					for _, bucket := range hist.Buckets {
						// bucket.Min will be from enum OverlayStrategies as defined
						// in tools/metrics/histograms/enums.xml in the chromium
						// code base. Here we check for 1 which is "No overlay."
						if bucket.Min == 1 {
							s.Fatalf("%d of %d frame(s) had no overlay", bucket.Count, hist.TotalCount())
						}
					}
				})
			}
		})
	}
}
