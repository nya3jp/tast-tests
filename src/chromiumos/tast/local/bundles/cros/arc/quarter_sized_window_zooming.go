// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"image/color"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arc/screenshot"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/colorcmp"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         QuarterSizedWindowZooming,
		Desc:         "Check quarter-sized window zooming feature is working properly",
		Contacts:     []string{"taishiakwa@google.com", "arc-framework+tast@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"android_p", "chrome"},
		Pre:          arc.Booted(),
		Data:         []string{"ArcQuarterSizedWindowZoomingTest.apk"},
	})
}

func QuarterSizedWindowZooming(ctx context.Context, s *testing.State) {
	const (
		apkName      = "ArcQuarterSizedWindowZoomingTest.apk"
		pkgName      = "org.chromium.arc.testapp.quartersizedwindowzoomingtest"
		activityName = "MainActivity"
	)

	a := s.PreValue().(arc.PreData).ARC
	cr := s.PreValue().(arc.PreData).Chrome

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	if err := a.Command(ctx, "setprop", "persist.sys.ui.quarter_window_zooming", "whitelist").Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to set developer option: ", err)
	}
	defer a.Command(ctx, "setprop", "persist.sys.ui.quarter_window_zooming", "default").Run(testexec.DumpLogOnError)

	tabletModeEnabled, err := ash.TabletModeEnabled(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get tablet mode: ", err)
	}
	defer ash.SetTabletModeEnabled(ctx, tconn, tabletModeEnabled)

	if err := a.Install(ctx, s.DataPath(apkName)); err != nil {
		s.Fatal("Failed to install app: ", err)
	}

	act, err := arc.NewActivity(a, pkgName, "."+activityName)
	if err != nil {
		s.Fatal("Failed to create new activity: ", err)
	}
	defer act.Close()

	if err := act.Start(ctx); err != nil {
		s.Fatal("Failed to start the QuarterSizedWindowZooming activity: ", err)
	}

	if err := ash.SetTabletModeEnabled(ctx, tconn, false); err != nil {
		s.Fatal("Failed to set tablet mode enabled to false: ", err)
	}

	if err := act.SetWindowState(ctx, arc.WindowStateFullscreen); err != nil {
		s.Fatal("Failed to set window state to Maximized: ", err)
	}

	if err := act.WaitForResumed(ctx, 4*time.Second); err != nil {
		s.Fatal("Failed to wait for the activity to resume: ", err)
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
				// should be black
				if !colorcmp.ColorsMatch(img.At(rect.Min.X+j, rect.Min.Y+i), black, colorMaxDiff) {
					s.Fatal("Feature does not work properly: expect black but: ", rect.Min.X+j, rect.Min.Y+i, img.At(rect.Min.X+j, rect.Min.Y+i))
				}
			} else {
				// should be white
				if !colorcmp.ColorsMatch(img.At(rect.Min.X+j, rect.Min.Y+i), white, colorMaxDiff) {
					s.Fatal("Feature does not work properly: expect white but: ", rect.Min.X+j, rect.Min.Y+i, img.At(rect.Min.X+j, rect.Min.Y+i))
				}
			}
		}
	}
}
