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
	// TODO(ricardoq): Find a way to get hardcoded values in runtime.

	// kTrayMenuWidth is hardcoded to 360dp. Using a smaller value to be safe.
	// See: https://cs.chromium.org/chromium/src/ash/system/tray/tray_constants.cc
	trayMenuWidthDp = 360 - 36
	// kShelfSize is hardcoded to 56dp. Using a smaller size to be safe.
	// See: https://cs.chromium.org/chromium/src/ash/shelf/shelf_constants.h
	shelfSizeDp = 56 - 8
	// kPipWorkAreaInsetsDp is hardcoded to 8dp. Using a bigger value to safe.
	// See: https://cs.chromium.org/chromium/src/ash/wm/pip/pip_positioner.cc
	pipWorkAreaInsetsDp = 8 + 5
	// kPipSnapToEdgeAnimationDurationMs is hardcoded to 150ms. Using bigger value to be safe.
	// See: https://cs.chromium.org/chromium/src/ash/wm/pip/pip_window_resizer.cc
	pipSnapToEdgeAnimationDuration = 250 * time.Millisecond

	// pipMaxSizeNormalized represents the PiP window size compared to the display size,
	// as defined in WindowPosition.getMaximumSizeForPip().
	// See: https://cs.corp.google.com/pi-arcvm-dev/frameworks/base/services/core/arc/java/com/android/server/am/WindowPositioner.java
	pipMaxSizeNormalized = 0.5

	// pipMenuAnimationDuration is the duration it takes to show the PiP Menu activity in ARC++.
	// TODO(ricardoq): Find a reliable number.
	pipMenuAnimationDuration = 300 * time.Millisecond

	// pipPositionErrorMarginPx represents the quantity of pixels that a position could be off.
	// This is needed in devices like Nocturne, which has many off-by-one-pixel bugs.
	// Ignoring the off-by-one-pixel bugs in the PiP tests.
	pipPositionErrorMarginPx = 1
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Pip,
		Desc:         "Checks that ARC++ Picture-in-Picture works as expected.",
		Contacts:     []string{"ricardoq@chromium.org", "edcourtney@chromium.org", "arc-eng@google.com"},
		Attr:         []string{"disabled", "informational"},
		SoftwareDeps: []string{"touch_view", "android", "android_p", "chrome_login"},
		Timeout:      5 * time.Minute,
		Data:         []string{"piptest.apk"},
	})
}

func Pip(ctx context.Context, s *testing.State) {
	const (
		apkName = "piptest.apk"
		pkgName = "com.example.edcourtney.pictureinpicturetest"
		actName = ".MainActivity"

		idPrefix = pkgName + ":id/"
		fabID    = idPrefix + "fab"
	)

	must := func(err error) {
		if err != nil {
			s.Fatal("Failed: ", err)
		}
	}

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

	s.Log("Installing APK")
	if err := a.Install(ctx, s.DataPath(apkName)); err != nil {
		s.Fatal("Failed installing app: ", err)
	}

	act, err := arc.NewActivity(a, pkgName, actName)
	if err != nil {
		s.Fatal("Failed to create new activity: ", err)
	}
	defer act.Close()

	d, err := ui.NewDevice(ctx, a)
	if err != nil {
		s.Fatal("Failed initializing UI automator device: ", err)
	}
	defer d.Close()
	d.EnableDebug()

	tabletModeEnabled, err := isTabletModeEnabled(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get tablet mode: ", err)
	}
	// Be nice and, on exit, restore tablet mode to its original state.
	defer setTabletModeEnabled(ctx, tconn, tabletModeEnabled)

	firstTime := true

	// Run all subtests twice. Firstly, all subtests with tablet mode disabled. And then, with it enabled.
	for _, tabletMode := range []bool{false, true} {

		testing.ContextLogf(ctx, "Setting tablet mode enabled to %t", tabletMode)
		tm, err := isTabletModeEnabled(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to get tablet mode: ", err)
		}
		if tm != tabletMode {
			if err := setTabletModeEnabled(ctx, tconn, tabletMode); err != nil {
				s.Fatalf("Failed to set tablet mode enabled to %t: %v", tabletMode, err)
			}
		}

		type testFunc func(ctx context.Context, cr *chrome.Chrome, tconn *chrome.Conn, act *arc.Activity, d *ui.Device) error
		for idx, entry := range []struct {
			testName string
			fn       testFunc
		}{
			{"PiP Move", testPIPMove},
			{"PiP Resize", testPIPResize},
			{"PiP Fling", testPIPFling},
			{"PIP GravityStatusArea", testPIPGravityStatusArea},
			{"PiP GravityShelfAutoHide", testPIPGravityShelfAutoHide},
			{"PiP ToggleTabletMode", testPIPToggleTabletMode},
		} {
			s.Logf("Running '%s'", entry.testName)

			if firstTime {
				must(act.Start(ctx))
				firstTime = false
			} else {
				must(act.Stop(ctx))
				// Clear task WM state. Window should be positioned at their default location.
				a.Command(ctx, "am", "broadcast", "-a", "android.intent.action.arc.cleartaskstate")
				must(act.Start(ctx))
			}
			waitForPIPReady(ctx, time.Second)
			// Press button that triggers PiP mode in activity.
			must(d.Object(ui.ID(fabID)).Click(ctx))
			waitForPIPReady(ctx, time.Second)

			if err := entry.fn(ctx, cr, tconn, act, d); err != nil {
				path := fmt.Sprintf("%s/screenshot-pip-failed-test-%d.png", s.OutDir(), idx)
				if err := screenshot.CaptureChrome(ctx, cr, path); err != nil {
					s.Log("Failed to capture screenshot: ", err)
				}
				s.Errorf("'%s' + tablet mode(%t) failed: %v", entry.testName, tabletMode, err)
			}
		}
	}
}

