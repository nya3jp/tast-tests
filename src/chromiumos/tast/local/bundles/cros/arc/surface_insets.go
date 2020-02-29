// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SurfaceInsets,
		Desc:         "Test to handle SurfaceInsets not to exceed android window frame",
		Contacts:     []string{"hirokisato@google.com", "arc-framework+tast@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"android_p", "chrome"},
		Data:         []string{"ArcSurfaceInsetsTestApp.apk"},
		Timeout:      4 * time.Minute,
	})
}

func SurfaceInsets(ctx context.Context, s *testing.State) {
	const (
		apk = "ArcSurfaceInsetsTestApp.apk"
		pkg = "org.chromium.arc.testapp.surfaceinsets"
		cls = ".MainActivity"
	)

	// TODO(crbug.com/1002958) Replace with Ash API to enable clamshell mode once it gets fixed.
	// With the Ash flag, we can also use precondition arc.Booted() and tast efficiency improves with it.
	cr, err := chrome.New(ctx, chrome.ARCEnabled(), chrome.ExtraArgs("--force-tablet-mode=clamshell"))
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	// Prepare TouchScreen.
	// Touchscreen bounds: The size of the touchscreen might not be the same
	// as the display size.
	tsw, err := input.Touchscreen(ctx)
	if err != nil {
		s.Fatal("Failed to open touchscreen device: ", err)
	}
	defer tsw.Close()

	stw, err := tsw.NewSingleTouchWriter()
	if err != nil {
		s.Fatal("Could not get a new TouchEventWriter: ", err)
	}
	defer stw.Close()

	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close()

	if err := a.Install(ctx, s.DataPath(apk)); err != nil {
		s.Fatal("Failed installing app: ", err)
	}

	act, err := arc.NewActivity(a, pkg, cls)
	if err != nil {
		s.Fatal("Failed to create new activity: ", err)
	}
	defer act.Close()

	if err := act.Start(ctx); err != nil {
		s.Fatal("Failed start Settings activity: ", err)
	}

	if err := act.WaitForResumed(ctx, time.Second); err != nil {
		s.Fatal("Failed to wait for activity to resume: ", err)
	}

	if err := act.SetWindowState(ctx, arc.WindowStateNormal); err != nil {
		s.Fatal("Failed to set window state to Normal: ", err)
	}

	if err := ash.WaitForARCAppWindowState(ctx, tconn, act.PackageName(), ash.WindowStateNormal); err != nil {
		s.Fatal("Failed to wait for window state Normal: ", err)
	}

	disp, err := arc.NewDisplay(a, arc.DefaultDisplayID)
	if err != nil {
		s.Fatal("Failed to obtain a default display: ", err)
	}

	arcDisplaySize, err := disp.Size(ctx)
	if err != nil {
		s.Fatal("Failed to get arc size: ", err)
	}

	arcCaptionHeight, err := disp.CaptionHeight(ctx)
	if err != nil {
		s.Fatal("Failed to get arc size: ", err)
	}

	tcc := coords.NewTouchCoordConverter(arcDisplaySize, tsw)

	// Repeat twice to validate b/80441010.
	for _, op := range []struct {
		op    string
		state ash.WindowStateType
	}{
		{"maximize", ash.WindowStateMaximized},
		{"restore", ash.WindowStateNormal},
		{"maximize", ash.WindowStateMaximized},
		{"restore", ash.WindowStateNormal},
	} {
		s.Logf("Pressing %q", op)

		activityBounds, err := act.WindowBounds(ctx)
		if err != nil {
			s.Fatal("Failed to get window bounds: ", err)
		}

		// Touch lower edge of the restore/maximize button to validate that any surface does not cover the caption button.
		// TODO(crbug.com/1005010) : Do not hard code the calculation of caption position below.
		// Instead, we should get Chrome constants in real time.
		buttonCoordX, buttonCoordY := tcc.ConvertLocation(coords.NewPoint(
			activityBounds.Left+activityBounds.Width-arcCaptionHeight/2-arcCaptionHeight,
			activityBounds.Top+arcCaptionHeight-arcCaptionHeight/10))

		stw.Move(buttonCoordX, buttonCoordY)
		stw.End()

		if err := ash.WaitForARCAppWindowState(ctx, tconn, act.PackageName(), ash.WindowStateNormal); err != nil {
			s.Fatalf("Pressing %q did not work: tried to touch (%d, %d): %v", op, buttonCoordX, buttonCoordY, err)
		}
	}
}
