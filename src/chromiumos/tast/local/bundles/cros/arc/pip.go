// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"fmt"
	"math"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
)

const (
	// kCollisionWindowWorkAreaInsetsDp is hardcoded to 8dp. Using a bigger value to safe.
	// See: https://cs.chromium.org/chromium/src/ash/wm/collision_detection/collision_detection_utils.h
	// TODO(crbug.com/949754): Get this value in runtime.
	collisionWindowWorkAreaInsetsDP = 8 + 5

	// pipPositionErrorMarginPX represents the error margin in pixels when comparing positions. See b/129976114 for more info.
	// TODO(ricardoq): Remove this constant once the bug gets fixed.
	pipPositionErrorMarginPX = 1
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PIP,
		Desc:         "Checks that ARC++ Picture-in-Picture works as expected",
		Contacts:     []string{"edcourtney@chromium.org", "arc-framework+tast@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"tablet_mode", "android_p", "chrome"},
		Data:         []string{"ArcPipTastTest.apk"},
		Pre:          arc.Booted(),
		Timeout:      5 * time.Minute,
	})
}

func PIP(ctx context.Context, s *testing.State) {
	must := func(err error) {
		if err != nil {
			s.Fatal("Failed: ", err)
		}
	}

	// For debugging, create a Chrome session with chrome.ExtraArgs("--show-taps")
	cr := s.PreValue().(arc.PreData).Chrome

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	a := s.PreValue().(arc.PreData).ARC

	const apkName = "ArcPipTastTest.apk"
	s.Log("Installing ", apkName)
	if err := a.Install(ctx, s.DataPath(apkName)); err != nil {
		s.Fatal("Failed installing app: ", err)
	}

	const pkgName = "org.chromium.arc.testapp.pictureinpicture"
	act, err := arc.NewActivity(a, pkgName, ".MainActivity")
	if err != nil {
		s.Fatal("Failed to create new activity: ", err)
	}
	defer act.Close()

	dev, err := ui.NewDevice(ctx, a)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer dev.Close()

	dispInfo, err := display.GetInternalInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get internal display info: ", err)
	}

	origShelfAlignment, err := ash.GetShelfAlignment(ctx, tconn, dispInfo.ID)
	if err != nil {
		s.Fatal("Failed to get shelf alignment: ", err)
	}
	if err := ash.SetShelfAlignment(ctx, tconn, dispInfo.ID, ash.ShelfAlignmentBottom); err != nil {
		s.Fatal("Failed to set shelf alignment to Bottom: ", err)
	}
	// Be nice and restore shelf alignment to its original state on exit.
	defer ash.SetShelfAlignment(ctx, tconn, dispInfo.ID, origShelfAlignment)

	origShelfBehavior, err := ash.GetShelfBehavior(ctx, tconn, dispInfo.ID)
	if err != nil {
		s.Fatal("Failed to get shelf behavior: ", err)
	}
	if err := ash.SetShelfBehavior(ctx, tconn, dispInfo.ID, ash.ShelfBehaviorNeverAutoHide); err != nil {
		s.Fatal("Failed to set shelf behavior to Never Auto Hide: ", err)
	}
	// Be nice and restore shelf behavior to its original state on exit.
	defer ash.SetShelfBehavior(ctx, tconn, dispInfo.ID, origShelfBehavior)

	tabletModeEnabled, err := ash.TabletModeEnabled(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get tablet mode: ", err)
	}
	// Be nice and restore tablet mode to its original state on exit.
	defer ash.SetTabletModeEnabled(ctx, tconn, tabletModeEnabled)

	dispMode, err := getInternalDisplayMode(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get display mode: ", err)
	}

	// Run all subtests twice. First, with tablet mode disabled. And then, with it enabled.
	for _, tabletMode := range []bool{false, true} {
		s.Logf("Running tests with tablet mode enabled=%t", tabletMode)
		if err := ash.SetTabletModeEnabled(ctx, tconn, tabletMode); err != nil {
			s.Fatalf("Failed to set tablet mode enabled to %t: %v", tabletMode, err)
		}

		type testFunc func(context.Context, *chrome.Conn, *arc.Activity, *ui.Device, *display.DisplayMode) error
		for idx, test := range []struct {
			name string
			fn   testFunc
		}{
			{"PIP Move", testPIPMove},
			{"PIP Resize", testPIPResize},
			{"PIP Fling", testPIPFling},
			{"PIP GravityStatusArea", testPIPGravityStatusArea},
			{"PIP GravityShelfAutoHide", testPIPGravityShelfAutoHide},
			{"PIP Toggle Tablet mode", testPIPToggleTabletMode},
		} {
			s.Logf("Running %q", test.name)
			// Clear task WM state. Windows should be positioned at their default location.
			if err := a.Command(ctx, "am", "broadcast", "-a", "android.intent.action.arc.cleartaskstate").Run(); err != nil {
				s.Error("Failed to clear WM state: ", err)
			}
			must(act.Start(ctx))
			must(act.WaitForIdle(ctx, 5*time.Second))
			// Press button that triggers PIP mode in activity.
			const pipButtonID = pkgName + ":id/enter_pip"
			must(dev.Object(ui.ID(pipButtonID)).Click(ctx))
			// TODO(b/131248000) WaitForIdle doesn't catch all PIP possible animations.
			// Add temporary delay until it gets fixed.
			must(testing.Sleep(ctx, 200*time.Millisecond))
			must(act.WaitForIdle(ctx, 5*time.Second))

			if err := test.fn(ctx, tconn, act, dev, dispMode); err != nil {
				path := fmt.Sprintf("%s/screenshot-pip-failed-test-%d.png", s.OutDir(), idx)
				if err := screenshot.CaptureChrome(ctx, cr, path); err != nil {
					s.Log("Failed to capture screenshot: ", err)
				}
				s.Errorf("%s test with tablet mode(%t) failed: %v", test.name, tabletMode, err)
			}
			must(act.Stop(ctx))
		}
	}
}