// testPIPMove verifies that drag-moving the PiP window works as expected.
// It does that by drag-moving that PiP window horizontally 3 times and verifying that the position is correct.
func testPIPMove(ctx context.Context, cr *chrome.Chrome, tconn *chrome.Conn, act *arc.Activity, d *ui.Device) error {
	// When the drag-move sequence is started, the gesture controller might miss a few pixels before it finally
	// recognizes it as a drag-move gesture. This is specially true for PiP windows.
	// The value varies depending on acceleration/speed of the gesture. 35 works for our purpose.
	const missedByGestureControllerDp = 35

	dispMode, err := getInternalDisplayMode(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get display mode")
	}

	missedByGestureControllerPx := dispMode.DeviceScaleFactor * missedByGestureControllerDp
	testing.ContextLog(ctx, "Using: missedByGestureControllerPx = ", missedByGestureControllerPx)

	bounds, err := act.WindowBounds(ctx)
	if err != nil {
		return errors.Wrap(err, "could not get PiP window bounds")
	}
	testing.ContextLogf(ctx, "Initial PiP bounds: %+v", bounds)

	deltaX := dispMode.WidthInNativePixels / 4
	for i := 0; i < 3; i++ {
		testing.ContextLogf(ctx, "Moving PiP window to %d,%d", bounds.Left-deltaX, bounds.Top)
		if err := act.MoveWindow(ctx, arc.NewPoint(bounds.Left-deltaX, bounds.Top), 2*time.Second); err != nil {
			return errors.Wrap(err, "could not move Pip window")
		}
		if err := waitForPIPReady(ctx, time.Second); err != nil {
			return err
		}

		newBounds, err := act.WindowBounds(ctx)
		if err != nil {
			return errors.Wrap(err, "could not get PiP window bounds")
		}
		testing.ContextLogf(ctx, "PiP bounds after move: %+v", newBounds)

		diff := math.Abs(float64(bounds.Left - deltaX - newBounds.Left))
		if diff > missedByGestureControllerPx {
			return errors.Wrapf(err, "invalid PiP bounds: %+v; expected %g < %g", bounds, diff, missedByGestureControllerPx)
		}
		bounds = newBounds
	}

	return nil
}

