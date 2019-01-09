// Copyright 2018 The Chromium OS Authors. All rights reserved.
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

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ResizeActivity,
		Desc:         "Verifies that resizing ARC++ applications don't generate black content",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android", "android_p", "chrome_login"},
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
		s.Fatal("Could not create new activity: ", err)
	}
	defer ac.Close()

	if err := ac.Start(); err != nil {
		s.Fatal("Could not launch settings: ", err)
	}

	if result, err := ac.SetWindowState(arc.WindowNormal); err != nil {
		s.Fatal("Failed to set window state: ", err)
	} else {
		s.Log(result)
	}

	bounds, err := ac.Bounds()
	if err != nil {
		s.Fatal("Error getting bounds: ", err)
	}
	s.Logf("Bounds = %v\n", bounds)
	s.Logf("Half W = %d\n", (bounds.Right-bounds.Left)/2)

	dispWidth, dispHeight, err := disp.Bounds()
	if err != nil {
		s.Fatal("Failed to get display bounds")
	}

	// Make it as small as possible before the resizing, and move to the left-top corner.
	if err := ac.Resize(arc.BorderBottomRight, bounds.Left, bounds.Top, 300*time.Millisecond); err != nil {
		s.Fatal("Failed to resize activity: ", err)
	}
	if err := ac.Move(0, 0, 1000*time.Millisecond); err != nil {
		s.Fatal("Failed to move activity: ", err)
	}

	// Update bounds.
	bounds, err = ac.Bounds()
	if err != nil {
		s.Fatal("Error getting bounds: ", err)
	}
	s.Logf("Updated bounds = %v", bounds)
	restoreBounds := bounds

	screenshotName := "screenshot"

	// centerWidth := bounds.Left + (bounds.Right-bounds.Left)/2
	centerHeight := bounds.Top + (bounds.Bottom-bounds.Top)/2
	for idx, entry := range []struct {
		border   uint
		x        int
		y        int
		duration time.Duration
	}{
		{arc.BorderRight, dispWidth - 150, centerHeight, 100 * time.Millisecond},
		// {arc.BorderLeft, bounds.Left - 400, bounds.Top, 200 * time.Millisecond},
		{arc.BorderBottom, bounds.Left, dispHeight - 200, 300 * time.Millisecond},
		// {arc.BorderTop, centerWidth, bounds.Top - 60, 200 * time.Millisecond},
		{arc.BorderBottomRight, dispWidth - 150, dispHeight - 200, 100 * time.Millisecond},
		// {arc.BorderBottomLeft, bounds.Left - 400, bounds.Bottom + 400, 300 * time.Millisecond},
		// {arc.BorderTopRight, bounds.Right + 400, bounds.Top - 60, 300 * time.Millisecond},
		// {arc.BorderTopLeft, bounds.Left - 400, bounds.Top - 60, 300 * time.Millisecond},
	} {
		sleep(ctx, 200*time.Millisecond)
		s.Log("----------------------------------------------------- BEGIN ")
		if err := ac.Resize(entry.border, entry.x, entry.y, entry.duration); err != nil {
			s.Fatal("Failed to resize activity: ", err)
		}

		path := filepath.Join(s.OutDir(), fmt.Sprintf("%s-%d.png", screenshotName, idx))
		s.Logf("Screenshot should be placed: %s\n", path)
		if err := screenshot.CaptureChrome(ctx, cr, path); err != nil {
			s.Fatal("Error taking screenshot: ", err)
		}

		fImg1, _ := os.Open(path)
		defer fImg1.Close()
		img, encode, err := image.Decode(fImg1)
		if err != nil {
			s.Fatal("Error opening screenshot file: ", err)
		}
		s.Logf("Encode: %s", encode)

		si, ok := (img).(subImager)
		if !ok {
			s.Fatal("image doesn't have subimage")
		}

		bounds, _ = ac.Bounds()
		sub := si.SubImage(
			image.Rect(bounds.Left, bounds.Top, bounds.Right-bounds.Left, bounds.Bottom-bounds.Top))

		s.Logf("Subimage: %+v / %+v", sub.Bounds(), bounds)

		fdNew, err := os.Create(fmt.Sprintf("/tmp/screenshot_%d.png", idx))
		if err != nil {
			s.Fatal("Could not create screenshot: ", err)
		}
		defer fdNew.Close()
		png.Encode(fdNew, sub)

		blackPixels := 0
		rect := sub.Bounds()
		for y := rect.Min.Y; y < rect.Max.Y; y++ {
			for x := rect.Min.X; x < rect.Max.X; x++ {
				p := sub.At(x, y)
				tc := color.RGBA{0, 0, 0, 255}
				if p == tc {
					blackPixels++
				}
			}
		}
		totalPixels := (rect.Max.Y - rect.Min.Y) * (rect.Max.X - rect.Min.X)
		s.Logf("black pixels = %d / %d (%d%%)", blackPixels, totalPixels, blackPixels*100/totalPixels)

		sleep(ctx, 200*time.Millisecond)
		ac.Resize(entry.border, restoreBounds.Left, restoreBounds.Top, 500*time.Millisecond)
		s.Log("----------------------------------------------------- END ")
	}
}

// Taken from here: https://stackoverflow.com/a/16093117/1119460
type subImager interface {
	SubImage(r image.Rectangle) image.Image
}

func sleep(ctx context.Context, t time.Duration) error {
	select {
	case <-time.After(t):
	case <-ctx.Done():
		return ctx.Err()
	}
	return nil
}