// testPIPMove verifies that drag-moving the PIP window works as expected.
// It does that by drag-moving that PIP window horizontally 3 times and verifying that the position is correct.
func testPIPMove(ctx context.Context, tconn *chrome.Conn, act *arc.Activity, dev *ui.Device, dispMode *display.DisplayMode) error {
	const (
		// When the drag-move sequence is started, the gesture controller might miss a few pixels before it finally
		// recognizes it as a drag-move gesture. This is specially true for PIP windows.
		// The value varies depending on acceleration/speed of the gesture. 35 works for our purpose.
		missedByGestureControllerDP = 35
		movementDuration            = time.Second
		totalMovements              = 3
	)

	missedByGestureControllerPX := int(math.Round(missedByGestureControllerDP * dispMode.DeviceScaleFactor))
	testing.ContextLog(ctx, "Using: missedByGestureControllerPX = ", missedByGestureControllerPX)

	bounds, err := act.WindowBounds(ctx)
	if err != nil {
		return errors.Wrap(err, "could not get PIP window bounds")
	}
	testing.ContextLogf(ctx, "Initial PIP bounds: %+v", bounds)

	deltaX := dispMode.WidthInNativePixels / (totalMovements + 1)
	for i := 0; i < totalMovements; i++ {
		testing.ContextLogf(ctx, "Moving PIP window to %d,%d", bounds.Left-deltaX, bounds.Top)
		if err := act.MoveWindow(ctx, arc.NewPoint(bounds.Left-deltaX, bounds.Top), movementDuration); err != nil {
			return errors.Wrap(err, "could not move PIP window")
		}
		if err := act.WaitForIdle(ctx, 5*time.Second); err != nil {
			return err
		}

		newBounds, err := act.WindowBounds(ctx)
		if err != nil {
			return errors.Wrap(err, "could not get PIP window bounds")
		}
		testing.ContextLogf(ctx, "PIP bounds after move: %+v", newBounds)

		diff := bounds.Left - deltaX - newBounds.Left
		if diff > missedByGestureControllerPX {
			return errors.Wrapf(err, "invalid PIP bounds: %+v; expected %d < %d", bounds, diff, missedByGestureControllerPX)
		}
		bounds = newBounds
	}
	return nil
}