// testPIPResize verifies that resizing the PiP window works as expected.
// It does that by drag-resizing the PiP window from its top-left corner. The new size should be ~25% the display screen size.
func testPIPResize(ctx context.Context, cr *chrome.Chrome, tconn *chrome.Conn, act *arc.Activity, d *ui.Device) error {
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get a Test API connection")
	}

	dispInfo, err := display.GetInternalInfo(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get display info")
	}

	origAutoHide, err := getShelfAutoHideBehavior(ctx, tconn, dispInfo.ID)
	if err != nil {
		return errors.Wrap(err, "could not get shelf auto hide behavior")
	}
	defer setShelfAutoHideBehavior(ctx, tconn, dispInfo.ID, origAutoHide)

	// Hide shelf to make test simpler. Otherwise shelf size needs to be taken into account when calculating max PiP size.
	if err := setShelfAutoHideBehavior(ctx, tconn, dispInfo.ID, "always"); err != nil {
		return errors.Wrap(err, "could not set shelf auto hide behavior to 'always'")
	}

	dispMode, err := getInternalDisplayMode(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get display mode")
	}

	// Activate "resize handler", otherwise resize will fail.
	if err := activatePIPMenuActivity(ctx, d); err != nil {
		return errors.Wrap(err, "could not activate PiP menu")
	}

	bounds, err := act.WindowBounds(ctx)
	if err != nil {
		return errors.Wrap(err, "could not get PiP window bounds")
	}
	testing.ContextLogf(ctx, "Bounds before resize: %+v", bounds)

	testing.ContextLog(ctx, "Resizing window to x=0, y=0")

	// The PiP window should be on the bottom-right corner. Resizing it from the top-left border to its max size.
	if err := act.ResizeWindow(ctx, arc.BorderTopLeft, arc.NewPoint(0, 0), time.Second); err != nil {
		return errors.Wrap(err, "could not move Pip window")
	}

	bounds, err = act.WindowBounds(ctx)
	if err != nil {
		return errors.Wrap(err, "could not get PiP window bounds")
	}
	testing.ContextLogf(ctx, "Bounds after resize: %+v", bounds)

	pipMaxSizeW := float64(dispMode.WidthInNativePixels) * pipMaxSizeNormalized
	pipMaxSizeH := float64(dispMode.HeightInNativePixels) * pipMaxSizeNormalized

	// Give an error margin. Resizing by injecting touches might not be %100 accurate.
	errorMarginW := pipMaxSizeW * 0.01
	errorMarginH := pipMaxSizeH * 0.01

	w := float64(bounds.Right - bounds.Left)
	h := float64(bounds.Bottom - bounds.Top)

	// Aspect ratio gets honored after resize. It is enough to check only one of the dimensions.
	if pipMaxSizeH < pipMaxSizeW {
		if h > pipMaxSizeH || pipMaxSizeH-h > errorMarginH {
			return errors.Wrapf(err, "invalid height; expected %g <= %g <= %g", pipMaxSizeH-errorMarginH, h, pipMaxSizeH)
		}
	} else {
		if w > pipMaxSizeW || pipMaxSizeW-w > errorMarginW {
			return errors.Wrapf(err, "invalid width; expected %g <= %g <= %g", pipMaxSizeW-errorMarginW, w, pipMaxSizeW)
		}
	}
	return nil
}

