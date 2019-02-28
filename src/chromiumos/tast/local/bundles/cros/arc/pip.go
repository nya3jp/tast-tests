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

func init() {
	testing.AddTest(&testing.Test{
		Func:         Pip,
		Desc:         "Checks that ARC++ Picture-in-Pictures works as expected.",
		Contacts:     []string{"ricardoq@chromium.org", "arc-eng@google.com"},
		Attr:         []string{"disabled", "informational"},
		SoftwareDeps: []string{"touch_view", "android", "android_p", "chrome_login"},
		Timeout:      5 * time.Minute,
		Data:         []string{"piptest.apk"},
	})
}

func Pip(ctx context.Context, s *testing.State) {
	const (
		apk = "piptest.apk"
		pkg = "com.example.edcourtney.pictureinpicturetest"

		idPrefix = pkg + ":id/"
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
	if err := a.Install(ctx, s.DataPath(apk)); err != nil {
		s.Fatal("Failed installing app: ", err)
	}

	act, err := arc.NewActivity(a, pkg, ".MainActivity")
	if err != nil {
		s.Fatal("Failed to create new activity: ", err)
	}
	defer act.Close()

	d, err := ui.NewDevice(ctx, a)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close()
	d.EnableDebug()

	tabletModeEnabled, err := isTabletModeEnabled(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get tablet mode")
	}

	firstTime := true
	for _, tabletMode := range []bool{false, true} {

		testing.ContextLogf(ctx, "Setting tablet mode enabled to %t", tabletMode)
		tm, err := isTabletModeEnabled(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to get tablet mode")
		}
		if tm != tabletMode {
			if err := setTabletModeEnabled(ctx, tconn, tabletMode); err != nil {
				s.Fatal("Failed to set tablet mode enabled to %t", tabletMode)
			}
			d.WaitForIdle(ctx, 3*time.Second)
		}

		type testFunc func(ctx context.Context, cr *chrome.Chrome, tconn *chrome.Conn, act *arc.Activity, d *ui.Device) error
		for idx, entry := range []struct {
			testName string
			fn       testFunc
		}{
			{"PiP Move", testPIPMove},
			{"PiP Resize", testPIPResize},
			{"PIP Avoid Obstacle", testPIPAvoidObstacle},
			{"PiP Fling", testPIPFling},
			{"PiP ShelfAutoHide", testPIPShelfAutoHide},
			{"PiP TabletClamshell", testPIPTabletClamshell},
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
			d.WaitForIdle(ctx, 3*time.Second)
			must(d.Object(ui.ID(fabID)).Click(ctx))
			d.WaitForIdle(ctx, 3*time.Second)

			if err := entry.fn(ctx, cr, tconn, act, d); err != nil {
				path := fmt.Sprintf("%s/screenshot-pip-failed-test-%d.png", s.OutDir(), idx)
				if err := screenshot.CaptureChrome(ctx, cr, path); err != nil {
					s.Log("Failed to capture screenshot")
				}
				s.Errorf("'%s' + tablet mode(%t) failed: %v", entry.testName, tabletMode, err)
			}
		}
	}

	// Be a good citizen and restore tablet mode to its original state.
	if err := setTabletModeEnabled(ctx, tconn, tabletModeEnabled); err != nil {
		testing.ContextLogf(ctx, "Failed to restore tablet mode enabled = %q", tabletModeEnabled)
	}
}

// testPIPShelfAutoHide tests that the PiP window moves down when the
func testPIPShelfAutoHide(ctx context.Context, cr *chrome.Chrome, tconn *chrome.Conn, act *arc.Activity, d *ui.Device) error {
	const (
		// FIXME(ricardoq): How many pixels should it move?
		pixelsToMove = 10
	)
	info, err := display.GetInternalInfo(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "could not internal info")
	}

	origBounds, err := act.WindowBounds(ctx)
	if err != nil {
		return errors.Wrap(err, "could not get PiP window bounds")
	}

	// 1) Shelf Auto Hide = Always. PiP window should move down a few pixels.

	testing.ContextLog(ctx, "Setting shelf auto hide = always")
	if err := setShelfAutoHideBehavior(ctx, tconn, info.ID, "always"); err != nil {
		return errors.Wrap(err, "failed to set shelf autohide behavior")
	}

	// Wait a few ms to let the shelf animation finishes.
	select {
	case <-time.After(200 * time.Millisecond):
	case <-ctx.Done():
		return errors.Wrap(err, "timeout")
	}

	bounds, err := act.WindowBounds(ctx)
	if err != nil {
		return errors.Wrap(err, "could not get PiP window bounds")
	}

	if bounds.Top < origBounds.Top+pixelsToMove {
		testing.ContextLogf(ctx, "PiP failed to move %d pixels. Original position: %+v, final position: %+v", origBounds, bounds)
		return errors.Errorf("PiP window did not move enough; moved pixels: %d", bounds.Top-origBounds.Top)
	}

	// 2) Shelf Auto Hide = Never. PiP window should move to its original position.

	testing.ContextLog(ctx, "Setting shelf auto hide = never")
	if err := setShelfAutoHideBehavior(ctx, tconn, info.ID, "never"); err != nil {
		return errors.Wrap(err, "failed to set shelf autohide behavior")
	}

	// Wait a few ms to let the shelf animation finishes.
	select {
	case <-time.After(200 * time.Millisecond):
	case <-ctx.Done():
		return errors.Wrap(err, "timeout")
	}

	bounds, err = act.WindowBounds(ctx)
	if err != nil {
		return errors.Wrap(err, "could not get PiP window bounds")
	}

	if bounds != origBounds {
		testing.ContextLogf(ctx, "PiP failed to return to its original position. Expected: %+v, actual: %+v", origBounds, bounds)
		return errors.Errorf("PiP window failed to return to its original position; expected: %+v, actual: %+v", origBounds, bounds)
	}

	return nil
}