// testPIPResize verifies that resizing the PIP window works as expected.
// It performs a drag-resize from PIP's left-top corner and compares the resized-PIP size with the expected one.
func testPIPResize(ctx context.Context, tconn *chrome.Conn, act *arc.Activity, dev *ui.Device, dispMode *display.DisplayMode) error {
	// Activate PIP "resize handler", otherwise resize will fail. See:
	// https://android.googlesource.com/platform/frameworks/base/+/refs/heads/pie-release/services/core/java/com/android/server/policy/PhoneWindowManager.java#6387
	if err := dev.PressKeyCode(ctx, ui.KEYCODE_WINDOW, 0); err != nil {
		return errors.Wrap(err, "could not activate PIP menu")
	}
	if err := act.WaitForIdle(ctx, 5*time.Second); err != nil {
		return err
	}

	bounds, err := act.WindowBounds(ctx)
	if err != nil {
		return errors.Wrap(err, "could not get PIP window bounds")
	}
	testing.ContextLogf(ctx, "Bounds before resize: %+v", bounds)

	testing.ContextLog(ctx, "Resizing window to x=0, y=0")
	// Resizing PIP to x=0, y=0, but it should stop once it reaches its max size.
	if err := act.ResizeWindow(ctx, arc.BorderTopLeft, arc.NewPoint(0, 0), time.Second); err != nil {
		return errors.Wrap(err, "could not resize PIP window")
	}

	bounds, err = act.WindowBounds(ctx)
	if err != nil {
		return errors.Wrap(err, "could not get PIP window bounds")
	}
	testing.ContextLogf(ctx, "Bounds after resize: %+v", bounds)

	rectDP, err := getShelfRect(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get shelf rect")
	}
	shelfHeightPX := int(math.Round(float64(rectDP.Height) * dispMode.DeviceScaleFactor))

	// Max PIP window size relative to the display size, as defined in WindowPosition.getMaximumSizeForPip().
	// See: https://cs.corp.google.com/pi-arc-dev/frameworks/base/services/core/arc/java/com/android/server/am/WindowPositioner.java
	// Dividing by integer 2 could loose the fraction, but so does the Java implementation.
	// TODO(crbug.com/949754): Get this value in runtime.
	const pipMaxSizeFactor = 2
	pipMaxSizeW := dispMode.WidthInNativePixels / pipMaxSizeFactor
	pipMaxSizeH := (dispMode.HeightInNativePixels - shelfHeightPX) / pipMaxSizeFactor

	// Aspect ratio gets honored after resize. Only test one dimension.
	if pipMaxSizeH < pipMaxSizeW {
		if err := checkRange("height", bounds.Height,
			pipMaxSizeH-pipPositionErrorMarginPX, pipMaxSizeH+pipPositionErrorMarginPX); err != nil {
			return err
		}
	} else {
		if err := checkRange("width", bounds.Width,
			pipMaxSizeW-pipPositionErrorMarginPX, pipMaxSizeW+pipPositionErrorMarginPX); err != nil {
			return err
		}
	}
	return nil
}

