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
	_ "image/png" // register the PNG format with the image package
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ResizeActivity,
		Desc:         "Checks that resizing ARC applications works without generating black background",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android_p", "chrome_login"},
		Timeout:      4 * time.Minute,
	})
}

func ResizeActivity(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx, chrome.ARCEnabled())
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close()

	disp, err := arc.NewDisplay(ctx, a, arc.DefaultDisplayID)
	if err != nil {
		s.Fatal("Failed to obtain a default display: ", err)
	}
	dispMode, err := disp.Mode()
	if err != nil {
		s.Fatal("Failed to get display mode: ", err)
		return
	}

	if dispMode != arc.ClamshellMode {
		s.Log("This test only runs when device is in clamshell (resizable) mode")
		return
	}

	ac, err := arc.NewActivity(ctx, a, "com.android.settings", ".Settings")
	if err != nil {
		s.Fatal("Failed to create new activity: ", err)
	}
	defer ac.Close()

	if err := ac.Start(); err != nil {
		s.Fatal("Failed start Settings activity: ", err)
	}

	if err := ac.SetWindowState(arc.WindowNormal); err != nil {
		s.Fatal("Failed to set window state to Normal: ", err)
	}

	bounds, err := ac.Bounds()
	if err != nil {
		s.Fatal("Failed getting activity bounds: ", err)
	}

	dispWidth, dispHeight, err := disp.Bounds()
	if err != nil {
		s.Fatal("Failed to get display bounds")
	}

	// Make it as small as possible before the resizing, and place it on the left-top corner
	// in order to have maximum real-estate for the resizing.
	if err := ac.Resize(arc.BorderBottomRight, bounds.Left, bounds.Top, 300*time.Millisecond); err != nil {
		s.Fatal("Failed to resize activity: ", err)
	}
	if err := ac.Move(0, 0, 1000*time.Millisecond); err != nil {
		s.Fatal("Failed to move activity: ", err)
	}

	restoreBounds, err := ac.Bounds()
	if err != nil {
		s.Fatal("Failed to get activity bounds: ", err)
	}

	centerHeight := bounds.Top + (bounds.Bottom-bounds.Top)/2

	// Perform 3 different tests: resize to right, to the bottom and to the bottom-right.
	// Resize should be as big as possible in order to have higher changes to trigger the bug.
	// But we should leave some margin to resize it back to its orignal size. That means the
	// apk should not overlap the shelf.
	const marginForShelf = 200
	for idx, entry := range []struct {
		border   uint // resize origin (from which border)
		x        int  // resize destination
		y        int
		duration time.Duration
	}{
		{arc.BorderRight, dispWidth - marginForShelf, centerHeight, 100 * time.Millisecond},
		{arc.BorderBottom, bounds.Left, dispHeight - marginForShelf, 300 * time.Millisecond},
		{arc.BorderBottomRight, dispWidth - marginForShelf, dispHeight - marginForShelf, 100 * time.Millisecond},
	} {
		if err := ac.Resize(entry.border, entry.x, entry.y, entry.duration); err != nil {
			s.Fatal("Failed to resize activity: ", err)
		}

		img, err := grabScreenshot(ctx, cr, fmt.Sprintf("%s/screenshot-%s.png", s.OutDir(), idx))
		if err != nil {
			s.Fatal("Failed to grab screenshot: ", err)
		}

		si, ok := (img).(subImager)
		if !ok {
			s.Fatal("Failed to create a subimage of the screenshot")
		}

		bounds, err = ac.Bounds()
		if err != nil {
			s.Fatal("Failed to get activity bounds: ", err)
		}
		subImage := si.SubImage(image.Rect(bounds.Left, bounds.Top, bounds.Right-bounds.Left, bounds.Bottom-bounds.Top))

		// Log results
		blackPixels := countBlackPixels(subImage)
		rect := subImage.Bounds()
		totalPixels := (rect.Max.Y - rect.Min.Y) * (rect.Max.X - rect.Min.X)
		percent := blackPixels * 100 / totalPixels
		s.Logf("Black pixels = %d / %d (%d%%)", blackPixels, totalPixels, percent)

		// "3 percent" is arbitrary. It shouldn't have any black pixel. But in case
		// the Settings apk changes its default theme, we use 3% as a margin.
		if percent > 3 {
			// Save image with black pixels
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
		ac.Resize(entry.border, restoreBounds.Left, restoreBounds.Top, 500*time.Millisecond)
	}
}

func countBlackPixels(image image.Image) int {
	rect := image.Bounds()
	blackPixels := 0
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			p := image.At(x, y)
			tc := color.RGBA{0, 0, 0, 255}
			if p == tc {
				blackPixels++
			}
		}
	}
	return blackPixels
}

// grabScreenshot creates a screenshot in path, and returns a image.Image
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

// Taken from here: https://stackoverflow.com/a/16093117/1119460
type subImager interface {
	SubImage(r image.Rectangle) image.Image
}