// testPIPMove tests that dragging the PiP window works as expected.
func testPIPMove(ctx context.Context, cr *chrome.Chrome, tconn *chrome.Conn, act *arc.Activity, d *ui.Device) error {
	origBounds, err := act.WindowBounds(ctx)
	if err != nil {
		return errors.Wrap(err, "could not get PiP window bounds")
	}
	testing.ContextLog(ctx, "original bounds: %+v", origBounds)

	// Moving window very slowly to prevent triggering any possible gesture.
	testing.ContextLog(ctx, "Moving PiP window to x=0, y=0")
	if err := act.MoveWindow(ctx, arc.Point{X: 2000, Y: 1500}, 3*time.Second); err != nil {
		return errors.Wrap(err, "could not move Pip window")
	}

	bounds, err := act.WindowBounds(ctx)
	if err != nil {
		return errors.Wrap(err, "could not get PiP window bounds")
	}

	// A PiP window doesn't have caption. According to Android the Y position will be negative.
	// PiP windows don't snap to borders. A margin is used.
	if bounds.Top > 0 || bounds.Left > 50 {
		return errors.Wrapf(err, "invalid PiP bounds: %+v", bounds)
	}
	return nil
}

func testPIPResize(ctx context.Context, cr *chrome.Chrome, tconn *chrome.Conn, act *arc.Activity, d *ui.Device) error {
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get a Test API connection")
	}

	// Activate "resize handler", otherwise resize will fail.
	if err := activatePIPMenuActivity(ctx, d); err != nil {
		return errors.Wrap(err, "could not activate PiP menu")
	}

	testing.ContextLog(ctx, "Resizing PiP window to x=0, y=0 from top-left corner")

	// The PiP window should be on the bottom-right corner. Resizing it from the top-left border to its max size.
	if err := act.ResizeWindow(ctx, arc.BorderTopLeft, arc.Point{X: 0, Y: 0}, time.Second); err != nil {
		return errors.Wrap(err, "could not move Pip window")
	}
	bounds, err := act.WindowBounds(ctx)
	if err != nil {
		return errors.Wrap(err, "could not get PiP window bounds")
	}

	dispInfo, err := display.GetInternalInfo(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get internal display info")
	}

	if len(dispInfo.Modes) == 0 {
		return errors.New("invalid size for dispInfo.Modes")
	}

	// Ignore the selected mode, just use mode[0]. We only care about the native pixels which must be the same for all modes.
	dispW := float64(dispInfo.Modes[0].WidthInNativePixels)
	dispH := float64(dispInfo.Modes[0].HeightInNativePixels)

	// Give a 2% error margin. Resizing by injecting touches might not be %100 precise.
	marginW := dispW * 0.02
	marginH := dispH * 0.02

	w := float64(bounds.Right - bounds.Left)
	h := float64(bounds.Bottom - bounds.Top)

	// PiP will honor its aspect ratio after resize. It is enough to check only one of the dimensions.
	if dispH < dispW {
		if math.Abs(dispH/2-h) >= marginH {
			testing.ContextLogf(ctx, "%f - %f >= %f (%v)", dispH/2, h, marginH, bounds)
			return errors.Wrapf(err, "unexpected bounds size (%+v)", bounds)
		}
	} else {
		if math.Abs(dispW/2-w) >= marginW {
			testing.ContextLogf(ctx, "%f - %f >= %f", dispW/2, h, marginW)
			return errors.Wrapf(err, "unexpected bounds size (%+v)", bounds)
		}
	}
	return nil
}