// testPIPFling tests that fling works as expected. It tests the fling gesture in four directions: left, up, right and down.
func testPIPFling(ctx context.Context, tconn *chrome.Conn, act *arc.Activity, dev *ui.Device, dispMode *display.DisplayMode) error {
	type borderType int
	const (
		// Borders to check after swipe.
		left borderType = iota
		right
		top
		bottom
	)

	tsw, err := input.Touchscreen(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to open touchscreen device")
	}
	defer tsw.Close()

	stw, err := tsw.NewSingleTouchWriter()
	if err != nil {
		return errors.Wrap(err, "could not create TouchEventWriter")
	}
	defer stw.Close()

	collisionWindowWorkAreaInsetsPX := int(math.Round(collisionWindowWorkAreaInsetsDP * dispMode.DeviceScaleFactor))
	testing.ContextLog(ctx, "Using: collisionWindowWorkAreaInsetsPX = ", collisionWindowWorkAreaInsetsPX)

	// Calculate Pixel (screen display) / Tuxel (touch device) ratio.
	dispW := dispMode.WidthInNativePixels
	dispH := dispMode.HeightInNativePixels
	pixelToTuxelX := float64(tsw.Width()) / float64(dispW)
	pixelToTuxelY := float64(tsw.Height()) / float64(dispH)

	for _, dir := range []struct {
		x, y   int
		border borderType
	}{
		{-1, 0, left},  // swipe to left
		{0, -1, top},   // swipe to top
		{1, 0, right},  // swipe to right
		{0, 1, bottom}, // swipe to bottom
	} {
		if err := act.WaitForIdle(ctx, 5*time.Second); err != nil {
			return err
		}
		bounds, err := act.WindowBounds(ctx)
		if err != nil {
			return errors.Wrap(err, "could not get PIP window bounds")
		}

		pipCenterX := float64(bounds.Left + bounds.Width/2)
		pipCenterY := float64(bounds.Top + bounds.Height/2)

		x0 := input.TouchCoord(pipCenterX * pixelToTuxelX)
		y0 := input.TouchCoord(pipCenterY * pixelToTuxelY)
		x1 := input.TouchCoord((pipCenterX + float64(dir.x*dispW/3)) * pixelToTuxelX)
		y1 := input.TouchCoord((pipCenterY + float64(dir.y*dispH/3)) * pixelToTuxelY)

		testing.ContextLogf(ctx, "Running swipe gesture from {%d,%d} to {%d,%d}", x0, y0, x1, y1)
		if err := stw.Swipe(ctx, x0, y0, x1, y1, 200*time.Millisecond); err != nil {
			return errors.Wrap(err, "failed to execute a swipe gesture")
		}
		if err := stw.End(); err != nil {
			return errors.Wrap(err, "failed to finish the swipe gesture")
		}

		// TODO(b/131248000): WaitForIdle doesn't catch all PIP possible animations, like fling.
		// Add temporary delay until it gets fixed.
		if err := testing.Sleep(ctx, 500*time.Millisecond); err != nil {
			return err
		}
		if err := act.WaitForIdle(ctx, 5*time.Second); err != nil {
			return err
		}

		// After swipe, check that the PIP window arrived to destination.
		bounds, err = act.WindowBounds(ctx)
		if err != nil {
			return errors.Wrap(err, "could not get PIP window bounds after swipe")
		}
		switch dir.border {
		case left:
			if err := checkRange("left bounds", bounds.Left,
				0, collisionWindowWorkAreaInsetsPX); err != nil {
				return errors.Wrap(err, "failed swipe to left")
			}
		case right:
			if err := checkRange("right bounds", bounds.Left+bounds.Width,
				dispW-collisionWindowWorkAreaInsetsPX, dispW-1); err != nil {
				return errors.Wrap(err, "failed swipe to right")
			}
		case top:
			if err := checkRange("top bounds", bounds.Top,
				0, collisionWindowWorkAreaInsetsPX); err != nil {
				return errors.Wrap(err, "failed swipe to top")
			}
		case bottom:
			rectDP, err := getShelfRect(ctx, tconn)
			if err != nil {
				return errors.Wrap(err, "failed to get shelf rect")
			}
			shelfTopPX := int(math.Round(float64(rectDP.Top) * dispMode.DeviceScaleFactor))
			if err := checkRange("bottom bounds", bounds.Top+bounds.Height,
				shelfTopPX-collisionWindowWorkAreaInsetsPX, shelfTopPX-1); err != nil {
				return errors.Wrap(err, "failed swipe to bottom")
			}
		}
	}
	return nil
}

