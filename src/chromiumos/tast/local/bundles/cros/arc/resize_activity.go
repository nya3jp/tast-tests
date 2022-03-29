// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"image"
	"image/color"
	"path/filepath"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/media/imgcmp"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ResizeActivity,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that resizing ARC applications works without generating black background",
		Contacts:     []string{"ruanc@chromium.org", "arc-framework+tast@google.com"},
		// TODO(http://b/166827552): Test is disabled until it can be fixed
		// Attr:     []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Fixture:      "arcBooted",
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
	})
}

func ResizeActivity(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*arc.PreData).Chrome

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	dispInfo, err := display.GetInternalInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get internal display info: ", err)
	}

	origShelfBehavior, err := ash.GetShelfBehavior(ctx, tconn, dispInfo.ID)
	if err != nil {
		s.Fatal("Failed to get shelf behavior: ", err)
	}

	// Hide shelf. Maximum screen real-estate is needed, especially for devices where its height is as high
	// as the default height of freeform applications.
	if err := ash.SetShelfBehavior(ctx, tconn, dispInfo.ID, ash.ShelfBehaviorAlwaysAutoHide); err != nil {
		s.Fatal("Failed to set shelf behavior to Always Auto Hide: ", err)
	}
	// Be nice and restore shelf behavior to its original state on exit.
	defer ash.SetShelfBehavior(ctx, tconn, dispInfo.ID, origShelfBehavior)

	tabletModeEnabled, err := ash.TabletModeEnabled(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get tablet mode: ", err)
	}
	if tabletModeEnabled {
		// Be nice and restore tablet mode to its original state on exit.
		defer ash.SetTabletModeEnabled(ctx, tconn, tabletModeEnabled)
		if err := ash.SetTabletModeEnabled(ctx, tconn, false); err != nil {
			s.Fatal("Failed to set tablet mode disabled: ", err)
		}
		// TODO(crbug.com/1002958): Wait for "tablet mode animation is finished" in a reliable way.
		// If an activity is launched while the tablet mode animation is active, the activity
		// will be launched in un undefined state, making the test flaky.
		if err := testing.Sleep(ctx, 5*time.Second); err != nil {
			s.Fatal("Failed to wait until tablet-mode animation finished: ", err)
		}
	}

	a := s.FixtValue().(*arc.PreData).ARC

	act, err := arc.NewActivity(a, "com.android.settings", ".Settings")
	if err != nil {
		s.Fatal("Failed to create new activity: ", err)
	}
	defer act.Close()

	if err := act.StartWithDefaultOptions(ctx, tconn); err != nil {
		s.Fatal("Failed start Settings activity: ", err)
	}
	// This is an issue to re-enable the tablet mode at the end of the test when
	// there is a freeform app still open. See: https://crbug.com/1002666
	defer act.Stop(ctx, tconn)

	if err := ash.WaitForVisible(ctx, tconn, act.PackageName()); err != nil {
		s.Fatal("Failed to wait for Setting activity visible: ", err)
	}

	if err := act.SetWindowState(ctx, tconn, arc.WindowStateNormal); err != nil {
		s.Fatal("Failed to set window state to Normal: ", err)
	}

	if err := ash.WaitForARCAppWindowState(ctx, tconn, act.PackageName(), ash.WindowStateNormal); err != nil {
		s.Fatal("Failed to wait for window state: ", err)
	}

	bounds, err := act.WindowBounds(ctx)
	if err != nil {
		s.Fatal("Failed to get activity bounds: ", err)
	}

	disp, err := arc.NewDisplay(a, arc.DefaultDisplayID)
	if err != nil {
		s.Fatal("Failed to obtain a default display: ", err)
	}

	dispSize, err := disp.Size(ctx)
	if err != nil {
		s.Fatal("Failed to get display bounds")
	}

	// Make it as small as possible before the resizing, since maximum screen real-estate is needed for the test.
	// And then place it on the left-top corner.
	// Resizing from TopLeft corner, since BottomRight corner might trigger the shelf, even if it is hidden.
	if err := act.ResizeWindow(ctx, tconn, arc.BorderTopLeft, coords.NewPoint(bounds.Left+bounds.Width, bounds.Top+bounds.Height), 300*time.Millisecond); err != nil {
		s.Fatal("Failed to resize window: ", err)
	}

	// b/150731172: swipe gesture goes out of the screen bounds. Wait until window gets stable after resizing.
	// TODO(crbug.com/1062920): Wait for bounds here.
	if err := testing.Sleep(ctx, 500*time.Millisecond); err != nil {
		s.Fatal("Failed to sleep: ", err)
	}

	bounds, err = act.WindowBounds(ctx)
	if err != nil {
		s.Fatal("Failed to get activity bounds after initial resizing: ", err)
	}

	// When the window move too close to the top-left corner of the display, it will be maximized.
	// So set up margin for the top-left cornner to prevent the window switching to maximized state.
	const marginForDispTopLeft = 50
	destBounds := coords.NewRect(marginForDispTopLeft, marginForDispTopLeft, bounds.Width, bounds.Height)
	dispTopLeft := coords.NewPoint(marginForDispTopLeft, marginForDispTopLeft)
	// Moving the window slowly (in one second) to prevent triggering any kind of gesture like "snap to border", or "maximize".
	if err := act.MoveWindow(ctx, tconn, time.Second, destBounds, bounds); err != nil {
		s.Fatal("Failed to move window: ", err)
	}

	// TODO(crbug.com/1062920): Wait for bounds here.
	if err := testing.Sleep(ctx, 500*time.Millisecond); err != nil {
		s.Fatal("Failed to sleep: ", err)
	}

	restoreBounds, err := act.WindowBounds(ctx)
	if err != nil {
		s.Fatal("Failed to get window bounds: ", err)
	}

	// Make sure the window is located at the top-left corner.
	curTopLeft := coords.NewPoint(restoreBounds.Left, restoreBounds.Top)
	// There are some pixels off due to the calculation.
	const pixelErrorMargin = 2
	if curTopLeft.X < dispTopLeft.X-pixelErrorMargin || curTopLeft.X > dispTopLeft.X+pixelErrorMargin || curTopLeft.Y < dispTopLeft.Y-pixelErrorMargin || curTopLeft.Y > dispTopLeft.Y+pixelErrorMargin {
		s.Fatalf("Unexpected window position after moving window: got %+v; want %+v +/- (2, 2)", curTopLeft, dispTopLeft)
	}

	// Perform 3 different subtests: resize from right border, from bottom border and from bottom-right border.
	// If one of these subtests fail, the test fails and the remaining subtests are not executed.
	// The bug is not 100% reproducible. It might happen that the test pass even if the bug is not fixed.

	// Resize should be as big as possible in order to have higher changes to trigger the bug.
	// But we should leave some margin to resize it back to its original size. That means the
	// window should not overlap the shelf; and we should leave some extra room to place the touches.

	// Leaving room for the touch + extra space to prevent any kind of "resize to fullscreen" gesture.
	const marginForTouch = 100
	for _, entry := range []struct {
		desc     string
		border   arc.BorderType // resize origin (from which border)
		dst      coords.Point
		duration time.Duration
	}{
		{"right", arc.BorderRight, coords.NewPoint(dispSize.Width-marginForTouch-restoreBounds.Left, restoreBounds.Top+restoreBounds.Height/2), 100 * time.Millisecond},
		{"bottom", arc.BorderBottom, coords.NewPoint(restoreBounds.Left+restoreBounds.Width/2, dispSize.Height-marginForTouch-restoreBounds.Top), 300 * time.Millisecond},
		{"bottom-right", arc.BorderBottomRight, coords.NewPoint(dispSize.Width-marginForTouch-restoreBounds.Left, dispSize.Height-marginForTouch-restoreBounds.Top), 100 * time.Millisecond},
	} {
		s.Logf("Resizing window from %s border to %+v", entry.desc, entry.dst)
		if err := act.ResizeWindow(ctx, tconn, entry.border, entry.dst, entry.duration); err != nil {
			s.Fatal("Failed to resize activity: ", err)
		}

		// Not waiting on purpose. We have to grab the screenshot as soon as ResizeWindow() returns.

		img, err := screenshot.GrabScreenshot(ctx, cr)
		if err != nil {
			s.Fatal("Failed to grab screenshot: ", err)
		}

		bounds, err = act.WindowBounds(ctx)
		if err != nil {
			s.Fatal("Failed to get activity bounds: ", err)
		}

		subImage := img.(interface {
			SubImage(r image.Rectangle) image.Image
		}).SubImage(image.Rect(bounds.Left, bounds.Top, bounds.Width, bounds.Height))

		blackPixels := imgcmp.CountPixels(subImage, color.RGBA{0, 0, 0, 255})
		rect := subImage.Bounds()
		totalPixels := (rect.Max.Y - rect.Min.Y) * (rect.Max.X - rect.Min.X)
		percent := blackPixels * 100 / totalPixels
		s.Logf("Black pixels = %d / %d (%d%%)", blackPixels, totalPixels, percent)

		// "3 percent" is arbitrary. It shouldn't have any black pixel. But in case
		// the Settings app changes its default theme, we use 3% as a margin.
		if percent > 3 {
			// Save image with black pixels.
			path := filepath.Join(s.OutDir(), "screenshot_fail.png")
			if err := imgcmp.DumpImageToPNG(ctx, &subImage, path); err != nil {
				s.Fatal("Failed to create screenshot: ", err)
			}
			s.Logf("Image containing the black pixels: %s", path)

			s.Fatalf("Test failed. Contains %d / %d (%d%%) black pixels", blackPixels, totalPixels, percent)
		}

		// Restore the activity bounds.
		if err := act.ResizeWindow(ctx, tconn, entry.border, coords.NewPoint(restoreBounds.Left, restoreBounds.Top), 500*time.Millisecond); err != nil {
			s.Fatal("Failed to resize activity: ", err)
		}

		// TODO(crbug.com/1062920): Wait for bounds here.
		if err := testing.Sleep(ctx, 500*time.Millisecond); err != nil {
			s.Fatal("Failed to sleep: ", err)
		}
	}
}
