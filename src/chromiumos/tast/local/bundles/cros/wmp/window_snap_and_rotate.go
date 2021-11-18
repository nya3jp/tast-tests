// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wmp

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
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
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "In clamshell mode, checks that snap in landscape and portrait works properly",
		Contacts: []string{
			"cattalyya@chromium.org",
			"chromeos-wmp@google.com",
			"chromeos-sw-engprod@google.com",
		},
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

func WindowSnapAndRotate(ctx context.Context, s *testing.State) {
	// Reserve ten seconds for various cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	cr := s.FixtValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	info, err := display.GetInternalInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to obtain internal display info: ", err)
	}

	defer display.SetDisplayRotationSync(cleanupCtx, tconn, info.ID, display.Rotate0)

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
	}

	// Obtain the latest display info after rotating the display.
	info, err = display.GetInternalInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to obtain internal display info: ", err)
	}

	// Open two windows, a Chrome browser and a File app.
	if err := ash.CreateWindows(ctx, tconn, cr, "", 1); err != nil {
		s.Fatal("Failed to create new windows: ", err)
	}

	app := apps.Files
	if err := apps.Launch(ctx, tconn, app.ID); err != nil {
		s.Fatalf("Failed to launch %s: %s", app.Name, err)
	}
	if err := ash.WaitForApp(ctx, tconn, app.ID, 10*time.Second); err != nil {
		s.Fatalf("%s did not appear in shelf after launch: %s", app.Name, err)
	}

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		s.Fatal("Failed to ensure clamshell mode: ", err)
	}
	defer cleanup(cleanupCtx)

	windows, err := ash.GetAllWindows(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to obtain all windows: ", err)
	}

	// Activate the first window.
	if err := windows[0].ActivateWindow(ctx, tconn); err != nil {
		s.Fatalf("Failed to activate the first window(id=%d): %v", windows[0].ID, err)
	}

	if window, err := ash.GetWindow(ctx, tconn, windows[0].ID); err != nil || !window.IsActive {
		s.Fatalf("The first window(id=%d) is not active", windows[0].ID)
	}

	if err = verifyState(ctx, tconn, windows[0].ID, ash.WindowStateNormal); err != nil {
		s.Fatalf("The first window(id=%d) is opened with non-default state: %v", windows[0].ID, err)
	}

	if err = verifyState(ctx, tconn, windows[1].ID, ash.WindowStateNormal); err != nil {
		s.Fatalf("The second window(id=%d) is opened with non-default state: %v", windows[1].ID, err)
	}

	workArea := info.WorkArea
	if portrait {
		// Test drag to maximize, snap top and snap bottom.
		// Drag a window to the position that y-value is greater than |kSnapTriggerVerticalMoveThreshold|
		// first to make sure that it can be snapped top or maximized (see crbug/1158553).
		startingWindowPosition := coords.NewPoint(workArea.CenterPoint().X, workArea.Top+100)
		if err = dragWindowTo(ctx, tconn, windows[0].ID, startingWindowPosition, 0); err != nil {
			s.Fatal("Failed to drag the first window to the starting position: ", err)
		}

		// Test that maximizing works with the presence of snap top when holding longer than a second.
		topSnappedPoint := coords.NewPoint(workArea.CenterPoint().X, workArea.Top)
		if err = dragWindowTo(ctx, tconn, windows[0].ID, topSnappedPoint, 1600*time.Millisecond); err != nil {
			s.Fatal("Failed to drag to maximize: ", err)
		}
		if err = verifyState(ctx, tconn, windows[0].ID, ash.WindowStateMaximized); err != nil {
			s.Fatal("Failed to maximize: ", err)
		}

		// Drag a window down to unmaximize and then up to snap top.
		if err = dragWindowTo(ctx, tconn, windows[0].ID, startingWindowPosition, 0); err != nil {
			s.Fatal("Failed to drag to unmaximize: ", err)
		}
		if err = dragWindowTo(ctx, tconn, windows[0].ID, topSnappedPoint, 0); err != nil {
			s.Fatal("Failed to drag to snap top: ", err)
		}
		// TODO(crbug/1264617): Rename left and right snapped to primary and secondary snapped.
		if err = verifyState(ctx, tconn, windows[0].ID, ash.WindowStateLeftSnapped); err != nil {
			s.Fatal("Failed to snap top: ", err)
		}

		// Activate the second window to make sure it is not hidden behind |windows[0]| before
		// dragging it to snap bottom.
		if err = windows[1].ActivateWindow(ctx, tconn); err != nil {
			s.Fatalf("Failed to activate the second window(id=%d): %v", windows[1].ID, err)
		}
		bottomSnappedPoint := coords.NewPoint(workArea.CenterPoint().X, workArea.BottomRight().Y)
		if err = dragWindowTo(ctx, tconn, windows[1].ID, bottomSnappedPoint, 0); err != nil {
			s.Fatal("Failed to drag to snap bottom: ", err)
		}

		// After snap both windows, tests their state.
		if err = verifyState(ctx, tconn, windows[1].ID, ash.WindowStateRightSnapped); err != nil {
			s.Fatal("The first window lost top-snapped state after snapping the second window to the bottom: ", err)
		}
		// Make sure the first window still remains primary snapped.
		if err = verifyState(ctx, tconn, windows[0].ID, ash.WindowStateLeftSnapped); err != nil {
			s.Fatal("Failed to snap bottom: ", err)
		}
	} else {
		// For landscape display, test drag to snap left and right.
		leftSnappedPoint := coords.NewPoint(workArea.Left, workArea.CenterPoint().Y)
		if err = dragWindowTo(ctx, tconn, windows[0].ID, leftSnappedPoint, 0); err != nil {
			s.Fatal("Failed to drag to snap left: ", err)
		}
		// Activate the second window first to make sure it is not hidden behind |windows[0]|
		// before dragging it to snap right.
		if err = windows[1].ActivateWindow(ctx, tconn); err != nil {
			s.Fatalf("Failed to activate the second window(id=%d): %v", windows[1].ID, err)
		}
		rightSnappedPoint := coords.NewPoint(workArea.BottomRight().X, workArea.CenterPoint().Y)
		if err = dragWindowTo(ctx, tconn, windows[1].ID, rightSnappedPoint, 0); err != nil {
			s.Fatal("Failed to drag to snap right: ", err)
		}

		// Test states of windows after being snapped.
		if err = verifyState(ctx, tconn, windows[0].ID, ash.WindowStateLeftSnapped); err != nil {
			s.Fatal("Failed to snap left: ", err)
		}
		if err = verifyState(ctx, tconn, windows[1].ID, ash.WindowStateRightSnapped); err != nil {
			s.Fatal("Failed to snap right: ", err)
		}
	}

	// Rotate the display for all four possible orientations and makes sure that
	// for each orientation, |windows[0]| and |windows[1]| remain primary and secondary
	// snapped respectively.
	for i := 1; i <= 4; i++ {
		rot := rotations[(rotIndex+i)%len(rotations)]
		if err = display.SetDisplayRotationSync(ctx, tconn, info.ID, rot); err != nil {
			s.Fatal("Failed to rotate display: ", err)
		}
		if err = verifyState(ctx, tconn, windows[0].ID, ash.WindowStateLeftSnapped); err != nil {
			s.Fatalf("The first window lost primary snapped state after rotating %d times to rotation = %v: %v", i, rot, err)
		}
		if err = verifyState(ctx, tconn, windows[1].ID, ash.WindowStateRightSnapped); err != nil {
			s.Fatalf("The second window lost primary snapped state after rotating %d times to rotation = %v: %v", i, rot, err)
		}
	}
}

