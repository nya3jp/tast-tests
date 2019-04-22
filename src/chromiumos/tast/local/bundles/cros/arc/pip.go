// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
)

const (
	// kPipWorkAreaInsetsDP is hardcoded to 8dp. Using a bigger value to safe.
	// See: https://cs.chromium.org/chromium/src/ash/wm/pip/pip_positioner.cc
	// TODO(crbug.com/949754): Get this value in runtime.
	pipWorkAreaInsetsDP = 8 + 5

	// pipPositionErrorMarginPX represents the error margin in pixels when comparing positions. See b/129976114 for more info.
	// TODO(ricardoq): Remove this constant once the bug gets fixed.
	pipPositionErrorMarginPX = 1
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PIP,
		Desc:         "Checks that ARC++ Picture-in-Picture works as expected",
		Contacts:     []string{"ricardoq@chromium.org", "edcourtney@chromium.org", "arc-eng@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"tablet_mode", "android_p", "chrome_login"},
		Timeout:      5 * time.Minute,
		Data:         []string{"ArcPipTastTest.apk"},
	})
}

func PIP(ctx context.Context, s *testing.State) {
	must := func(err error) {
		if err != nil {
			s.Fatal("Failed: ", err)
		}
	}

	// For debugging, add chrome.ExtraArgs("--show-taps")
	cr, err := chrome.New(ctx, chrome.ARCEnabled(), chrome.ExtraArgs("--show-taps"))

	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close()

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

	s.Log("pre - getShelfAlignment")
	origShelfAlignment, err := getShelfAlignment(ctx, tconn, dispInfo.ID)
	if err != nil {
		s.Fatal("Failed to get shelf alignment: ", err)
	}
	s.Log("post - getShelfAlignment: ", origShelfAlignment)

	s.Log("pre - setShelfAlignment")
	if err := setShelfAlignment(ctx, tconn, dispInfo.ID, shelfAlignmentBottom); err != nil {
		s.Fatal("Failed to set shelf alignment to Bottom: ", err)
	}
	// Be nice and restore shelf alignment to its original state on exit.
	defer setShelfAlignment(ctx, tconn, dispInfo.ID, origShelfAlignment)

	s.Log("pre - getShelfBehavior")
	origShelfBehavior, err := getShelfBehavior(ctx, tconn, dispInfo.ID)
	if err != nil {
		s.Fatal("Failed to get shelf behavior: ", err)
	}
	s.Log("post - getShelfBehavior: ", origShelfBehavior)
	s.Log("pre - setShelfBehavior")
	if err := setShelfBehavior(ctx, tconn, dispInfo.ID, shelfBehaviorNeverAutoHide); err != nil {
		s.Fatal("Failed to set shelf behavior to Never Auto Hide: ", err)
	}
	// Be nice and restore shelf behavior to its original state on exit.
	defer setShelfBehavior(ctx, tconn, dispInfo.ID, origShelfBehavior)

	s.Log("pre - isTabletModeEnabled")
	tabletModeEnabled, err := isTabletModeEnabled(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get tablet mode: ", err)
	}
	// Be nice and restore tablet mode to its original state on exit.
	defer setTabletModeEnabled(ctx, tconn, tabletModeEnabled)

	dispMode, err := getInternalDisplayMode(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get display mode: ", err)
	}

	// Run all subtests twice. First, with tablet mode disabled. And then, with it enabled.
	for _, tabletMode := range []bool{false, true} {
		s.Logf("Running tests with tablet mode enabled=%t", tabletMode)
		if err := setTabletModeEnabled(ctx, tconn, tabletMode); err != nil {
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
			a.Command(ctx, "am", "broadcast", "-a", "android.intent.action.arc.cleartaskstate")
			must(act.Start(ctx))
			must(act.WaitForIdle(ctx, time.Second))
			// Press button that triggers PIP mode in activity.
			const pipButtonID = pkgName + ":id/enter_pip"
			must(dev.Object(ui.ID(pipButtonID)).Click(ctx))
			must(act.WaitForIdle(ctx, time.Second))

			if err := test.fn(ctx, tconn, act, dev, dispMode); err != nil {
				path := fmt.Sprintf("%s/screenshot-pip-failed-test-%d.png", s.OutDir(), idx)
				if err := screenshot.CaptureChrome(ctx, cr, path); err != nil {
					s.Log("Failed to capture screenshot: ", err)
				}
				s.Errorf("%q + tablet mode(%t) failed: %v", test.name, tabletMode, err)
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
		if err := act.WaitForIdle(ctx, time.Second); err != nil {
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
	if err := act.WaitForIdle(ctx, time.Second); err != nil {
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

	w := bounds.Right - bounds.Left
	h := bounds.Bottom - bounds.Top

	// Aspect ratio gets honored after resize. Only test one dimension.
	if pipMaxSizeH < pipMaxSizeW {
		if pipMaxSizeH-h > pipPositionErrorMarginPX {
			return errors.Wrapf(err, "invalid height %d; want %d (error margin is %d)", h, pipMaxSizeH, pipPositionErrorMarginPX)
		}
	} else {
		if pipMaxSizeW-w > pipPositionErrorMarginPX {
			return errors.Wrapf(err, "invalid width %d; want %d (error margin is %d)", w, pipMaxSizeW, pipPositionErrorMarginPX)
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

	pipWorkAreaInsetsPX := int(math.Round(pipWorkAreaInsetsDP * dispMode.DeviceScaleFactor))
	testing.ContextLog(ctx, "Using: pipWorkAreaInsetsPX = ", pipWorkAreaInsetsPX)

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
		if err := testing.Sleep(ctx, 500*time.Millisecond); err != nil {
			return err
		}
		bounds, err := act.WindowBounds(ctx)
		if err != nil {
			return errors.Wrap(err, "could not get PIP window bounds")
		}

		pipCenterX := float64(bounds.Left + (bounds.Right-bounds.Left)/2)
		pipCenterY := float64(bounds.Top + (bounds.Bottom-bounds.Top)/2)

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

		if err := act.WaitForIdle(ctx, time.Second); err != nil {
			return err
		}

		// After swipe, check that the PIP window arrived to destination.
		bounds, err = act.WindowBounds(ctx)
		if err != nil {
			return errors.Wrap(err, "could not get PIP window bounds after swipe")
		}
		switch dir.border {
		case left:
			if bounds.Left < 0 || bounds.Left > pipWorkAreaInsetsPX {
				return errors.Errorf("failed swipe to left; expected bounds.Left 0 <= %d <= %d",
					bounds.Left, pipWorkAreaInsetsPX)
			}
		case right:
			if bounds.Right > dispW || dispW-bounds.Right > pipWorkAreaInsetsPX {
				return errors.Errorf("failed swipe to right; expected bounds.Right %d <= %d <= %d",
					dispW-pipWorkAreaInsetsPX, bounds.Right, dispW)
			}
		case top:
			if bounds.Top < 0 || bounds.Top > pipWorkAreaInsetsPX {
				return errors.Errorf("failed swipe to top; expected bounds.Top 0 <= %d <= %d",
					bounds.Top, pipWorkAreaInsetsPX)
			}
		case bottom:
			rectDP, err := getShelfRect(ctx, tconn)
			if err != nil {
				return errors.Wrap(err, "failed to get shelf rect")
			}
			shelfTopPX := int(math.Round(float64(rectDP.Top) * dispMode.DeviceScaleFactor))
			if bounds.Bottom >= shelfTopPX || shelfTopPX-bounds.Bottom > pipWorkAreaInsetsPX {
				return errors.Errorf("failed swipe to bottom; expected bounds.Bottom %d <= %d < %d",
					bounds.Bottom-pipWorkAreaInsetsPX, bounds.Bottom, shelfTopPX)
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
	//    This is because the gravity is "to the right".

	pipWorkAreaInsetsPX := int(pipWorkAreaInsetsDP * dispMode.DeviceScaleFactor)
	testing.ContextLog(ctx, "Using: pipWorkAreaInsetsPX = ", pipWorkAreaInsetsPX)

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

	min := dispMode.WidthInNativePixels - pipWorkAreaInsetsPX
	max := dispMode.WidthInNativePixels
	if bounds.Right < min || bounds.Right >= max {
		return errors.Errorf("invalid right bounds %d; want %d <= %d < %d",
			min, bounds.Right, max)
	}

	// 1) The PIP window moves to the left of the status area when it is shown.

	testing.ContextLog(ctx, "Showing system status area")
	if err := showSystemStatusArea(ctx, tconn); err != nil {
		return err
	}
	// Be nice, and no matter what happens, hide the Status Area on exit.
	defer hideSystemStatusArea(ctx, tconn)

	if err := act.WaitForIdle(ctx, time.Second); err != nil {
		return err
	}

	statusRectDP, err := getStatusAreaRect(ctx, tconn)
	statusLeftPX := int(float64(statusRectDP.Left) * dispMode.DeviceScaleFactor)
	if err != nil {
		return errors.Wrap(err, "failed to get system status area rect")
	}

	bounds, err = act.WindowBounds(ctx)
	if err != nil {
		return errors.Wrap(err, "could not get PIP window bounds")
	}

	min = statusLeftPX - pipWorkAreaInsetsPX
	max = statusLeftPX
	if bounds.Right < min || bounds.Right >= max {
		return errors.Errorf("invalid right bounds; want %d <= %d < %d",
			min, bounds.Right, max)
	}

	// 2) The PIP window returns close the right border when the status area is dismissed.

	testing.ContextLog(ctx, "Dismissing system status area")
	if err := hideSystemStatusArea(ctx, tconn); err != nil {
		return err
	}

	if err := act.WaitForIdle(ctx, time.Second); err != nil {
		return err
	}

	bounds, err = act.WindowBounds(ctx)
	if err != nil {
		return errors.Wrap(err, "could not get PIP window bounds")
	}
	testing.ContextLogf(ctx, "Bounds after dismissing status area: %+v", bounds)

	if bounds.Right < dispMode.WidthInNativePixels-pipWorkAreaInsetsPX {
		return errors.Errorf("failed to return to right border; expected right bounds %d > %d",
			bounds.Right, dispMode.WidthInNativePixels-pipWorkAreaInsetsPX)
	}
	return nil
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

	shelfTopPX := int(float64(shelfRectDP.Top) * dispMode.DeviceScaleFactor)
	testing.ContextLog(ctx, "Shelf Top is = ", shelfTopPX)

	// 1) PIP window: on top of shelf.

	origBounds, err := act.WindowBounds(ctx)
	if err != nil {
		return errors.Wrap(err, "could not get PIP window bounds")
	}
	testing.ContextLogf(ctx, "Initial bounds: %+v", origBounds)
	pipInsetsPX := int(pipWorkAreaInsetsDP * dispMode.DeviceScaleFactor)

	if shelfTopPX-origBounds.Bottom < 0 || shelfTopPX-origBounds.Bottom > pipInsetsPX {
		return errors.Errorf("unexpected initial bounds: %+v; expected: %d <= %d <= %d",
			origBounds, origBounds.Bottom, shelfTopPX, origBounds.Bottom+pipInsetsPX)
	}

	// 2) PIP window does not fall down when the shelf disappears.

	testing.ContextLogf(ctx, "Setting shelf auto hide = %q", shelfBehaviorAlwaysAutoHide)
	if err := setShelfBehavior(ctx, tconn, dispInfo.ID, shelfBehaviorAlwaysAutoHide); err != nil {
		return errors.Wrap(err, "failed to set shelf behavior")
	}
	// On exit restore to NeverAutoHide no matter what.
	defer setShelfBehavior(ctx, tconn, dispInfo.ID, shelfBehaviorNeverAutoHide)

	if err := act.WaitForIdle(ctx, time.Second); err != nil {
		return err
	}

	bounds, err := act.WindowBounds(ctx)
	if err != nil {
		return errors.Wrap(err, "could not get PIP window bounds")
	}
	testing.ContextLogf(ctx, "Bounds after shelf disappeared: %+v", bounds)

	if math.Abs(float64(bounds.Bottom-origBounds.Bottom)) > pipPositionErrorMarginPX {
		return errors.Errorf("expected bounds %+v, actual %+v", origBounds, bounds)
	}

	// 3) PIP is moved to bottom/center causing a "down" gravity.

	newX := dispMode.WidthInNativePixels / 2
	testing.ContextLogf(ctx, "Moving PIP to %d,%d", newX, bounds.Top)
	if err := act.MoveWindow(ctx, arc.NewPoint(dispMode.WidthInNativePixels/2, bounds.Top), 2*time.Second); err != nil {
		return errors.Wrapf(err, "failed to move PIP window to %d,%d", newX, bounds.Top)
	}

	if err := act.WaitForIdle(ctx, time.Second); err != nil {
		return err
	}

	bounds, err = act.WindowBounds(ctx)
	if err != nil {
		return errors.Wrap(err, "could not get PIP window bounds")
	}
	testing.ContextLogf(ctx, "Bounds after activity got restarted: %+v", bounds)

	pipWorkAreaInsetsPX := int(pipWorkAreaInsetsDP * dispMode.DeviceScaleFactor)
	testing.ContextLog(ctx, "Using: pipWorkAreaInsetsPX = ", pipWorkAreaInsetsPX)

	if bounds.Bottom+pipWorkAreaInsetsPX < dispMode.HeightInNativePixels {
		return errors.Errorf("invalid PIP bounds %+v; expected %d > %d",
			bounds, bounds.Bottom+pipWorkAreaInsetsPX, dispMode.HeightInNativePixels)
	}

	// 4) PIP window should go up when the shelf appears

	testing.ContextLogf(ctx, "Setting shelf auto hide = %q", shelfBehaviorNeverAutoHide)
	if err := setShelfBehavior(ctx, tconn, dispInfo.ID, shelfBehaviorNeverAutoHide); err != nil {
		return errors.Wrap(err, "failed to set shelf behavior")
	}

	if err := act.WaitForIdle(ctx, time.Second); err != nil {
		return err
	}

	bounds, err = act.WindowBounds(ctx)
	if err != nil {
		return errors.Wrap(err, "could not get PIP window bounds")
	}
	testing.ContextLogf(ctx, "Bounds after shelf appeared: %+v", bounds)

	if math.Abs(float64(bounds.Bottom-origBounds.Bottom)) > pipPositionErrorMarginPX {
		return errors.Errorf("expected bounds: %+v, actual: %+v", origBounds, bounds)
	}

	return nil
}

// testPIPToggleTabletMode verifies that the window position is the same after toggling tablet mode.
func testPIPToggleTabletMode(ctx context.Context, tconn *chrome.Conn, act *arc.Activity, dev *ui.Device, dispMode *display.DisplayMode) error {
	origBounds, err := act.WindowBounds(ctx)
	if err != nil {
		return errors.Wrap(err, "could not get window bounds")
	}
	testing.ContextLogf(ctx, "Initial bounds: %+v", origBounds)

	tabletEnabled, err := isTabletModeEnabled(ctx, tconn)
	if err != nil {
		return errors.New("failed to get whether tablet mode is enabled")
	}
	defer setTabletModeEnabled(ctx, tconn, tabletEnabled)

	testing.ContextLogf(ctx, "Setting 'tablet mode enabled = %t'", !tabletEnabled)
	if err := setTabletModeEnabled(ctx, tconn, !tabletEnabled); err != nil {
		return errors.New("failed to set tablet mode")
	}

	if err := act.WaitForIdle(ctx, time.Second); err != nil {
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

// setTabletModeEnabled enables / disables tablet mode.
// After calling this function, it won't be possible to physically switch to/from tablet mode since that functionality will be disabled.
func setTabletModeEnabled(ctx context.Context, c *chrome.Conn, enabled bool) error {
	e := strconv.FormatBool(enabled)
	expr := fmt.Sprintf(
		`new Promise(function(resolve, reject) {
		  chrome.autotestPrivate.setTabletModeEnabled(%s, function(enabled) {
		    if (chrome.runtime.lastError) {
		      reject(new Error(chrome.runtime.lastError.message));
		      return;
		    }
		    if (enabled != %s) {
		      reject(new Error("unexpected tablet mode: " + enabled));
		    } else {
		      resolve();
		    }
		  })
		})`, e, e)
	return c.EvalPromise(ctx, expr, nil)
}

// shelfBehavior represents the different Chrome OS shelf behaviors.
type shelfBehavior string

// As defined in ShelfAutoHideBehavior here:
// https://cs.chromium.org/chromium/src/ash/public/cpp/shelf_types.h
const (
	// shelfBehaviorAlwaysAutoHide represents always auto-hide.
	shelfBehaviorAlwaysAutoHide shelfBehavior = "always"
	//shelfBehaviorNeverAutoHide represents never auto-hide, meaning that it is always visible.
	shelfBehaviorNeverAutoHide = "never"
	// shelfBehaviorHidden represents always hidden, used for debugging, since this state is not exposed to the user.
	shelfBehaviorHidden = "hidden"
	// shelfBehaviorInvalid represents an invalid state.
	shelfBehaviorInvalid = "invalid"
)

// setShelfBehavior sets the shelf visibility behavior.
// displayId is the display that contains the shelf.
func setShelfBehavior(ctx context.Context, c *chrome.Conn, displayID string, behavior shelfBehavior) error {
	expr := fmt.Sprintf(
		`new Promise(function(resolve, reject) {
		  chrome.autotestPrivate.setShelfAutoHideBehavior(%q, %q, function() {
		    if (chrome.runtime.lastError) {
		      reject(new Error(chrome.runtime.lastError.message));
		    } else {
		      resolve();
		    }
		  });
		})`, displayID, behavior)
	return c.EvalPromise(ctx, expr, nil)
}

// getShelfBehavior returns the shelf visibility behavior.
// displayId is the display that contains the shelf.
func getShelfBehavior(ctx context.Context, c *chrome.Conn, displayID string) (shelfBehavior, error) {
	var behavior shelfBehavior
	expr := fmt.Sprintf(
		`new Promise(function(resolve, reject) {
		  chrome.autotestPrivate.getShelfAutoHideBehavior(%q, function(behavior) {
		    if (chrome.runtime.lastError) {
		      reject(new Error(chrome.runtime.lastError.message));
		    } else {
		      resolve(behavior);
		    }
		  });
		})`, displayID)
	err := c.EvalPromise(ctx, expr, &behavior)
	if err != nil {
		return shelfBehaviorInvalid, err
	}
	switch behavior {
	case shelfBehaviorAlwaysAutoHide, shelfBehaviorNeverAutoHide, shelfBehaviorHidden:
	default:
		return shelfBehaviorInvalid, errors.Errorf("unsupported shelf value: %q", behavior)
	}
	return behavior, nil
}

// shelfAlignment represents the different Chrome OS shelf alignments.
type shelfAlignment string

// As defined in ShelfAlignment here:
// https://cs.chromium.org/chromium/src/ash/public/cpp/shelf_types.h
const (
	shelfAlignmentBottom       shelfAlignment = "Bottom"
	shelfAlignmentLeft                        = "Left"
	shelfAlignmentRight                       = "Right"
	shelfAlignmentBottomLocked                = "BottomLocked"
	shelfAlignmentInvalid                     = "Invalid"
)

// setShelfAlignment sets the shelf alignment.
// displayId is the display that contains the shelf.
func setShelfAlignment(ctx context.Context, c *chrome.Conn, displayID string, a shelfAlignment) error {
	expr := fmt.Sprintf(
		`new Promise(function(resolve, reject) {
		  chrome.autotestPrivate.setShelfAlignment(%q, %q, function() {
		    if (chrome.runtime.lastError) {
		      reject(new Error(chrome.runtime.lastError.message));
		    } else {
		      resolve();
		    }
		  });
		})`, displayID, a)
	return c.EvalPromise(ctx, expr, nil)
}

// getShelfAlignment returns the shelf alignment.
// displayId is the display that contains the shelf.
func getShelfAlignment(ctx context.Context, c *chrome.Conn, displayID string) (shelfAlignment, error) {
	var a shelfAlignment
	expr := fmt.Sprintf(
		`new Promise(function(resolve, reject) {
		  chrome.autotestPrivate.getShelfAlignment(%q, function(behavior) {
		    if (chrome.runtime.lastError) {
		      reject(new Error(chrome.runtime.lastError.message));
		    } else {
		      resolve(behavior);
		    }
		  });
		})`, displayID)
	err := c.EvalPromise(ctx, expr, &a)
	if err != nil {
		return shelfAlignmentInvalid, err
	}
	switch a {
	case shelfAlignmentBottom, shelfAlignmentLeft, shelfAlignmentRight, shelfAlignmentBottomLocked:
	default:
		return shelfAlignmentInvalid, errors.Errorf("invalid alignment %q", a)
	}
	return a, nil
}

// isTabletModeEnabled gets the tablet mode enabled status.
func isTabletModeEnabled(ctx context.Context, tconn *chrome.Conn) (bool, error) {
	var enabled bool
	err := tconn.EvalPromise(ctx,
		`new Promise(function(resolve, reject) {
		  chrome.autotestPrivate.isTabletModeEnabled(function(enabled) {
		    if (chrome.runtime.lastError) {
		      reject(new Error(chrome.runtime.lastError.message));
		    } else {
		      resolve(enabled);
		    }
		  })
		})`, &enabled)
	return enabled, err
}

// rect represents a rectangle, as defined here:
// https://developers.chrome.com/extensions/automation#type-Rect
type rect struct {
	Left   int `json:"left"`
	Top    int `json:"top"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

// getShelfRect returns Chrome OS's shelf rect, in DPs.
func getShelfRect(ctx context.Context, tconn *chrome.Conn) (rect, error) {
	var r rect
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
func getStatusAreaRect(ctx context.Context, tconn *chrome.Conn) (rect, error) {
	var r rect
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

	var r rect
	err := tconn.EvalPromise(ctx,
		`new Promise((resolve, reject) => {
			chrome.automation.getDesktop(function(root) {
				const areaWidget = root.find({ attributes: { className: 'StatusAreaWidget'}});
				if (!areaWidget) {
					reject("Failed to locate StatusAreaWidget");
				}
				const button = areaWidget.find({ attributes: { role: 'button'}})
				if (!button) {
					reject("Failed to locate button in StatusAreaWidget");
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