// testPIPFling tests that the "fling gesture" works as expected in PiP windows.
// It tests the fling in four directions: left, up, right and down.
// Assumes the shelf is visible and placed at the bottom.
func testPIPFling(ctx context.Context, cr *chrome.Chrome, tconn *chrome.Conn, act *arc.Activity, d *ui.Device) error {
	type borderType int
	const (
		// Borders to check after swipe.
		left borderType = iota
		right
		top
		bottom
	)
	const (
		// Steps used for each swipe. Each step takes 5ms. A 200ms swipe
		steps = 200 / 5
	)

	dispMode, err := getInternalDisplayMode(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get display mode")
	}

	pipWorkAreaInsetsPx := int(pipWorkAreaInsetsDp * dispMode.DeviceScaleFactor)
	testing.ContextLog(ctx, "Using: pipWorkAreaInsetsPx = ", pipWorkAreaInsetsPx)

	shelfSizePx := int(shelfSizeDp * dispMode.DeviceScaleFactor)
	testing.ContextLog(ctx, "Using: shelfSizePx = ", shelfSizePx)

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

	// Calculate Pixel (screen display) / Tuxel (touch device) ratio.
	touchW := tsw.Width()
	touchH := tsw.Height()
	dispW := dispMode.WidthInNativePixels
	dispH := dispMode.HeightInNativePixels
	pixelToTuxelX := float64(touchW) / float64(dispW)
	pixelToTuxelY := float64(touchH) / float64(dispH)

	// Used as reference once the 4 swipes are done.
	origBounds, err := act.WindowBounds(ctx)

	for _, dir := range []struct {
		x, y   int
		border borderType
	}{
		{-1, 0, left},  // swipe to left
		{0, -1, top},   // swipe to top
		{1, 0, right},  // swipe to right
		{0, 1, bottom}, // swipe to bottom
	} {
		bounds, err := act.WindowBounds(ctx)
		if err != nil {
			return errors.Wrap(err, "could not get PiP window bounds")
		}

		appCenterX := float64(bounds.Left + (bounds.Right-bounds.Left)/2)
		appCenterY := float64(bounds.Top + (bounds.Bottom-bounds.Top)/2)

		x0 := input.TouchCoord(appCenterX * pixelToTuxelX)
		y0 := input.TouchCoord(appCenterY * pixelToTuxelY)
		x1 := input.TouchCoord((appCenterX + float64(dir.x*dispW/3)) * pixelToTuxelX)
		y1 := input.TouchCoord((appCenterY + float64(dir.y*dispH/3)) * pixelToTuxelY)

		testing.ContextLogf(ctx, "Running swipe gesture from {%d,%d} to {%d,%d}", x0, y0, x1, y1)
		if err := stw.Swipe(ctx, x0, y0, x1, y1, steps); err != nil {
			return errors.Wrap(err, "failed to execute a swipe gesture")
		}
		if err := stw.End(); err != nil {
			return errors.Wrap(err, "failed to finish the swipe gesture")
		}

		select {
		case <-time.After(pipSnapToEdgeAnimationDuration):
		case <-ctx.Done():
			return errors.Wrap(ctx.Err(), "timeout while doing sleep")
		}

		// After swipe, check that the PiP window arrived to destination.
		bounds, err = act.WindowBounds(ctx)
		if err != nil {
			return errors.Wrap(err, "could not get PiP window bounds after swipe")
		}
		switch dir.border {
		case left:
			if bounds.Left > pipWorkAreaInsetsPx {
				return errors.Errorf("failed swipe to left; expected < %d, actual %d", pipWorkAreaInsetsPx, bounds.Left)
			}
		case right:
			if bounds.Right < dispW-pipWorkAreaInsetsPx {
				return errors.Errorf("failed swipe to right; expected < %d, actual %d", dispW-pipWorkAreaInsetsPx, bounds.Right)
			}
		case top:
			if bounds.Top > pipWorkAreaInsetsPx {
				return errors.Errorf("failed swipe to top; expected < %d, actual %d", pipWorkAreaInsetsPx, bounds.Top)
			}
		case bottom:
			if bounds.Bottom > dispH-shelfSizePx-pipWorkAreaInsetsPx {
				return errors.Errorf("failed swipe to bottom; expected %+v, actual %+v", origBounds, bounds)
			}
		}
	}
	return nil
}