// testPIPAvoidObstacle tests the PiP window moves away from "obstables" like the system status area.
// The test launches the "system status area". The PiP window should move to its left.
// Then the "system status area" is dismissed, and the PiP window should return to its original position.
func testPIPAvoidObstacle(ctx context.Context, cr *chrome.Chrome, tconn *chrome.Conn, act *arc.Activity, d *ui.Device) error {
	const (
		// FIXME(ricardoq): find actual value
		statusAreaWidth = 150
	)

	// PiP window should automatically move away from the system status area.
	ew, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to open keyboard device")
	}
	defer ew.Close()

	origBounds, err := act.WindowBounds(ctx)
	if err != nil {
		return errors.Wrap(err, "could not get PiP window bounds")
	}

	// 1) Verify that PiP window moved away

	// Display system status area. Assuming it is already dismissed state.
	testing.ContextLog(ctx, "Showing system status area")
	if err = ew.Accel(ctx, "Alt+Shift+s"); err != nil {
		return errors.Wrap(err, "failed")
	}

	// TODO(ricardoq): Find a robust way to detect end-of-animation.
	// Wait until status area end of animation
	select {
	case <-time.After(1 * time.Second):
	case <-ctx.Done():
		return errors.Wrap(err, "timeout")
	}

	bounds, err := act.WindowBounds(ctx)
	if err != nil {
		return errors.Wrap(err, "could not get PiP window bounds")
	}

	if bounds.Left+statusAreaWidth > origBounds.Left {
		testing.ContextLogf(ctx, "Failed to avoid obstable. Position before status area: %+v. After: %+v", origBounds, bounds)
		return errors.Errorf("PiP window failed to avoid status area; position before status area: %+v, after: %+v", origBounds, bounds)
	}

	// 2) Verify that PiP returns to its original position after status area is dismissed

	// Dismiss system status area
	testing.ContextLog(ctx, "Dismissing system status area")
	if err = ew.Accel(ctx, "Alt+Shift+s"); err != nil {
		return errors.Wrap(err, "failed")
	}

	// Wait until status area end of animation
	select {
	case <-time.After(1 * time.Second):
	case <-ctx.Done():
		return errors.Wrap(err, "timeout")
	}

	bounds, err = act.WindowBounds(ctx)
	if err != nil {
		return errors.Wrap(err, "could not get PiP window bounds")
	}

	if bounds != origBounds {
		testing.ContextLogf(ctx, "Failed to return to its original position. Expected: %+v, actual: %+v", origBounds, bounds)
		return errors.Errorf("PiP window failed to return to its original position;  expected: %+v, actual: %+v", origBounds, bounds)
	}

	return nil
}

