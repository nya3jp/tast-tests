// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
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
		Desc:         "Verifies that resizing ARC++ applications work",
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

	// make it as small as possible before the resizing, and move to the left-top corner
	ac.Resize(arc.BorderBottomRight, bounds.Left, bounds.Top, 300*time.Millisecond)
	ac.Move(0, 0, 1000*time.Millisecond)

	// update bounds
	bounds, err = ac.Bounds()
	if err != nil {
		s.Fatal("Error getting bounds: ", err)
	}
	s.Logf("Updated bounds = %v", bounds)
	restoreBounds := bounds

	// centerWidth := bounds.Left + (bounds.Right-bounds.Left)/2
	centerHeight := bounds.Top + (bounds.Bottom-bounds.Top)/2
	for _, entry := range []struct {
		border   uint
		x        int
		y        int
		duration time.Duration
	}{
		{arc.BorderRight, bounds.Right + 1500, centerHeight, 500 * time.Millisecond},
		// {arc.BorderLeft, bounds.Left - 400, bounds.Top, 200 * time.Millisecond},
		// {arc.BorderBottom, bounds.Left, bounds.Bottom + 400, 300 * time.Millisecond},
		// {arc.BorderTop, centerWidth, bounds.Top - 60, 200 * time.Millisecond},
		// {arc.BorderBottomRight, bounds.Right + 400, bounds.Bottom + 400, 300 * time.Millisecond},
		// {arc.BorderBottomLeft, bounds.Left - 400, bounds.Bottom + 400, 300 * time.Millisecond},
		// {arc.BorderTopRight, bounds.Right + 400, bounds.Top - 60, 300 * time.Millisecond},
		// {arc.BorderTopLeft, bounds.Left - 400, bounds.Top - 60, 300 * time.Millisecond},
	} {
		sleep(ctx, 200*time.Millisecond)
		s.Log("----------------------------------------------------- BEGIN ")
		ac.Resize(entry.border, entry.x, entry.y, entry.duration)
		sleep(ctx, 200*time.Millisecond)
		ac.Resize(entry.border, restoreBounds.Left, restoreBounds.Top, 500*time.Millisecond)
		s.Log("----------------------------------------------------- END ")
	}

	screenshotName := "screenshot.png"
	path := filepath.Join(s.OutDir(), screenshotName)
	s.Logf("Screenshot should be placed: %s\n", path)

	if err := screenshot.CaptureChrome(ctx, cr, path); err != nil {
		s.Fatal("Error taking screenshot: ", err)
	}
}

func sleep(ctx context.Context, t time.Duration) error {
	select {
	case <-time.After(t):
	case <-ctx.Done():
		return ctx.Err()
	}
	return nil
}
