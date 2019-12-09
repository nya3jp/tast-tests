// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"image"
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
		Data:         []string{"ArcQuarterSizedWindowZoomingTest20191206.apk"},
	})
}

func QuarterSizedWindowZooming(ctx context.Context, s *testing.State) {
	const (
		apkName      = "ArcQuarterSizedWindowZoomingTest20191206.apk"
		pkgName      = "org.chromium.arc.testapp.quartersizedwindowzoomingtest"
		activityName = "MainActivity"
	)

	// Reuse existing ARC and Chrome Session
	a := s.PreValue().(arc.PreData).ARC
	cr := s.PreValue().(arc.PreData).Chrome

	// Create test API connection
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	// Enable developer option
	output, err := a.Command(ctx, "getprop", "persist.sys.ui.quarter_window_zooming").Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatal("Failed to get developer option value: ", err)
	}
	s.Log("current developer option is: ", output)

	// Restore tablet mode state
	tabletModeEnabled, err := ash.TabletModeEnabled(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get tablet mode: ", err)
	}
	defer ash.SetTabletModeEnabled(ctx, tconn, tabletModeEnabled)

	// Install application package
	if err := a.Install(ctx, s.DataPath(apkName)); err != nil {
		s.Fatal("Failed to install app: ", err)
	}

	// Create ane Start activity
	act, err := arc.NewActivity(a, pkgName, "."+activityName)
	if err != nil {
		s.Fatal("Failed to create new activity: ", err)
	}
	defer act.Close()

	if err := act.Start(ctx); err != nil {
		s.Fatal("Failed to start the QuarterSizedWindowZooming activity: ", err)
	}

	// Set tablet mode
	if err := ash.SetTabletModeEnabled(ctx, tconn, false); err != nil {
		s.Fatal("Failed to set tablet mode enabled to false: ", err)
	}

	/*
		dispInfo, err := display.GetInternalInfo(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to get internal display info: ", err)
		}

		origShelfBehavior, err := ash.GetShelfBehavior(ctx, tconn, dispInfo.ID)
		if err != nil {
			s.Fatal("Failed to get shelf behavior: ", err)
		}
		defer ash.SetShelfBehavior(ctx, tconn, dispInfo.ID, origShelfBehavior)

		if err := ash.SetShelfBehavior(ctx, tconn, dispInfo.ID, ash.ShelfBehaviorAlwaysAutoHide); err != nil {
			s.Fatal("Failed to set shelf behavior to Always Auto Hide: ", err)
		}
	*/

	// Set the activity to Maximized
	if err := act.SetWindowState(ctx, arc.WindowStateFullscreen); err != nil {
		s.Fatal("Failed to set window state to Maximized: ", err)
	}

	// Enable immersive mode

	if err := act.WaitForResumed(ctx, 4*time.Second); err != nil {
		s.Fatal("Failed to wait for the activity to resume: ", err)
	}

	// Take screenshot: returns image.Image object
	img, err := screenshot.GrabScreenshot(ctx, cr)
	if err != nil {
		s.Fatal("Failed to grab screenshot: ", err)
	}

	bounds, err := act.WindowBounds(ctx)
	if err != nil {
		s.Fatal("Failed to get window bounds: ", err)
	}

	s.Log("window bounds : ", bounds)
	s.Log("img bounds : ", img.Bounds())

	subImage := img.(interface {
		SubImage(r image.Rectangle) image.Image
	}).SubImage(image.Rect(bounds.Left, bounds.Top, bounds.Width, bounds.Height))

	// Check that the screenshot is 2-alternate black and white
	rect := subImage.Bounds()
	height := rect.Max.Y - rect.Min.Y
	width := rect.Max.X - rect.Min.X
	const colorMaxDiff = 30
	black := color.RGBA{0, 0, 0, 255}
	white := color.RGBA{255, 255, 255, 255}

	for i := 0; i < height; i++ {
		for j := 0; j < width; j++ {
			if i < 4 {
				s.Log("sykwer", rect.Min.X+j, rect.Min.Y+i, subImage.At(rect.Min.X+j, rect.Min.Y+i))
			}

			if i%2 == 0 {
				// should be black
				if !colorcmp.ColorsMatch(subImage.At(rect.Min.X+j, rect.Min.Y+i), black, colorMaxDiff) {
					s.Fatal("Feature does not work properly: expect black but: ", rect.Min.X+j, rect.Min.Y+i, img.At(rect.Min.X+j, rect.Min.Y+i))
				}
			} else {
				// should be white
				if !colorcmp.ColorsMatch(subImage.At(rect.Min.X+j, rect.Min.Y+i), white, colorMaxDiff) {
					s.Fatal("Feature does not work properly: expect black but: ", rect.Min.X+j, rect.Min.Y+i, subImage.At(rect.Min.X+j, rect.Min.Y+i))
				}
			}
		}
	}
}