// testPIPGravityStatusArea tests that PIP windows moves accordingly when the status area is hidden / displayed.
func testPIPGravityStatusArea(ctx context.Context, tconn *chrome.Conn, act *arc.Activity, dev *ui.Device, dispMode *display.DisplayMode) error {
	// testPIPGravityStatusArea verifies that:
	// 1) The PIP window moves to the left of the status area when it is shown.
	// 2) The PIP window returns close the right border when the status area is dismissed.

	collisionWindowWorkAreaInsetsPX := int(math.Round(collisionWindowWorkAreaInsetsDP * dispMode.DeviceScaleFactor))
	testing.ContextLog(ctx, "Using: collisionWindowWorkAreaInsetsPX = ", collisionWindowWorkAreaInsetsPX)

	// 0) Sanity check. Verify that PIP window is in the expected initial position and that Status Area is hidden.

	testing.ContextLog(ctx, "Hiding system status area")
	if err := hideSystemStatusArea(ctx, tconn); err != nil {
		return err
	}

	bounds, err := act.WindowBounds(ctx)
	if err != nil {
		return errors.Wrap(err, "could not get PIP window bounds")
	}
	testing.ContextLogf(ctx, "Initial bounds: %+v", bounds)

	if err := checkRange("right bounds", bounds.Left+bounds.Width,
		dispMode.WidthInNativePixels-collisionWindowWorkAreaInsetsPX, dispMode.WidthInNativePixels-1); err != nil {
		return err
	}

	// A newly launched ARC++ PIP window has "no gravity". We move it a few pixels to the top
	// to activate "right" gravity.
	const pixelsToMove = 100
	dst := bounds.Top - pixelsToMove
	if err := act.MoveWindow(ctx, arc.NewPoint(bounds.Left, dst), time.Second); err != nil {
		return errors.Wrapf(err, "failed to move PIP window to (%d, %d)", bounds.Left, dst)
	}

	// 1) The PIP window should move to the left of the status area.

	testing.ContextLog(ctx, "Showing system status area")
	if err := showSystemStatusArea(ctx, tconn); err != nil {
		return err
	}
	// Be nice, and no matter what happens, hide the Status Area on exit.
	defer hideSystemStatusArea(ctx, tconn)

	if err := act.WaitForIdle(ctx, 5*time.Second); err != nil {
		return err
	}

	statusRectDP, err := getStatusAreaRect(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get system status area rect")
	}
	statusLeftPX := int(math.Round(float64(statusRectDP.Left) * dispMode.DeviceScaleFactor))

	bounds, err = act.WindowBounds(ctx)
	if err != nil {
		return errors.Wrap(err, "could not get PIP window bounds")
	}

	if err := checkRange("right bounds", bounds.Left+bounds.Width,
		statusLeftPX-collisionWindowWorkAreaInsetsPX, statusLeftPX-1); err != nil {
		return err
	}

	// 2) The PIP window should move close the right border when the status area is dismissed.

	testing.ContextLog(ctx, "Dismissing system status area")
	if err := hideSystemStatusArea(ctx, tconn); err != nil {
		return err
	}

	if err := act.WaitForIdle(ctx, 5*time.Second); err != nil {
		return err
	}

	bounds, err = act.WindowBounds(ctx)
	if err != nil {
		return errors.Wrap(err, "could not get PIP window bounds")
	}
	testing.ContextLogf(ctx, "Bounds after dismissing status area: %+v", bounds)

	return checkRange("right bounds", bounds.Left+bounds.Width,
		dispMode.WidthInNativePixels-collisionWindowWorkAreaInsetsPX, dispMode.WidthInNativePixels-1)
}