// testPIPfling tests that the "fling gesture" works as expected in PiP windows.
// It tests the fling in four directions: left, up, right and down.
// Assumes that no obstables are in the PiP window way. If the shelf is present, it assumes it is at the bottom.
// Otherwise, it will fail.
func testPIPFling(ctx context.Context, cr *chrome.Chrome, tconn *chrome.Conn, act *arc.Activity, ui *ui.Device) error {
	type borderType int
	const (
		// Borders to check after swipe.
		left borderType = iota
		right
		top
		bottom
	)
	const (
		// PiP windows are placed a few pixels away from the border.
		// FIXME(ricardoq): discuss it with edcourtney@. Where is this value defined?
		borderOffset = 25
		// Steps used for each swipe. Each step takes 5ms. A 200ms swipe
		steps = 200 / 5
	)

	dispInfo, err := display.GetInternalInfo(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "could not get internal display")
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

	// Calculate Pixel (screen display) / Tuxel (touch device) ratio.
	touchW := tsw.Width()
	touchH := tsw.Height()

	// Display bounds.
	if len(dispInfo.Modes) == 0 {
		return errors.New("invalid size for dispInfo.Modes")
	}
	// Ignore the selected mode, just use mode[0]. We only care about the native pixels which must be the same for all modes.
	dispW := dispInfo.Modes[0].WidthInNativePixels
	dispH := dispInfo.Modes[0].HeightInNativePixels

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

		testing.ContextLogf(ctx, "Executing swipe gesture from {%d,%d} to {%d,%d}", x0, y0, x1, y1)
		if err := stw.Swipe(ctx, x0, y0, x1, y1, steps); err != nil {
			return errors.Wrap(err, "failed to execute a swipe gesture")
		}

		// PiP fling animation should not take more than 150m (kPipSnapToEdgeAnimationDurationMs). Using 300ms to be safe.
		// See: https://cs.chromium.org/chromium/src/ash/wm/pip/pip_window_resizer.cc?type=cs&g=0&l=34
		select {
		case <-time.After(300 * time.Millisecond):
		case <-ctx.Done():
			return errors.Wrap(ctx.Err(), "timeout while doing sleep")
		}

		// after swipe, check that the PiP window arrived to destination
		bounds, err = act.WindowBounds(ctx)
		if err != nil {
			return errors.Wrap(err, "could not get PiP window bounds after swipe")
		}
		switch dir.border {
		case left:
			if bounds.Left > borderOffset {
				testing.ContextLogf(ctx, "Failed swipe to left. Expected left border: <%f. Actual: %f", borderOffset, bounds.Left)
				return errors.Errorf("failed swipe to left; expected left border: <%f, actual: %f", borderOffset, bounds.Left)
			}
		case right:
			if bounds.Right < dispW-borderOffset {
				testing.ContextLogf(ctx, "Failed swipe to right. Expected right border: <%f. Actual: %f", dispW-borderOffset, bounds.Right)
				return errors.Errorf("failed swipe to right; expected right border: <%f, actual: %f", dispW-borderOffset, bounds.Right)
			}
		case top:
			if bounds.Top > borderOffset {
				testing.ContextLogf(ctx, "Failed swipe to top. Expected top border: <%f. Actual: %f", borderOffset, bounds.Top)
				return errors.Errorf("failed swipe to top; expected top border: <%f, actual: %f", borderOffset, bounds.Top)
			}
		case bottom:
			// After all swipes are done, final position must be the same as starting position.
			// FIXME(ricardoq): Discuss it with edcourtney@. Currently this test fails. Final position has a one pixel difference. Bug?
			if bounds != origBounds {
				testing.ContextLogf(ctx, "Failed swipe to bottom. Expected bounds: %+v. Actual: %+v", origBounds, bounds)
				return errors.Errorf("failed swipe to bottom; expected bounds: %+v, actual: %+v", origBounds, bounds)
			}
		}
	}
	return nil
}

// testPIPTabletClamshell tests that the PiP window keeps its position after switching to tablet/clamshell mode.
func testPIPTabletClamshell(ctx context.Context, cr *chrome.Chrome, tconn *chrome.Conn, act *arc.Activity, ui *ui.Device) error {
	// Used as reference once the 4 swipes are done.
	origBounds, err := act.WindowBounds(ctx)

	enabled, err := isTabletModeEnabled(ctx, tconn)
	if err != nil {
		return errors.New("failed to get whether tablet mode is enabled")
	}

	testing.ContextLogf(ctx, "Setting 'tablet mode enabled = %t'", !enabled)
	if err := setTabletModeEnabled(ctx, tconn, !enabled); err != nil {
		return errors.New("failed to set tablet mode")
	}

	// Used as reference once the 4 swipes are done.
	bounds, err := act.WindowBounds(ctx)

	if origBounds != bounds {
		testing.ContextLogf(ctx, "Incorrect PiP window position. Expected: %+v, actual: %+v", origBounds, bounds)
	}

	// Restore tablet mode to original state
	testing.ContextLogf(ctx, "Restoring tablet mode. 'tablet mode enabled = %t'", !enabled)
	if err := setTabletModeEnabled(ctx, tconn, enabled); err != nil {
		return errors.New("failed to restore tablet mode")
	}
	return nil
}

// helper functions

// activatePIPMenuActivity activates the PiP Menu overlay by injecting KEYCODE_WINDOW event in Android.
// Undefined behavior if there are no active PiP windows.
// TODO(ricardoq): This code should be placed inside arc.Activity, but that will create a circular dependency between arc and arc.ui.
// Alternative: use tast keyboard injector, but apparently keycode 171 gets filtered by CrOs (?)
func activatePIPMenuActivity(ctx context.Context, d *ui.Device) error {
	// Corresponds to Android KEYCODE_WINDOW. See:
	// https://cs.corp.google.com/android/frameworks/base/core/java/android/view/KeyEvent.java
	const keyCodeWindow = 171

	// Activate PiP menu. See:
	// https://cs.corp.google.com/android/frameworks/base/services/core/java/com/android/server/policy/PhoneWindowManager.java?type=cs&g=0&l=4007
	d.PressKeyCode(ctx, keyCodeWindow, 0)

	// Wait a few milliseconds until the PiP Menu animation is complete.
	select {
	case <-time.After(300 * time.Millisecond):
	case <-ctx.Done():
		return errors.Wrap(ctx.Err(), "timeout while doing sleep")
	}
	return nil
}

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