// testPIPGravityStatusArea tests that PiP windows moves accordingly when the status area is hidden / displayed.
func testPIPGravityStatusArea(ctx context.Context, cr *chrome.Chrome, tconn *chrome.Conn, act *arc.Activity, d *ui.Device) error {
	// testPIPGravityStatusArea verifies that:
	// 1) The PiP window moves to the left of the status area when it is shown.
	// 2) The PiP window returns close the right border when the status area is dismissed.
	//    This is because the gravity is "to the right".
	tabletMode, err := isTabletModeEnabled(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get tablet mode")
	}
	if tabletMode {
		// TODO(ricardoq): Trigger the status area by injecting a touch, or add autotest private API.
		testing.ContextLog(ctx, "Ignoring testPIPGravityStatusArea() in tablet mode. Cannot trigger status area with keyboard.")
		return nil
	}

	// PiP window should automatically move away from the system status area.
	ew, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to open keyboard device")
	}
	defer ew.Close()

	dispMode, err := getInternalDisplayMode(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get selected display mode")
	}
	trayMenuWidthPx := int(trayMenuWidthDp * dispMode.DeviceScaleFactor)
	testing.ContextLog(ctx, "Using: trayMenuWidthPx = ", trayMenuWidthPx)

	// 0) Sanity check. Verify that PiP window is in the expected initial position.

	bounds, err := act.WindowBounds(ctx)
	if err != nil {
		return errors.Wrap(err, "could not get PiP window bounds")
	}
	testing.ContextLogf(ctx, "Initial bounds: %+v", bounds)

	if bounds.Right < dispMode.WidthInNativePixels-trayMenuWidthPx {
		return errors.Errorf("unexpected initial position %+v", bounds)
	}

	// 1) The PiP window moves to the left of the status area when it is shown.

	// Assuming status area is already dismissed state. Will fail otherwise.
	testing.ContextLog(ctx, "Injecting Alt+Shift+s to display status area.")
	if err = ew.Accel(ctx, "Alt+Shift+s"); err != nil {
		return err
	}

	waitForPIPReady(ctx, time.Second)

	bounds, err = act.WindowBounds(ctx)
	if err != nil {
		return errors.Wrap(err, "could not get PiP window bounds")
	}
	testing.ContextLogf(ctx, "Bounds after showing status area: %+v", bounds)

	if bounds.Right > dispMode.WidthInNativePixels-trayMenuWidthPx {
		return errors.Errorf("failed to avoid status area; expected right bounds: %d < %d",
			bounds.Right, dispMode.WidthInNativePixels-trayMenuWidthPx)
	}

	// 2) The PiP window returns close the right border when the status area is dismissed.

	testing.ContextLog(ctx, "Injecting Alt+Shift+s again to dismiss status area.")
	if err = ew.Accel(ctx, "Alt+Shift+s"); err != nil {
		return err
	}

	waitForPIPReady(ctx, time.Second)

	bounds, err = act.WindowBounds(ctx)
	if err != nil {
		return errors.Wrap(err, "could not get PiP window bounds")
	}
	testing.ContextLogf(ctx, "Bounds after dismissing status area: %+v", bounds)

	pipWorkAreaInsetsPx := int(pipWorkAreaInsetsDp * dispMode.DeviceScaleFactor)
	testing.ContextLog(ctx, "Using: pipWorkAreaInsetsPx = ", pipWorkAreaInsetsPx)

	if bounds.Right < dispMode.WidthInNativePixels-pipWorkAreaInsetsPx {
		return errors.Errorf("failed to return to right border; expected right bounds %d > %d",
			bounds.Right, dispMode.WidthInNativePixels-pipWorkAreaInsetsPx)
	}
	return nil
}