// testPIPGravityShelfAutoHide tests that PIP windows moves accordingly when the shelf is hidden / displayed.
func testPIPGravityShelfAutoHide(ctx context.Context, tconn *chrome.Conn, act *arc.Activity, dev *ui.Device, dispMode *display.DisplayMode) error {
	// The test verifies that:
	// 1) PIP window is created on top of the shelf.
	// 2) PIP window does not fall down when the shelf disappears. This is because gravity is "to the right."
	// 3) PIP is moved to bottom/center causing a gravity is "down".
	// 4) The PIP window moves up, staying on top of the shelf, when the shelf appears again.

	dispInfo, err := display.GetInternalInfo(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get internal display info")
	}

	shelfRectDP, err := getShelfRect(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "could not get shelf rect")
	}

	shelfTopPX := int(math.Round(float64(shelfRectDP.Top) * dispMode.DeviceScaleFactor))
	testing.ContextLog(ctx, "Shelf Top = ", shelfTopPX)

	// 1) PIP window should be above the shelf.

	origBounds, err := act.WindowBounds(ctx)
	if err != nil {
		return errors.Wrap(err, "could not get PIP window bounds")
	}
	testing.ContextLogf(ctx, "Initial bounds: %+v", origBounds)

	collisionWindowWorkAreaInsetsPX := int(math.Round(collisionWindowWorkAreaInsetsDP * dispMode.DeviceScaleFactor))
	testing.ContextLog(ctx, "Using: collisionWindowWorkAreaInsetsPX = ", collisionWindowWorkAreaInsetsPX)

	if err := checkRange("bottom bounds", origBounds.Top+origBounds.Height,
		shelfTopPX-collisionWindowWorkAreaInsetsPX, shelfTopPX-1); err != nil {
		return err
	}

	// 2) PIP window should not fall down when the shelf disappears. Since by default it is "gravity-less".

	testing.ContextLogf(ctx, "Setting shelf auto hide = %q", ash.ShelfBehaviorAlwaysAutoHide)
	if err := ash.SetShelfBehavior(ctx, tconn, dispInfo.ID, ash.ShelfBehaviorAlwaysAutoHide); err != nil {
		return errors.Wrapf(err, "failed to set shelf behavior to %q", ash.ShelfBehaviorAlwaysAutoHide)
	}
	// On exit restore to NeverAutoHide no matter what.
	defer ash.SetShelfBehavior(ctx, tconn, dispInfo.ID, ash.ShelfBehaviorNeverAutoHide)

	if err := act.WaitForIdle(ctx, 5*time.Second); err != nil {
		return err
	}

	bounds, err := act.WindowBounds(ctx)
	if err != nil {
		return errors.Wrap(err, "could not get PIP window bounds")
	}
	testing.ContextLogf(ctx, "Bounds after shelf disappeared: %+v", bounds)

	origBoundsBottom := origBounds.Top + origBounds.Height
	if err := checkRange("bottom bounds", bounds.Top+bounds.Height,
		origBoundsBottom-pipPositionErrorMarginPX, origBoundsBottom+pipPositionErrorMarginPX); err != nil {
		return err
	}

	// 3) PIP should fall-down ('down' gravity) after being moved to the center of the screen.

	// Set shelf to visible again.
	if err := ash.SetShelfBehavior(ctx, tconn, dispInfo.ID, ash.ShelfBehaviorNeverAutoHide); err != nil {
		return errors.Wrapf(err, "failed to set shelf behavior to %q", ash.ShelfBehaviorNeverAutoHide)
	}
	newX := dispMode.WidthInNativePixels / 2
	testing.ContextLogf(ctx, "Moving PIP to (%d, %d)", newX, bounds.Top)
	if err := act.MoveWindow(ctx, arc.NewPoint(dispMode.WidthInNativePixels/2, bounds.Top), 2*time.Second); err != nil {
		return errors.Wrapf(err, "failed to move PIP window to (%d, %d)", newX, bounds.Top)
	}

	if err := act.WaitForIdle(ctx, 5*time.Second); err != nil {
		return err
	}

	// Set shelf to auto-hide again, causing the PIP window to fall down.
	if err := ash.SetShelfBehavior(ctx, tconn, dispInfo.ID, ash.ShelfBehaviorAlwaysAutoHide); err != nil {
		return errors.Wrapf(err, "failed to set shelf behavior to %q", ash.ShelfBehaviorNeverAutoHide)
	}

	bounds, err = act.WindowBounds(ctx)
	if err != nil {
		return errors.Wrap(err, "could not get PIP window bounds")
	}
	testing.ContextLogf(ctx, "Bounds after shelf disappeared: %+v", bounds)

	if err := checkRange("bottom bounds", bounds.Top+bounds.Height,
		dispMode.HeightInNativePixels-collisionWindowWorkAreaInsetsPX, dispMode.HeightInNativePixels-1); err != nil {
		return err
	}

	// 4) PIP window should go up when the shelf reappears.

	testing.ContextLogf(ctx, "Setting shelf auto hide = %q", ash.ShelfBehaviorNeverAutoHide)
	if err := ash.SetShelfBehavior(ctx, tconn, dispInfo.ID, ash.ShelfBehaviorNeverAutoHide); err != nil {
		return errors.Wrapf(err, "failed to set shelf behavior to %q", ash.ShelfBehaviorNeverAutoHide)
	}

	if err := act.WaitForIdle(ctx, 5*time.Second); err != nil {
		return err
	}

	bounds, err = act.WindowBounds(ctx)
	if err != nil {
		return errors.Wrap(err, "could not get PIP window bounds")
	}
	testing.ContextLogf(ctx, "Bounds after shelf appeared: %+v", bounds)

	return checkRange("bottom bounds", bounds.Top+bounds.Height,
		origBoundsBottom-pipPositionErrorMarginPX, origBoundsBottom+pipPositionErrorMarginPX)
}

