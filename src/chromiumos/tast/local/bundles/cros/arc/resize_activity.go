// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/colorcmp"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     ResizeActivity,
		Desc:     "Checks that resizing ARC applications works without generating black background",
		Contacts: []string{"ricardoq@chromium.org", "arc-eng@google.com"},
		Attr:     []string{"informational"},
		// Add tablet_mode to support screen touch.
		SoftwareDeps: []string{"android", "android_p", "chrome_login", "tablet_mode"},
		Timeout:      4 * time.Minute,
	})
}

func ResizeActivity(ctx context.Context, s *testing.State) {
	// Force Chrome to be in clamshell mode, where windows are resizable.
	cr, err := chrome.New(ctx, chrome.ARCEnabled(), chrome.ExtraArgs("--force-tablet-mode=clamshell"))
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close()

	act, err := arc.NewActivity(a, "com.android.settings", ".Settings")
	if err != nil {
		s.Fatal("Failed to create new activity: ", err)
	}
	defer act.Close()

	if err := act.Start(ctx); err != nil {
		s.Fatal("Failed start Settings activity: ", err)
	}

	if err := act.SetWindowState(ctx, arc.WindowStateNormal); err != nil {
		s.Fatal("Failed to set window state to Normal: ", err)
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

	// Make it as small as possible before the resizing, and place it on the left-top corner
	// in order to have maximum real-estate for the resizing.
	if err := act.ResizeWindow(ctx, arc.BorderBottomRight, arc.Point{X: bounds.Left, Y: bounds.Top}, 300*time.Millisecond); err != nil {
		s.Fatal("Failed to resize window: ", err)
	}

	// Moving the window slowly (in one second) to prevent triggering any kind of gesture like "snap to border", or "maximize".
	if err := act.MoveWindow(ctx, arc.Point{X: 0, Y: 0}, time.Second); err != nil {
		s.Fatal("Failed to move window: ", err)
	}

	restoreBounds, err := act.WindowBounds(ctx)
	if err != nil {
		s.Fatal("Failed to get window bounds: ", err)
	}

	centerHeight := bounds.Top + (bounds.Bottom-bounds.Top)/2

	// Perform 3 different subtests: resize from right border, from bottom border and from bottom-right border.
	// If one of these subtests fail, the test fails and the remaining subtests are not executed.
	// The bug is not 100% reproducible. It might happen that the test pass even if the bug is not fixed.

	// Resize should be as big as possible in order to have higher changes to trigger the bug.
	// But we should leave some margin to resize it back to its original size. That means the
	// window should not overlap the shelf; and we should leave some extra room to place the touches.

	// TODO(ricardoq): Find a robust way to get the marginForShelf value.
	// Shelf height depends on the DPI. According to internal tests, 200 pixels is more than enough
	// for high DPI devices like Nocturne which has a 128-pixel shelf. But this could break in the future.
	const marginForShelf = 200 // shelf height + extra room for touch
	for idx, entry := range []struct {
		desc     string
		border   arc.BorderType // resize origin (from which border)
		dst      arc.Point
		duration time.Duration
	}{
		{"right", arc.BorderRight, arc.Point{X: dispSize.W - marginForShelf, Y: centerHeight}, 100 * time.Millisecond},
		{"bottom", arc.BorderBottom, arc.Point{X: bounds.Left, Y: dispSize.H - marginForShelf}, 300 * time.Millisecond},
		{"bottom-right", arc.BorderBottomRight, arc.Point{X: dispSize.W - marginForShelf, Y: dispSize.H - marginForShelf}, 100 * time.Millisecond},
	} {
		s.Logf("Resizing window from %s border to %+v", entry.desc, entry.dst)
		if err := act.ResizeWindow(ctx, entry.border, entry.dst, entry.duration); err != nil {
			s.Fatal("Failed to resize activity: ", err)
		}

		img, err := grabScreenshot(ctx, cr, fmt.Sprintf("%s/screenshot-%d.png", s.OutDir(), idx))
		if err != nil {
			s.Fatal("Failed to grab screenshot: ", err)
		}

		bounds, err = act.WindowBounds(ctx)
		if err != nil {
			s.Fatal("Failed to get activity bounds: ", err)
		}

		subImage := img.(interface {
			SubImage(r image.Rectangle) image.Image
		}).SubImage(image.Rect(bounds.Left, bounds.Top, bounds.Right-bounds.Left, bounds.Bottom-bounds.Top))

		blackPixels := countBlackPixels(subImage)
		rect := subImage.Bounds()
		totalPixels := (rect.Max.Y - rect.Min.Y) * (rect.Max.X - rect.Min.X)
		percent := blackPixels * 100 / totalPixels
		s.Logf("Black pixels = %d / %d (%d%%)", blackPixels, totalPixels, percent)

		// "3 percent" is arbitrary. It shouldn't have any black pixel. But in case
		// the Settings app changes its default theme, we use 3% as a margin.
		if percent > 3 {
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

		// Restore the activity bounds.
		act.ResizeWindow(ctx, entry.border, arc.Point{X: restoreBounds.Left, Y: restoreBounds.Top}, 500*time.Millisecond)
	}
}

// countBlackPixels returns how many black pixels are contained in image.
func countBlackPixels(image image.Image) int {
	// TODO(ricardoq): At least on Eve, Nocturne, Caroline, Kevin and Dru the color
	// that we are looking for is RGBA(0,0,0,255). But it might be possible that
	// on certain devices the color is slightly different. In that case we should
	// adjust the colorMaxDiff.
	const colorMaxDiff = 0
	black := color.RGBA{0, 0, 0, 255}
	rect := image.Bounds()
	blackPixels := 0
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			if colorcmp.ColorsMatch(image.At(x, y), black, colorMaxDiff) {
				blackPixels++
			}
		}
	}
	return blackPixels
}

// grabScreenshot creates a screenshot in path, and returns an image.Image.
func grabScreenshot(ctx context.Context, cr *chrome.Chrome, path string) (image.Image, error) {
	if err := screenshot.CaptureChrome(ctx, cr, path); err != nil {
		return nil, errors.Wrap(err, "failed to capture screenshot")
	}

	fd, err := os.Open(path)
	if err != nil {
		return nil, errors.Wrap(err, "error opening screenshot file")
	}
	defer fd.Close()

	img, _, err := image.Decode(fd)
	if err != nil {
		return nil, errors.Wrap(err, "error decoding image file")
	}
	return img, nil
}
