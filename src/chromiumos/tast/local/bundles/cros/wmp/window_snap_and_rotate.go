// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wmp

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WindowSnapAndRotate,
		Desc:         "In clamshell mode, checks that snap in landscape and portrait and rotate works properly",
		Contacts:     []string{"cattalyya@chromium.org", "chromeos-wmp@google.com", "chromeos-sw-engprod@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeLoggedIn",
		Params: []testing.Param{{
			Name: "portrait",
			Val:  true,
		}, {
			Name: "landscape",
			Val:  false,
		}},
	})
}

func verifyState(ctx context.Context, s *testing.State, tconn *chrome.TestConn,
	windowID int, targetState ash.WindowStateType) {
	window, err := ash.GetWindow(ctx, tconn, windowID)
	if err != nil {
		s.Fatalf("Failed to obtain the window %v: %v", windowID, err)
	}
	if window.State != targetState {
		s.Fatalf("Expected window %v to be %v, got %v", windowID, targetState, window.State)
	}
}

func dragWindowTo(ctx context.Context, s *testing.State, tconn *chrome.TestConn,
	windowID int, targetPoint coords.Point, holdTime time.Duration) {
	window, err := ash.GetWindow(ctx, tconn, windowID)
	if err != nil {
		s.Fatalf("Failed to obtain the window %v: %v", windowID, err)
	}

	captionCenterPoint := coords.NewPoint(window.BoundsInRoot.CenterPoint().X, window.BoundsInRoot.Top+10)

	// Move the mouse to caption and press down.
	if err := mouse.Move(tconn, captionCenterPoint, 100*time.Millisecond)(ctx); err != nil {
		s.Fatal("Failed to move to caption")
	}
	if err := mouse.Press(tconn, mouse.LeftButton)(ctx); err != nil {
		s.Fatal("Failed to press the button")
	}

	// Drag the window around.
	const dragTime = 800 * time.Millisecond
	if err := mouse.Move(tconn, targetPoint, dragTime)(ctx); err != nil {
		s.Fatal("Failed to drag")
	}

	// Hold the window there for |holdTime| before releasing the mouse.
	if holdTime != 0 {
		if err := testing.Sleep(ctx, holdTime); err != nil {
			s.Fatal("Failed to wait")
		}
	}

	// Release the window. It is near the top of the screen so it should snap to maximize.
	if err := mouse.Release(tconn, mouse.LeftButton)(ctx); err != nil {
		s.Fatal("Failed to release the button")
	}

	// Wait for a window to finish snapping or maximizing animating before ending.
	if err := ash.WaitWindowFinishAnimating(ctx, tconn, windowID); err != nil {
		s.Fatal("Failed to wait for window animation")
	}
}