// testPIPToggleTabletMode verifies that the window position is the same after toggling tablet mode.
func testPIPToggleTabletMode(ctx context.Context, tconn *chrome.Conn, act *arc.Activity, dev *ui.Device, dispMode *display.DisplayMode) error {
	origBounds, err := act.WindowBounds(ctx)
	if err != nil {
		return errors.Wrap(err, "could not get window bounds")
	}
	testing.ContextLogf(ctx, "Initial bounds: %+v", origBounds)

	tabletEnabled, err := ash.TabletModeEnabled(ctx, tconn)
	if err != nil {
		return errors.New("failed to get whether tablet mode is enabled")
	}
	defer ash.SetTabletModeEnabled(ctx, tconn, tabletEnabled)

	testing.ContextLogf(ctx, "Setting 'tablet mode enabled = %t'", !tabletEnabled)
	if err := ash.SetTabletModeEnabled(ctx, tconn, !tabletEnabled); err != nil {
		return errors.New("failed to set tablet mode")
	}

	if err := act.WaitForIdle(ctx, 5*time.Second); err != nil {
		return err
	}

	bounds, err := act.WindowBounds(ctx)
	testing.ContextLogf(ctx, "Bounds after toggling tablet mode: %+v", origBounds)

	if origBounds != bounds {
		return errors.Errorf("invalid position %+v; want %+v", origBounds, bounds)
	}
	return nil
}

// helper functions

// getInternalDisplayMode returns the display mode that is currently selected in the internal display.
func getInternalDisplayMode(ctx context.Context, tconn *chrome.Conn) (*display.DisplayMode, error) {
	dispInfo, err := display.GetInternalInfo(ctx, tconn)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get internal display info")
	}
	for _, mode := range dispInfo.Modes {
		if mode.IsSelected {
			return mode, nil
		}
	}
	return nil, errors.New("failed to get selected mode")
}

// getShelfRect returns Chrome OS's shelf rect, in DPs.
func getShelfRect(ctx context.Context, tconn *chrome.Conn) (arc.Rect, error) {
	var r arc.Rect
	err := tconn.EvalPromise(ctx,
		`new Promise(function(resolve, reject) {
		  chrome.automation.getDesktop(function(root) {
		    const appWindow = root.find({attributes: {className: 'ShelfWidget'}});
		    if (!appWindow) {
		      reject(new Error("Failed to locate ShelfWidget"));
		    } else {
		      resolve(appWindow.location);
		    }
		  })
		})`, &r)
	return r, err
}

// getStatusAreaRect returns Chrome OS's Status Area rect, in DPs.
// Returns error if Status Area is not present.
func getStatusAreaRect(ctx context.Context, tconn *chrome.Conn) (arc.Rect, error) {
	var r arc.Rect
	err := tconn.EvalPromise(ctx,
		`new Promise(function(resolve, reject) {
		  chrome.automation.getDesktop(function(root) {
		    const appWindow = root.find({attributes: {className: 'BubbleFrameView'}});
		    if (!appWindow) {
		      reject(new Error("Failed to locate BubbleFrameView"));
		    } else {
		      resolve(appWindow.location);
		    }
		  })
		})`, &r)
	return r, err
}

