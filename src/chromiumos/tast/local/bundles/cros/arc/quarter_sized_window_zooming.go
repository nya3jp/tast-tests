// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"image/color"
	"path/filepath"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arc/screenshot"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/colorcmp"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     QuarterSizedWindowZooming,
		Desc:     "Check quarter-sized window zooming feature is working properly",
		Contacts: []string{"cuicuiruan@google.com", "ricardoq@google.com", "arc-framework+tast@google.com"},
		// Disable test until it can be fixed: https://crbug.com/1038163
		// Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"android_p", "chrome"},
		Fixture:      "arcBooted",
	})
}

func QuarterSizedWindowZooming(ctx context.Context, s *testing.State) {
	const (
		apkName            = "ArcQuarterSizedWindowZoomingTest.apk"
		pkgName            = "org.chromium.arc.testapp.quartersizedwindowzoomingtest"
		activityName       = "MainActivity"
		quarterSizeSetting = "persist.sys.ui.quarter_window_zooming"
	)

	a := s.FixtValue().(*arc.PreData).ARC
	cr := s.FixtValue().(*arc.PreData).Chrome

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	if err := arc.BootstrapCommand(ctx, "/system/bin/setprop", quarterSizeSetting, "allowlist").Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to set developer option: ", err)
	}
	defer arc.BootstrapCommand(ctx, "/system/bin/setprop", quarterSizeSetting, "default").Run(testexec.DumpLogOnError)

	tabletModeEnabled, err := ash.TabletModeEnabled(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get tablet mode: ", err)
	}
	if tabletModeEnabled {
		// Wait for set device to clamshell mode.
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			if err := ash.SetTabletModeEnabled(ctx, tconn, false); err != nil {
				return err
			}
			tabletModeEnabled, err := ash.TabletModeEnabled(ctx, tconn)
			if err != nil {
				return err
			}
			if tabletModeEnabled {
				return errors.New("failed to set device to clamshell mode")
			}
			return nil
		}, nil); err != nil {
			s.Fatal("Failed to set device to clamshell mode: ", err)
		}

		defer ash.SetTabletModeEnabled(ctx, tconn, tabletModeEnabled)

		// TODO(ricardoq): Wait for "tablet mode animation is finished" in a reliable way.
		// If an activity is launched while the tablet mode animation is active, the activity
		// will be launched in an undefined state, making the test flaky.
		if err := testing.Sleep(ctx, 5*time.Second); err != nil {
			s.Fatal("Failed to wait until tablet-mode animation finished: ", err)
		}
	}

	if err := a.Install(ctx, arc.APKPath(apkName)); err != nil {
		s.Fatal("Failed to install app: ", err)
	}

	act, err := arc.NewActivity(a, pkgName, "."+activityName)
	if err != nil {
		s.Fatal("Failed to create new activity: ", err)
	}
	defer act.Close()

	if err := act.Start(ctx, tconn); err != nil {
		s.Fatal("Failed to start the QuarterSizedWindowZooming activity: ", err)
	}
	defer act.Stop(ctx, tconn)

	if err := ash.WaitForVisible(ctx, tconn, act.PackageName()); err != nil {
		s.Fatal("Failed to wait for QuarterSizedWindowZooming activity visible: ", err)
	}

	if err := act.SetWindowState(ctx, tconn, arc.WindowStateFullscreen); err != nil {
		s.Fatal("Failed to set window state to Fullscreen: ", err)
	}

	if err := ash.WaitForARCAppWindowState(ctx, tconn, act.PackageName(), ash.WindowStateFullscreen); err != nil {
		s.Fatal("Failed to wait for activity to enter Fullscreen state: ", err)
	}

	// Wait for window finishing animating before taking screenshot,
	// or the line color will be off as expected.
	info, err := ash.GetARCAppWindowInfo(ctx, tconn, act.PackageName())
	if err := ash.WaitWindowFinishAnimating(ctx, tconn, info.ID); err != nil {
		s.Fatal("Failed to wait for top window animation: ", err)
	}

	// TODO(ruanc): Waiting for one second before taking screenshot.
	// The drawing is more stable after one second. Not sure about the root cause yet. (https://crbug.com/1123620)
	if err := testing.Sleep(ctx, time.Second); err != nil {
		s.Fatal("Failed to wait until tablet-mode animation finished: ", err)
	}

	img, err := screenshot.GrabScreenshot(ctx, cr)
	if err != nil {
		s.Fatal("Failed to grab screenshot: ", err)
	}

	rect := img.Bounds()
	height := rect.Max.Y - rect.Min.Y
	width := rect.Max.X - rect.Min.X

	// Ideally, we expect the pixels are painted in complete black or white,
	// but the chrome side renders pixels in not-complete black or white (gray).
	// Therefore, we check that each line in pixels are painted in gray which is
	// close to the expected color (black or white).
	const colorMaxDiff = 128

	black := color.RGBA{0, 0, 0, 255}
	white := color.RGBA{255, 255, 255, 255}

	// In the test app, we paint each row in display pixels black and white alternately.
	// When the feature is enabled, the window is halved to the quarter size and the
	// surface is zoomed in the chrome size, which results in alternate color changes
	// per two rows in pixels.
	for i := 0; i < height; i++ {
		for j := 0; j < width; j++ {
			if i%4 == 0 || i%4 == 1 {
				// Should be black.
				if !colorcmp.ColorsMatch(img.At(rect.Min.X+j, rect.Min.Y+i), black, colorMaxDiff) {
					path := filepath.Join(s.OutDir(), "screenshot_fail.png")
					if err := screenshot.DumpImageToPNG(ctx, &img, path); err != nil {
						s.Fatal("Failed to create screenshot: ", err)
					}
					s.Logf("Screenshot image for failed test created in: %s", path)
					s.Fatal("Feature does not work properly: expect black but: ", rect.Min.X+j, rect.Min.Y+i, img.At(rect.Min.X+j, rect.Min.Y+i))
				}
			} else {
				// Should be white.
				if !colorcmp.ColorsMatch(img.At(rect.Min.X+j, rect.Min.Y+i), white, colorMaxDiff) {
					path := filepath.Join(s.OutDir(), "screenshot_fail.png")
					if err := screenshot.DumpImageToPNG(ctx, &img, path); err != nil {
						s.Fatal("Failed to create screenshot: ", err)
					}
					s.Logf("Screenshot image for failed test created in: %s", path)
					s.Fatal("Feature does not work properly: expect white but: ", rect.Min.X+j, rect.Min.Y+i, img.At(rect.Min.X+j, rect.Min.Y+i))
				}
			}
		}
	}
}