// verifyState checks whether the state of the window with the given id |windowID| is |wantState| or not.
func verifyState(ctx context.Context, tconn *chrome.TestConn,
	windowID int, wantState ash.WindowStateType) error {
	window, err := ash.GetWindow(ctx, tconn, windowID)
	if err != nil {
		return errors.Wrapf(err, "failed to obtain window(id=%d)", windowID)
	}
	if window.State != wantState {
		return errors.Errorf("unexpected window(id=%d) state = got %v, want %v",
			windowID, window.State, wantState)
	}
	return nil
}

// dragWindowTo drags the caption center of the window with the given id |windowID| to |targetPoint|
// via a mouse and holds for |holdDuration| at the target point before releasing the mouse.
func dragWindowTo(ctx context.Context, tconn *chrome.TestConn,
	windowID int, targetPoint coords.Point, holdDuration time.Duration) error {
	window, err := ash.GetWindow(ctx, tconn, windowID)
	if err != nil {
		return errors.Wrapf(err, "failed to obtain window(id=%d)", windowID)
	}

	captionCenterPoint := coords.NewPoint(window.BoundsInRoot.CenterPoint().X, window.BoundsInRoot.Top+10)

	// Move the mouse to caption and press down.
	if err := mouse.Move(tconn, captionCenterPoint, 100*time.Millisecond)(ctx); err != nil {
		return errors.Wrap(err, "failed to move to caption")
	}
	if err := mouse.Press(tconn, mouse.LeftButton)(ctx); err != nil {
		return errors.Wrap(err, "failed to press the button")
	}

	// Drag the window around.
	const dragTime = 800 * time.Millisecond
	if err := mouse.Move(tconn, targetPoint, dragTime)(ctx); err != nil {
		return errors.Wrap(err, "failed to drag")
	}

	// Hold the window there for |holdDuration| before releasing the mouse.
	if err := testing.Sleep(ctx, holdDuration); err != nil {
		return errors.Wrap(err, "failed to sleep")
	}

	// Release the window. It is near the top of the screen so it should snap to maximize.
	if err := mouse.Release(tconn, mouse.LeftButton)(ctx); err != nil {
		return errors.Wrap(err, "failed to release the button")
	}

	// Wait for a window to finish snapping or maximizing animating before ending.
	if err := ash.WaitWindowFinishAnimating(ctx, tconn, windowID); err != nil {
		return errors.Wrap(err, "failed to wait for window animation to finish")
	}

	return nil
}