// showSystemStatusArea shows the System Status Area in case it is not already shown.
func showSystemStatusArea(ctx context.Context, tconn *chrome.Conn) error {
	// Already visible ?
	if _, err := getStatusAreaRect(ctx, tconn); err == nil {
		return nil
	}

	if err := toggleSystemStatusArea(ctx, tconn); err != nil {
		return err
	}

	// Verify that it is visible.
	if _, err := getStatusAreaRect(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to show the Status Area")
	}
	return nil
}

// hideSystemStatusArea hides the System Status Area in case it is not already hidden.
func hideSystemStatusArea(ctx context.Context, tconn *chrome.Conn) error {
	// Already hidden ?
	if _, err := getStatusAreaRect(ctx, tconn); err != nil {
		return nil
	}

	if err := toggleSystemStatusArea(ctx, tconn); err != nil {
		return err
	}

	// Verify that it is hidden.
	if _, err := getStatusAreaRect(ctx, tconn); err == nil {
		return errors.New("failed to hide the Status Area")
	}
	return nil
}

// toggleSystemStatusArea toggles Chrome OS's system status area.
func toggleSystemStatusArea(ctx context.Context, tconn *chrome.Conn) error {
	// A reliable way to toggle the status area is by injecting Alt+Shift+s. But on tablet mode
	// it doesn't work since the keyboard is disabled.
	// Instead, we inject a touch event in the StatusAreaWidget's button. The problem is that in tablet mode
	// there are two buttons and we cannot identify them in a reliable way. We assume that the first button
	// in the StatusAreaWidget hierarchy is the one that toggles the status area.
	// TODO(ricardoq): Find a reliable way to find "status tray" button.

	var r arc.Rect
	err := tconn.EvalPromise(ctx,
		`new Promise((resolve, reject) => {
		  chrome.automation.getDesktop(function(root) {
		    const areaWidget = root.find({ attributes: { className: 'StatusAreaWidget'}});
		    if (!areaWidget) {
		      reject("Failed to locate StatusAreaWidget");
		      return;
		    }
		    const button = areaWidget.find({ attributes: { role: 'button'}})
		    if (!button) {
		      reject("Failed to locate button in StatusAreaWidget");
		      return;
		    }
		    resolve(button.location);
		  })
		})`, &r)
	if err != nil {
		return errors.Wrap(err, "failed to find StatusAreaWidget")
	}

	tsw, err := input.Touchscreen(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to open touchscreen device")
	}
	defer tsw.Close()

	stw, err := tsw.NewSingleTouchWriter()
	if err != nil {
		return errors.Wrap(err, "could not create TouchEventWriter")
	}
	defer stw.Close()

	dispMode, err := getInternalDisplayMode(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get display mode")
	}

	// Inject touch at button's center. Coordinates coming from Chrome are in DPs.
	x := float64(r.Left+r.Width/2) * dispMode.DeviceScaleFactor
	y := float64(r.Top+r.Height/2) * dispMode.DeviceScaleFactor

	// Calculate Pixel (screen display) / Tuxel (touch device) ratio.
	pixelToTuxelX := float64(tsw.Width()) / float64(dispMode.WidthInNativePixels)
	pixelToTuxelY := float64(tsw.Height()) / float64(dispMode.HeightInNativePixels)

	if err := stw.Move(input.TouchCoord(x*pixelToTuxelX), input.TouchCoord(y*pixelToTuxelY)); err != nil {
		return err
	}
	const touchDuration = 100 * time.Millisecond
	if err := testing.Sleep(ctx, touchDuration); err != nil {
		return err
	}
	return stw.End()
}

// checkRange checks whether value is in the range of [min, max]
func checkRange(name string, value, min, max int) error {
	if value < min || value > max {
		return errors.Errorf("%s (%d) not in the expected range [%d, %d]", name, value, min, max)
	}
	return nil
}