// testPIPGravityShelfAutoHide tests that PiP windows moves accordingly when the shelf is hidden / displayed.
func testPIPGravityShelfAutoHide(ctx context.Context, cr *chrome.Chrome, tconn *chrome.Conn, act *arc.Activity, d *ui.Device) error {
	// The test verifies that:
	// 1) PiP window is created on top of the shelf.
	// 2) PiP window does not fall down when the shelf disappears. This is because gravity is "to the right."
	// 3) PiP is moved to bottom/center causing a gravity is "down".
	// 4) The PiP window moves up, staying on top of the shelf, when the shelf appears again.

	dispInfo, err := display.GetInternalInfo(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get internal display info")
	}

	dispMode, err := getInternalDisplayMode(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get display mode")
	}

	// Restore original shelf state on exit
	origAutoHide, err := getShelfAutoHideBehavior(ctx, tconn, dispInfo.ID)
	if err != nil {
		return errors.Wrap(err, "could not get shelf auto hide behavior")
	}
	defer setShelfAutoHideBehavior(ctx, tconn, dispInfo.ID, origAutoHide)

	if origAutoHide != "never" {
		return errors.Errorf("unexpected shelf auto hide state; expected = 'never', actual='%s'", origAutoHide)
	}

	shelfSizePx := int(shelfSizeDp * dispMode.DeviceScaleFactor)
	testing.ContextLog(ctx, "Using: shelfSizePx = ", shelfSizePx)

	// 1) PiP window: on top of shelf.

	origBounds, err := act.WindowBounds(ctx)
	if err != nil {
		return errors.Wrap(err, "could not get PiP window bounds")
	}
	testing.ContextLogf(ctx, "Initial bounds: %+v", origBounds)

	if origBounds.Bottom > dispMode.HeightInNativePixels-shelfSizePx {
		return errors.Errorf("unexpected initial bounds: %+v", origBounds)
	}

	// 2) PiP window does not fall down when the shelf disappears.

	testing.ContextLog(ctx, "Setting shelf auto hide = always")
	if err := setShelfAutoHideBehavior(ctx, tconn, dispInfo.ID, "always"); err != nil {
		return errors.Wrap(err, "failed to set shelf autohide behavior")
	}

	waitForPIPReady(ctx, time.Second)

	bounds, err := act.WindowBounds(ctx)
	if err != nil {
		return errors.Wrap(err, "could not get PiP window bounds")
	}
	testing.ContextLogf(ctx, "Bounds after shelf disappeared: %+v", bounds)

	if math.Abs(float64(bounds.Bottom-origBounds.Bottom)) > pipPositionErrorMarginPx {
		return errors.Errorf("expected bounds %+v, actual %+v", origBounds, bounds)
	}

	// 3) PiP is moved to bottom/center causing a "down" gravity.

	newX := dispMode.WidthInNativePixels / 2
	testing.ContextLogf(ctx, "Moving PiP to %d,%d", newX, bounds.Top)
	if err := act.MoveWindow(ctx, arc.NewPoint(dispMode.WidthInNativePixels/2, bounds.Top), 2*time.Second); err != nil {
		return errors.Wrapf(err, "failed to move PiP window to %d,%d", newX, bounds.Top)
	}

	waitForPIPReady(ctx, time.Second)

	bounds, err = act.WindowBounds(ctx)
	if err != nil {
		return errors.Wrap(err, "could not get PiP window bounds")
	}
	testing.ContextLogf(ctx, "Bounds after activity got restarted: %+v", bounds)

	pipWorkAreaInsetsPx := int(pipWorkAreaInsetsDp * dispMode.DeviceScaleFactor)
	testing.ContextLog(ctx, "Using: pipWorkAreaInsetsPx = ", pipWorkAreaInsetsPx)

	if bounds.Bottom+pipWorkAreaInsetsPx < dispMode.HeightInNativePixels {
		return errors.Errorf("invalid PiP bounds %+v; expected %d > %d",
			bounds, bounds.Bottom+pipWorkAreaInsetsPx, dispMode.HeightInNativePixels)
	}

	// 4) PiP window should go up when the shelf appears

	testing.ContextLog(ctx, "Setting shelf auto hide = never")
	if err := setShelfAutoHideBehavior(ctx, tconn, dispInfo.ID, "never"); err != nil {
		return errors.Wrap(err, "failed to set shelf autohide behavior")
	}

	waitForPIPReady(ctx, time.Second)

	bounds, err = act.WindowBounds(ctx)
	if err != nil {
		return errors.Wrap(err, "could not get PiP window bounds")
	}
	testing.ContextLogf(ctx, "Bounds after shelf appeared: %+v", bounds)

	if math.Abs(float64(bounds.Bottom-origBounds.Bottom)) > pipPositionErrorMarginPx {
		return errors.Errorf("expected bounds: %+v, actual: %+v", origBounds, bounds)
	}

	return nil
}

// testPIPToggleTabletMode verifies that the PiP window position is the same after toggling tablet mode.
func testPIPToggleTabletMode(ctx context.Context, cr *chrome.Chrome, tconn *chrome.Conn, act *arc.Activity, d *ui.Device) error {
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

	if err := waitForPIPReady(ctx, time.Second); err != nil {
		return err
	}

	bounds, err := act.WindowBounds(ctx)
	testing.ContextLogf(ctx, "Bounds after toggling tablet mode: %+v", origBounds)

	if origBounds != bounds {
		return errors.Errorf("expected position %+v, actual %+v", origBounds, bounds)
	}
	return nil
}

// helper functions

