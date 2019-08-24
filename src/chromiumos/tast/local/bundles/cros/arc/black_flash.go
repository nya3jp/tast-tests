// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arc/screenshot"
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

	tabletModeEnabled, err := ash.TabletModeEnabled(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get tablet mode: ", err)
	}
	defer ash.SetTabletModeEnabled(ctx, tconn, tabletModeEnabled)

	if err := ash.SetTabletModeEnabled(ctx, tconn, false); err != nil {
		s.Fatal("Failed to set tablet mode enabled to false: ", err)
	}

	dispInfo, err := display.GetInternalInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get internal display info: ", err)
	}

	origShelfBehavior, err := ash.GetShelfBehavior(ctx, tconn, dispInfo.ID)
	if err != nil {
		s.Fatal("Failed to get shelf behavior: ", err)
	}

	if err := ash.SetShelfBehavior(ctx, tconn, dispInfo.ID, ash.ShelfBehaviorAlwaysAutoHide); err != nil {
		s.Fatal("Failed to set shelf behavior to Always Auto Hide: ", err)
	}
	defer ash.SetShelfBehavior(ctx, tconn, dispInfo.ID, origShelfBehavior)

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

	if err := act.WaitForIdle(ctx, 4*time.Second); err != nil {
		s.Fatal("Failed to wait for idle activity: ", err)
	}

	// Set the activity to Maximized, but don't wait for the activity to be idle as we are interested in its transient state.
	if err := act.SetWindowState(ctx, arc.WindowStateMaximized); err != nil {
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

	// Check the screenshot of the activity a few times to see if it shows a black flash or not.
	// We keep taking screenshots for a maximum of 20 seconds until one of the following conditions are met:
	// (i) A black flash appears.
	//     Any black flash shouldn't appear during state transition, so at this point we can conclude this test has failed.
	// (ii) A blue flash appears.
	//     The ArcBlackFlashTest app becomes blue when it transitions from normal to maximized completely.
	//     If we see this blue flash without (1) happenning, this test has passed.
	//
	// The condition (ii) is necessary to tell when the maximized surface has drawn completely.
	// Even if state transition finishes completely and the maximized buffer is ready on the Android side,
	// it doesn't mean the buffer is shown on the Chrome side as transition animation can be still hapenning.
	if err = testing.Poll(ctx, func(ctx context.Context) error {
		bounds, err := act.WindowBounds(ctx)
		if err != nil {
			return err
		}
		// Note if Chrome caption is enabled, the size of a maximized activity can be larger than the display size by the height of the caption.
		if bounds.Width < dispSize.W || bounds.Height < dispSize.H {
			return errors.New("activity is smaller than display yet")
		}

		img, err := screenshot.GrabScreenshot(ctx, cr)
		if err != nil {
			s.Fatal("Failed to grab screenshot: ", err)
		}

		subImage := img.(interface {
			SubImage(r image.Rectangle) image.Image
		}).SubImage(image.Rect(bounds.Left, bounds.Top, bounds.Width, bounds.Height))

		rect := subImage.Bounds()
		totalPixels := (rect.Max.Y - rect.Min.Y) * (rect.Max.X - rect.Min.X)

		blackPixels := screenshot.CountPixels(subImage, color.RGBA{0, 0, 0, 255})
		percent := blackPixels * 100 / totalPixels

		// "10 percent" is arbitrary. It shouldn't have any black pixel.
		if percent > 10 {
			// Save image with black pixels.
			path := filepath.Join(s.OutDir(), "screenshot_fail.png")
			if fd, err := os.Create(path); err != nil {
				s.Error("Failed to create screenshot: ", err)
			} else {
				defer fd.Close()
				if err := png.Encode(fd, subImage); err != nil {
					s.Error("Failed to encode screenshot to png format: ", err)
				}
			}
			s.Fatalf("Test failed. Contains %d / %d (%d%%) black pixels", blackPixels, totalPixels, percent)
		}

		bluePixels := screenshot.CountPixels(subImage, color.RGBA{0, 0, 255, 255})
		percent = bluePixels * 100 / totalPixels

		// When the activity gets maximized, most of the pixels become blue.
		// However, the window can still have nav bar, caption, etc.
		// So, we set the threshold 50% here, but this can be changed roughly between 5% and 80%
		if percent <= 50 {
			return errors.New("new buffer hasn't been shown completely yet")
		}

		return nil
	}, &testing.PollOptions{Timeout: 20 * time.Second}); err != nil {
		s.Fatal("Failed waiting for the activity to be maximized: ", err)
	}
}
