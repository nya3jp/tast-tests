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

	if err := ac.Start(); err != nil {
		s.Fatal("Could not launch settings: ", err)
	}

	if result, err := ac.SetWindowState("normal"); err != nil {
		s.Fatal("Failed to set window state: ", err)
	} else {
		s.Log(result)
	}

	bounds, err := ac.Bounds()
	if err != nil {
		s.Fatal("Error getting bounds: ", err)
	}
	s.Logf("Bounds = %v", bounds)

	// make it as small as possible before the resizing
	ac.Resize(arc.BorderBottomRight, bounds.Left, bounds.Top, 300*time.Millisecond)

	// update bounds
	bounds, err = ac.Bounds()
	if err != nil {
		s.Fatal("Error getting bounds: ", err)
	}
	s.Logf("Updated bounds = %v", bounds)

	ac.Resize(arc.BorderRight, bounds.Right+400, bounds.Top, 200*time.Millisecond)
	ac.Resize(arc.BorderRight, bounds.Right, bounds.Top, 200*time.Millisecond)
	// ac.Resize(borderLeft, bounds.Left-400, bounds.Top, 200*time.Millisecond)
	// ac.Resize(borderLeft, bounds.Left, bounds.Top, 200*time.Millisecond)
	ac.Resize(arc.BorderTop, bounds.Left, bounds.Top-100, 200*time.Millisecond)
	// update bounds
	// bounds, err = ac.Bounds()
	// if err != nil {
	// 	s.Fatal("Error getting bounds: ", err)
	// }
	// s.Logf("Updated bounds = %v", bounds)

	// ac.Resize(borderTop, bounds.Left, bounds.Top, 200*time.Millisecond)
	// // update bounds
	// bounds, err = ac.Bounds()
	// if err != nil {
	// 	s.Fatal("Error getting bounds: ", err)
	// }
	// s.Logf("Updated bounds = %v", bounds)

	ac.Resize(arc.BorderBottom, bounds.Left, bounds.Bottom+400, 300*time.Millisecond)
	ac.Resize(arc.BorderBottom, bounds.Left, bounds.Bottom, 300*time.Millisecond)

	// ac.Resize(1500, 1200, 1*time.Second)
	// ac.Resize(1800, 1100, 200*time.Millisecond)

	screenshotName := "screenshot.png"
	path := filepath.Join(s.OutDir(), screenshotName)
	s.Logf("Screenshot should be placed: %s\n", path)

	if err := screenshot.CaptureChrome(ctx, cr, path); err != nil {
		s.Fatal("Error taking screenshot: ", err)
	}

	// s.Log("Sleeping for 10 seconds...")
	// sleep(ctx, 10*time.Second)
}
