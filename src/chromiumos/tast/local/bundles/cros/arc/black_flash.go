// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         BlackFlash,
		Desc:         "Checks that Black flashes don't appear when ARC applications change window states",
		Contacts:     []string{"takise@chromium.org", "arc-framework+tast@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android_p", "chrome"},
		Pre:          arc.Booted(),
		Timeout:      5 * time.Minute,
		Data:         []string{"ArcBlackFlashTest.apk"},
	})
}

func BlackFlash(ctx context.Context, s *testing.State) {
	const (
		apkName      = "ArcBlackFlashTest.apk"
		pkgName      = "org.chromium.arc.testapp.arcblackflashtest"
		activityName = "MainActivity"
	)

	a := s.PreValue().(arc.PreData).ARC
	cr := s.PreValue().(arc.PreData).Chrome
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	// Restore tablet mode to its original state on exit.
	tabletModeEnabled, err := ash.TabletModeEnabled(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get tablet mode: ", err)
	}
	defer ash.SetTabletModeEnabled(ctx, tconn, tabletModeEnabled)

	if tabletModeEnabled == false {
		if err := ash.SetTabletModeEnabled(ctx, tconn, false); err != nil {
			s.Fatal("Failed to set tablet mode enabled to false: ", err)
		}
	}

	dispInfo, err := display.GetInternalInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get internal display info: ", err)
	}

	origShelfBehavior, err := ash.GetShelfBehavior(ctx, tconn, dispInfo.ID)
	if err != nil {
		s.Fatal("Failed to get shelf behavior: ", err)
	}

	// Hide shelf as we want compare the display size with the window size.
	if err := ash.SetShelfBehavior(ctx, tconn, dispInfo.ID, ash.ShelfBehaviorAlwaysAutoHide); err != nil {
		s.Fatal("Failed to set shelf behavior to Always Auto Hide: ", err)
	}
	defer ash.SetShelfBehavior(ctx, tconn, dispInfo.ID, origShelfBehavior)

	s.Log("Installing ", apkName)
	if err := a.Install(ctx, s.DataPath(apkName)); err != nil {
		s.Fatal("Failed installing app: ", err)
	}

	act, err := arc.NewActivity(a, pkgName, "."+activityName)
	if err != nil {
		s.Fatal("Failed to create new activity: ", err)
	}
	defer act.Close()

	if err := act.Start(ctx); err != nil {
		s.Fatal("Failed to start the BlackFlashTest activity: ", err)
	}

	// Set the activity to Restored.
	if err := act.SetWindowState(ctx, arc.WindowStateNormal); err != nil {
		s.Fatal("Failed to set the activity to Normal: ", err)
	}

	if err := act.SetWindowStateAsync(ctx, arc.WindowStateMaximized); err != nil {
		s.Fatal("Failed to set the activity to Maximized: ", err)
	}

	disp, err := arc.NewDisplay(a, arc.DefaultDisplayID)
	if err != nil {
		s.Fatal("Failed to obtain a default display: ", err)
	}

	dispSize, err := disp.Size(ctx)
	if err != nil {
		s.Fatal("Failed to get display bounds")
	}

	if err = testing.Poll(ctx, func(ctx context.Context) error {
		bounds, err := act.WindowBounds(ctx)
		if err != nil {
			return err
		}
		// Note if Chrome caption is enabled, the size of a maximized activity can be larger than the display size by the height of the caption.
		if bounds.Width < dispSize.W || bounds.Height < dispSize.H {
			return errors.New("activity is smaller than display yet")
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		s.Fatal("failed waiting for size of activity to be same as display: ", err)
	}

	img, err := GrabScreenshot(ctx, cr, fmt.Sprintf("%s/screenshot.png", s.OutDir()))
	if err != nil {
		s.Fatal("Failed to grab screenshot: ", err)
	}

	bounds, err := act.WindowBounds(ctx)
	if err != nil {
		s.Fatal("Failed to get activity bounds: ", err)
	}

	subImage := img.(interface {
		SubImage(r image.Rectangle) image.Image
	}).SubImage(image.Rect(bounds.Left, bounds.Top, bounds.Width, bounds.Height))

	blackPixels := CountBlackPixels(subImage)
	rect := subImage.Bounds()
	totalPixels := (rect.Max.Y - rect.Min.Y) * (rect.Max.X - rect.Min.X)
	percent := blackPixels * 100 / totalPixels
	s.Logf("Black pixels = %d / %d (%d%%)", blackPixels, totalPixels, percent)

	// "10 percent" is arbitrary. It shouldn't have any black pixel.
	if percent > 10 {
		// Save image with black pixels.
		path := filepath.Join(s.OutDir(), "screenshot_fail.png")
		fd, err := os.Create(path)
		if err != nil {
			s.Fatal("Failed to create screenshot: ", err)
		}
		defer fd.Close()
		png.Encode(fd, subImage)
		s.Logf("Image containing the black pixels: %s", path)

		s.Fatalf("Test failed. Contains %d / %d (%d%%) black pixels", blackPixels, totalPixels, percent)
	}
}