func WindowSnapAndRotate(ctx context.Context, s *testing.State) {
	// Reserve five seconds for various cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	cr := s.FixtValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	info, err := display.GetInternalInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to obtain internal display info: ", err)
	}

	// Rotate the screen if it is a portrait test.
	portrait := s.Param().(bool)
	portraitByDefault := info.Bounds.Height > info.Bounds.Width

	rotations := []display.RotationAngle{display.Rotate0, display.Rotate90, display.Rotate180, display.Rotate270}
	rotIndex := 0
	if portrait != portraitByDefault {
		if portrait {
			// Start with primary portrait which is |display.Rotate270| from the primary landscape display.
			rotIndex = 3
		} else {
			// Start with primary landscape which is |display.Rotate90| from the primary portrait display.
			rotIndex = 1
		}
		if err = display.SetDisplayRotationSync(ctx, tconn, info.ID, rotations[rotIndex]); err != nil {
			s.Fatal("Failed to rotate display: ", err)
		}
		defer display.SetDisplayRotationSync(cleanupCtx, tconn, info.ID, display.Rotate0)
	}

	// Obtain the latest display info after rotating the display.
	info, err = display.GetInternalInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to obtain internal display info: ", err)
	}
	// Create chrome apps that are already installed.
	appsList := []apps.App{apps.Files, apps.Help}

	for _, app := range appsList {
		if err := apps.Launch(ctx, tconn, app.ID); err != nil {
			s.Fatalf("Failed to launch %s: %s", app.Name, err)
		}
		if err := ash.WaitForApp(ctx, tconn, app.ID, time.Minute); err != nil {
			s.Fatalf("%s did not appear in shelf after launch: %s", app.Name, err)
		}
	}

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		s.Fatal("Failed to ensure clamshell mode: ", err)
	}
	defer cleanup(ctx)

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	workArea := info.WorkArea

	windows, err := ash.GetAllWindows(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to obtain all windows: ", err)
	}

	window1 := windows[0]
	window2 := windows[1]
	window1ID := window1.ID
	window2ID := window2.ID

	// Activate chrome window and exit from overview.
	if err := window1.ActivateWindow(ctx, tconn); err != nil {
		s.Fatal("Failed to activate chrome window: ", err)
	}

	if !window1.IsActive {
		s.Fatalf("The first window %d is not active", window1.ID)
	}

	if window1.State != ash.WindowStateNormal {
		s.Fatalf("The first window %d is not normal (state %q)", window1ID, window1.State)
	}

	const snapHoldTime = 0
	const maximizeHoldTime = 1300 * time.Millisecond

	if portrait {
		// Test drag to maximize, snap top and snap bottom.
		// Drag a window to the position that y-value is greater than |kSnapTriggerVerticalMoveThreshold|
		// first to make sure that it can be snapped top or maximized (see crbug/1158553).
		startingWindowPosition := coords.NewPoint(workArea.CenterPoint().X, workArea.Top+100)
		dragWindowTo(ctx, s, tconn, window1ID, startingWindowPosition, snapHoldTime)

		// Test that maximizing works with the presence of snap top when holding longer than a second.
		topSnappedPoint := coords.NewPoint(workArea.CenterPoint().X, workArea.Top)
		dragWindowTo(ctx, s, tconn, window1ID, topSnappedPoint, maximizeHoldTime)
		verifyState(ctx, s, tconn, window1ID, ash.WindowStateMaximized)
		// Drag a window down to unmaximize and then up to snap top.
		dragWindowTo(ctx, s, tconn, window1ID, startingWindowPosition, snapHoldTime)
		dragWindowTo(ctx, s, tconn, window1ID, topSnappedPoint, snapHoldTime)
		verifyState(ctx, s, tconn, window1ID, ash.WindowStateLeftSnapped)

		// Activate the second window to make sure it is not hidden behind |window1| and tests
		// drag to snap bottom.
		window2.ActivateWindow(ctx, tconn)
		bottomSnappedPoint := coords.NewPoint(workArea.CenterPoint().X, workArea.BottomRight().Y)
		dragWindowTo(ctx, s, tconn, window2ID, bottomSnappedPoint, snapHoldTime)

		// After snap both windows, tests their state.
		verifyState(ctx, s, tconn, window2ID, ash.WindowStateRightSnapped)
		// Make sure the first window still remains primary snapped.
		verifyState(ctx, s, tconn, window1ID, ash.WindowStateLeftSnapped)
	} else {
		// Test drag to snap left and right.
		leftSnappedPoint := coords.NewPoint(workArea.Left, workArea.CenterPoint().Y)
		dragWindowTo(ctx, s, tconn, window1ID, leftSnappedPoint, snapHoldTime)
		// Activate the second window first to make sure it is not hidden behind |window1|.
		window2.ActivateWindow(ctx, tconn)
		rightSnappedPoint := coords.NewPoint(workArea.BottomRight().X, workArea.CenterPoint().Y)
		dragWindowTo(ctx, s, tconn, window2ID, rightSnappedPoint, snapHoldTime)

		// Test states of windows after being snapped.
		window1, err = ash.GetWindow(ctx, tconn, window1ID)
		verifyState(ctx, s, tconn, window1ID, ash.WindowStateLeftSnapped)
		verifyState(ctx, s, tconn, window2ID, ash.WindowStateRightSnapped)
	}

	// Rotate the display for all four possible orientations and makes sure that
	// for each orientation, |window1| and |window2| remain primary and secondary
	// snapped respectively.
	for i := 1; i <= 4; i++ {
		if err = display.SetDisplayRotationSync(ctx, tconn, info.ID, rotations[(rotIndex+i)%len(rotations)]); err != nil {
			s.Fatal("Failed to rotate display: ", err)
		}
		verifyState(ctx, s, tconn, window1ID, ash.WindowStateLeftSnapped)
		verifyState(ctx, s, tconn, window2ID, ash.WindowStateRightSnapped)
	}
}