// activatePIPMenuActivity activates the PiP Menu overlay by injecting KEYCODE_WINDOW event in Android.
// Undefined behavior if there are no active PiP windows.
func activatePIPMenuActivity(ctx context.Context, d *ui.Device) error {
	// TODO(ricardoq): Place this code inside arc.Activity once we figure out if it is possible to inject keyCodeWindow using input.Keyboard.
	// ATM is not possible to place this code as it is, since there is a circular dependency between arc and arc.ui.

	// Corresponds to Android KEYCODE_WINDOW. See:
	// https://cs.corp.google.com/android/frameworks/base/core/java/android/view/KeyEvent.java
	const keyCodeWindow = 171

	// Activates PiP menu. For reference, see:
	// https://cs.corp.google.com/android/frameworks/base/services/core/java/com/android/server/policy/PhoneWindowManager.java
	d.PressKeyCode(ctx, keyCodeWindow, 0)

	select {
	case <-time.After(pipMenuAnimationDuration):
	case <-ctx.Done():
		return errors.Wrap(ctx.Err(), "timeout while doing sleep")
	}
	return nil
}

// setTabletModeEnabled enables / disables tablet mode.
// After calling this function, it won't be possible to physically switch to/from tablet mode since that functionality will be disabled.
func setTabletModeEnabled(ctx context.Context, c *chrome.Conn, enabled bool) error {
	e := strconv.FormatBool(enabled)
	expr := fmt.Sprintf(
		`new Promise(function(resolve, reject) {
			chrome.autotestPrivate.setTabletModeEnabled(%s, 
				function(enabled) {
					if (enabled == %s) {
						resolve(chrome.runtime.lastError ? chrome.runtime.lastError.message : "");
					} else {
						reject();
					}
				});
		})`, e, e)

	if err := c.EvalPromise(ctx, expr, nil); err != nil {
		return err
	}
	return nil
}

// isTabletModeEnabled gets the tablet mode enabled status.
func isTabletModeEnabled(ctx context.Context, c *chrome.Conn) (enabled bool, err error) {
	const expr = `new Promise(function(resolve, reject) {
			chrome.autotestPrivate.isTabletModeEnabled(function(enabled) {
					resolve(chrome.runtime.lastError ? chrome.runtime.lastError.message : enabled);
				});
		})`

	if err := c.EvalPromise(ctx, expr, &enabled); err != nil {
		return false, err
	}
	return enabled, nil
}

// setShelfAutoHideBehavior sets the shelf auto hide behavior.
// displayId is the display that contains the shelf.
// Valid values for behavior: "always", "never" or "hidden".
func setShelfAutoHideBehavior(ctx context.Context, c *chrome.Conn, displayID string, behavior string) error {
	expr := fmt.Sprintf(
		`new Promise(function(resolve, reject) {
			chrome.autotestPrivate.setShelfAutoHideBehavior(%q, %q, () => {
				if (chrome.runtime.lastError === undefined) {
					resolve();
				} else {
					reject(chrome.runtime.lastError.message);
				}
			});
		})`, displayID, behavior)

	if err := c.EvalPromise(ctx, expr, nil); err != nil {
		return err
	}
	return nil
}

// getShelfAutoHideBehavior returns the shelf auto hide behavior.
// displayId is the display that contains the shelf.
// Possible return values: "always", "never" or "hidden".
func getShelfAutoHideBehavior(ctx context.Context, c *chrome.Conn, displayID string) (behavior string, err error) {
	expr := fmt.Sprintf(
		`new Promise(function(resolve, reject) {
			chrome.autotestPrivate.getShelfAutoHideBehavior(%q, (behavior) => {
				if (chrome.runtime.lastError === undefined) {
					resolve(behavior);
				} else {
					reject(chrome.runtime.lastError.message);
				}
			});
		})`, displayID)

	if err := c.EvalPromise(ctx, expr, &behavior); err != nil {
		return "", err
	}
	return behavior, nil
}

// waitForPIPReady waits until the PiP windows is ready.
func waitForPIPReady(ctx context.Context, timeout time.Duration) error {
	// TODOD(ricardoq): Find a robust way to wait for "PiP is ready" condition.
	// Calling ui.Device.WaitForIdle() or ui.Device.WaitForWindowUpdate() does not work
	// since PiP windows are not the "current windows".
	select {
	case <-time.After(timeout):
	case <-ctx.Done():
		return errors.New("waitForPIPReady aborted during sleep")
	}
	return nil
}

// getInternalDisplayMode returns the display mode that is currently selected in the internal display.
func getInternalDisplayMode(ctx context.Context, tconn *chrome.Conn) (dispMode *display.DisplayMode, err error) {
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
