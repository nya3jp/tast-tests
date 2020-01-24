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
	chromeui "chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
)

const (
	pipTestPkgName = "org.chromium.arc.testapp.pictureinpicture"

	// kCollisionWindowWorkAreaInsetsDp is hardcoded to 8dp.
	// See: https://cs.chromium.org/chromium/src/ash/wm/collision_detection/collision_detection_utils.h
	// TODO(crbug.com/949754): Get this value in runtime.
	collisionWindowWorkAreaInsetsDP = 8

	// pipPositionErrorMarginPX represents the error margin in pixels when comparing positions.
	// With some calculation, we expect the error could be a maximum of 2 pixels, but we use 1-pixel larger value just in case.
	// See b/129976114 for more info.
	// TODO(ricardoq): Remove this constant once the bug gets fixed.
	pipPositionErrorMarginPX = 3

	// When the drag-move sequence is started, the gesture controller might miss a few pixels before it finally
	// recognizes it as a drag-move gesture. This is specially true for PIP windows.
	// The value varies depending on acceleration/speed of the gesture. 35 works for our purpose.
	missedByGestureControllerDP = 35
)

type borderType int

const (
	left borderType = iota
	right
	top
	bottom
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PIP,
		Desc:         "Checks that ARC++ Picture-in-Picture works as expected",
		Contacts:     []string{"edcourtney@chromium.org", "arc-framework+tast@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"tablet_mode", "chrome"},
		Data:         []string{"ArcPipTastTest.apk"},
		Timeout:      5 * time.Minute,
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
			Pre:               arc.Booted(),
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
			Pre:               arc.VMBooted(),
		}},
	})
}

func PIP(ctx context.Context, s *testing.State) {

	// TODO(takise): This can hide the line number on which the test actually fails. Remove this.
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

	pipAct, err := arc.NewActivity(a, pipTestPkgName, ".PipActivity")
	if err != nil {
		s.Fatal("Failed to create PIP activity: ", err)
	}
	defer pipAct.Close()

	maPIPBaseAct, err := arc.NewActivity(a, pipTestPkgName, ".MaPipBaseActivity")
	if err != nil {
		s.Fatal("Failed to create multi activity PIP base activity: ", err)
	}
	defer maPIPBaseAct.Close()

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

	dispMode, err := ash.InternalDisplayMode(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get display mode: ", err)
	}

	// Run all subtests twice. First, with tablet mode disabled. And then, with it enabled.
	for _, tabletMode := range []bool{false, true} {
		s.Logf("Running tests with tablet mode enabled=%t", tabletMode)
		if err := ash.SetTabletModeEnabled(ctx, tconn, tabletMode); err != nil {
			s.Fatalf("Failed to set tablet mode enabled to %t: %v", tabletMode, err)
		}

		// There are two types of PIP: single activity PIP and multi activity PIP. Run each test with both types by default.
		for _, multiActivityPIP := range []bool{false, true} {

			type initializationType uint
			const (
				doNothing initializationType = iota
				startActivity
				enterPip
			)

			type testFunc func(context.Context, *chrome.TestConn, *arc.ARC, *arc.Activity, *ui.Device, *display.DisplayMode) error

			for idx, test := range []struct {
				name       string
				fn         testFunc
				initMethod initializationType
			}{
				{name: "PIP Move", fn: testPIPMove, initMethod: enterPip},
				{name: "PIP Resize To Max", fn: testPIPResizeToMax, initMethod: enterPip},
				{name: "PIP GravityStatusArea", fn: testPIPGravityStatusArea, initMethod: enterPip},
				{name: "PIP Toggle Tablet mode", fn: testPIPToggleTabletMode, initMethod: enterPip},
				{name: "PIP AutoPIP Minimize", fn: testPIPAutoPIPMinimize, initMethod: startActivity},
				{name: "PIP AutoPIP New Android Window", fn: testPIPAutoPIPNewAndroidWindow, initMethod: doNothing},
				{name: "PIP AutoPIP New Chrome Window", fn: testPIPAutoPIPNewChromeWindow, initMethod: startActivity},
				{name: "PIP ExpandPIP Shelf Icon", fn: testPIPExpandViaShelfIcon, initMethod: enterPip},
				{name: "PIP ExpandPIP Menu Touch", fn: testPIPExpandViaMenuTouch, initMethod: enterPip},
			} {
				if test.initMethod == startActivity || test.initMethod == enterPip {
					if multiActivityPIP {
						must(maPIPBaseAct.Start(ctx))
						must(maPIPBaseAct.WaitForResumed(ctx, 10*time.Second))
					}

					must(pipAct.Start(ctx))
					must(pipAct.WaitForResumed(ctx, 10*time.Second))
				}

				if test.initMethod == enterPip {
					// Make the app PIP via minimize.
					// We have some other ways to PIP an app, but for now this is the most reliable.
					must(pipAct.SetWindowState(ctx, arc.WindowStateMinimized))
					must(waitForPIPWindow(ctx, tconn))
				}

				if err := test.fn(ctx, tconn, a, pipAct, dev, dispMode); err != nil {
					path := fmt.Sprintf("%s/screenshot-pip-failed-test-%d.png", s.OutDir(), idx)
					if err := screenshot.CaptureChrome(ctx, cr, path); err != nil {
						s.Log("Failed to capture screenshot: ", err)
					}
					s.Errorf("%s test with tablet mode(%t) failed: %v", test.name, tabletMode, err)
				}

				must(pipAct.Stop(ctx))
				if multiActivityPIP {
					must(maPIPBaseAct.Stop(ctx))
				}
			}
		}
	}
}

// testPIPMove verifies that drag-moving the PIP window works as expected.
// It does that by drag-moving that PIP window horizontally 3 times and verifying that the position is correct.
func testPIPMove(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, pipAct *arc.Activity, dev *ui.Device, dispMode *display.DisplayMode) error {
	const (
		movementDuration = time.Second
		totalMovements   = 3
	)

	missedByGestureControllerPX := int(math.Round(missedByGestureControllerDP * dispMode.DeviceScaleFactor))
	testing.ContextLog(ctx, "Using: missedByGestureControllerPX = ", missedByGestureControllerPX)

	window, err := getPIPWindow(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "could not get PIP window")
	}
	origBounds := coords.ConvertBoundsFromDpToPx(window.BoundsInRoot, dispMode.DeviceScaleFactor)
	testing.ContextLogf(ctx, "Initial PIP bounds: %+v", origBounds)

	deltaX := dispMode.WidthInNativePixels / (totalMovements + 1)
	for i := 0; i < totalMovements; i++ {
		newBounds := origBounds
		newBounds.Left -= deltaX * (i + 1)
		testing.ContextLogf(ctx, "Moving PIP window to %d,%d", newBounds.Left, newBounds.Top)
		if err := pipAct.MoveWindow(ctx, coords.NewPoint(newBounds.Left, newBounds.Top), movementDuration); err != nil {
			return errors.Wrap(err, "could not move PIP window")
		}

		if err = waitForNewBoundsWithMargin(ctx, tconn, newBounds.Left, left, dispMode.DeviceScaleFactor, pipPositionErrorMarginPX+missedByGestureControllerPX); err != nil {
			return errors.Wrap(err, "failed to move PIP to left")
		}
	}
	return nil
}

// testPIPResizeToMax verifies that resizing the PIP window to a big size doesn't break its size constraints.
// It performs a drag-resize from PIP's left-top corner and compares the resized-PIP size with the expected one.
func testPIPResizeToMax(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, pipAct *arc.Activity, dev *ui.Device, dispMode *display.DisplayMode) error {
	// Activate PIP "resize handler", otherwise resize will fail. See:
	// https://android.googlesource.com/platform/frameworks/base/+/refs/heads/pie-release/services/core/java/com/android/server/policy/PhoneWindowManager.java#6387
	if err := dev.PressKeyCode(ctx, ui.KEYCODE_WINDOW, 0); err != nil {
		return errors.Wrap(err, "could not activate PIP menu")
	}

	if err := pipAct.WaitForResumed(ctx, 10*time.Second); err != nil {
		return errors.Wrap(err, "could not resume PIP menu actiivty")
	}

	window, err := getPIPWindow(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "could not get PIP window")
	}
	bounds := coords.ConvertBoundsFromDpToPx(window.BoundsInRoot, dispMode.DeviceScaleFactor)
	testing.ContextLogf(ctx, "Bounds before resize: %+v", bounds)

	testing.ContextLog(ctx, "Resizing window to x=0, y=0")
	// Resizing PIP to x=0, y=0, but it should stop once it reaches its max size.
	if err := pipAct.ResizeWindow(ctx, arc.BorderTopLeft, coords.NewPoint(0, 0), time.Second); err != nil {
		return errors.Wrap(err, "could not resize PIP window")
	}

	// Retrieve the PIP bounds again.
	window, err = getPIPWindow(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "could not get PIP window")
	}
	bounds = coords.ConvertBoundsFromDpToPx(window.BoundsInRoot, dispMode.DeviceScaleFactor)

	// Max PIP window size relative to the display size, as defined in WindowPosition.getMaximumSizeForPip().
	// See: https://cs.corp.google.com/pi-arc-dev/frameworks/base/services/core/arc/java/com/android/server/am/WindowPositioner.java
	// Dividing by integer 2 could loose the fraction, but so does the Java implementation.
	// TODO(crbug.com/949754): Get this value in runtime.
	const pipMaxSizeFactor = 2

	// Currently we have a synchronization issue, where the min/max value Android sends is incorrect because
	// an app enters PIP at the same time as the size of the shelf changes.
	// This issue is causing no problem in real use cases, but disallowing us to check the exact bounds here.
	// So, here we just check whether the maximum size we can set is smaller than the half size of the display, which must hold all the time.
	if dispMode.HeightInNativePixels < dispMode.WidthInNativePixels {
		if bounds.Height > dispMode.HeightInNativePixels/pipMaxSizeFactor+pipPositionErrorMarginPX {
			return errors.Wrap(err, "the maximum size of the PIP window must be half of the display height")
		}
	} else {
		if bounds.Width > dispMode.WidthInNativePixels/pipMaxSizeFactor+pipPositionErrorMarginPX {
			return errors.Wrap(err, "the maximum size of the PIP window must be half of the display width")
		}
	}

	return nil
}

// testPIPGravityStatusArea tests that PIP windows moves accordingly when the status area is hidden / displayed.
func testPIPGravityStatusArea(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, pipAct *arc.Activity, dev *ui.Device, dispMode *display.DisplayMode) error {
	// testPIPGravityStatusArea verifies that:
	// 1) The PIP window moves to the left of the status area when it is shown.
	// 2) The PIP window returns close the right border when the status area is dismissed.

	collisionWindowWorkAreaInsetsPX := int(math.Round(collisionWindowWorkAreaInsetsDP * dispMode.DeviceScaleFactor))
	testing.ContextLog(ctx, "Using: collisionWindowWorkAreaInsetsPX = ", collisionWindowWorkAreaInsetsPX)

	// 0) Sanity check. Verify that PIP window is in the expected initial position and that Status Area is hidden.

	window, err := getPIPWindow(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "could not get PIP window")
	}
	bounds := coords.ConvertBoundsFromDpToPx(window.BoundsInRoot, dispMode.DeviceScaleFactor)

	if err = waitForNewBoundsWithMargin(ctx, tconn, dispMode.WidthInNativePixels-collisionWindowWorkAreaInsetsPX, right, dispMode.DeviceScaleFactor, pipPositionErrorMarginPX); err != nil {
		return errors.Wrap(err, "the PIP window must be along the right edge of the display")
	}

	// 1) The PIP window should move to the left of the status area.

	testing.ContextLog(ctx, "Showing system status area")
	if err := showSystemStatusArea(ctx, tconn); err != nil {
		return err
	}
	// Be nice, and no matter what happens, hide the Status Area on exit.
	defer hideSystemStatusArea(ctx, tconn)

	statusRectDP, err := getStatusAreaRect(ctx, tconn, 10*time.Second)
	if err != nil {
		return errors.Wrap(err, "failed to get system status area rect")
	}
	statusLeftPX := int(math.Round(float64(statusRectDP.Left) * dispMode.DeviceScaleFactor))

	if err = waitForNewBoundsWithMargin(ctx, tconn, statusLeftPX-collisionWindowWorkAreaInsetsPX, right, dispMode.DeviceScaleFactor, pipPositionErrorMarginPX); err != nil {
		return errors.Wrap(err, "the PIP window must move to the left when system status area gets shown")
	}

	// 2) The PIP window should move close the right border when the status area is dismissed.

	testing.ContextLog(ctx, "Dismissing system status area")
	if err := hideSystemStatusArea(ctx, tconn); err != nil {
		return err
	}

	if err = waitForNewBoundsWithMargin(ctx, tconn, bounds.Left+bounds.Width, right, dispMode.DeviceScaleFactor, pipPositionErrorMarginPX); err != nil {
		return errors.Wrap(err, "the PIP window must go back to the original position when system status area gets hidden")
	}

	return nil
}

// testPIPToggleTabletMode verifies that the window position is the same after toggling tablet mode.
func testPIPToggleTabletMode(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, act *arc.Activity, dev *ui.Device, dispMode *display.DisplayMode) error {
	window, err := getPIPWindow(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "could not get PIP window")
	}
	origBounds := coords.ConvertBoundsFromDpToPx(window.BoundsInRoot, dispMode.DeviceScaleFactor)

	// Move the PIP window upwards as much as possible to avoid possible interaction with shelf.
	if err := act.MoveWindow(ctx, coords.NewPoint(origBounds.Left, 0), time.Second); err != nil {
		return errors.Wrap(err, "could not move PIP window")
	}
	missedByGestureControllerPX := int(math.Round(missedByGestureControllerDP * dispMode.DeviceScaleFactor))
	if err = waitForNewBoundsWithMargin(ctx, tconn, 0, top, dispMode.DeviceScaleFactor, pipPositionErrorMarginPX+missedByGestureControllerPX); err != nil {
		return errors.Wrap(err, "failed to move PIP to left")
	}

	// Update origBounds as we moved the window above.
	window, err = getPIPWindow(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "could not get PIP window")
	}

	origBounds = coords.ConvertBoundsFromDpToPx(window.BoundsInRoot, dispMode.DeviceScaleFactor)
	testing.ContextLogf(ctx, "Initial bounds: %+v", origBounds)

	tabletEnabled, err := ash.TabletModeEnabled(ctx, tconn)
	if err != nil {
		return errors.New("failed to get whether tablet mode is enabled")
	}
	defer ash.SetTabletModeEnabled(ctx, tconn, tabletEnabled)

	// TODO(takise): Currently there's no way to know if "everything's been done and nothing's changed on both Chrome and Android side".
	// We are thinking of adding a new sync logic for Tast tests, but until it gets done, we need to sleep for a while here.
	testing.Sleep(ctx, time.Second)

	testing.ContextLogf(ctx, "Setting 'tablet mode enabled = %t'", !tabletEnabled)
	if err := ash.SetTabletModeEnabled(ctx, tconn, !tabletEnabled); err != nil {
		return errors.New("failed to set tablet mode")
	}

	if err = waitForNewBoundsWithMargin(ctx, tconn, origBounds.Left, left, dispMode.DeviceScaleFactor, pipPositionErrorMarginPX); err != nil {
		return errors.Wrap(err, "failed swipe to left")
	}
	if err = waitForNewBoundsWithMargin(ctx, tconn, origBounds.Top, top, dispMode.DeviceScaleFactor, pipPositionErrorMarginPX); err != nil {
		return errors.Wrap(err, "failed swipe to left")
	}
	return nil
}

// testPIPAutoPIPMinimize verifies that minimizing an auto-PIP window will trigger PIP.
func testPIPAutoPIPMinimize(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, pipAct *arc.Activity, dev *ui.Device, dispMode *display.DisplayMode) error {
	// TODO(edcourtney): Test minimize via shelf icon, keyboard shortcut (alt-minus), and caption.
	if err := pipAct.SetWindowState(ctx, arc.WindowStateMinimized); err != nil {
		return errors.Wrap(err, "failed to set window state to minimized")
	}

	if err := waitForPIPWindow(ctx, tconn); err != nil {
		return errors.Wrap(err, "did not enter PIP")
	}

	return nil
}

// testPIPExpandViaMenuTouch verifies that PIP window is properly expanded by touching menu.
func testPIPExpandViaMenuTouch(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, pipAct *arc.Activity, dev *ui.Device, dispMode *display.DisplayMode) error {
	if err := expandPIPViaMenuTouch(ctx, tconn, pipAct, dev, dispMode); err != nil {
		return errors.Wrap(err, "could not expand PIP")
	}

	return ash.WaitForARCAppWindowState(ctx, tconn, pipTestPkgName, ash.WindowStateMaximized)
}

// testPIPExpandViaShelfIcon verifies that PIP window is properly expanded by pressing shelf icon.
func testPIPExpandViaShelfIcon(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, pipAct *arc.Activity, dev *ui.Device, dispMode *display.DisplayMode) error {
	if err := pressShelfIcon(ctx, tconn); err != nil {
		return errors.Wrap(err, "could not expand PIP")
	}

	return ash.WaitForARCAppWindowState(ctx, tconn, pipTestPkgName, ash.WindowStateMaximized)
}

// testPIPAutoPIPNewAndroidWindow verifies that creating a new Android window that occludes an auto-PIP window will trigger PIP.
func testPIPAutoPIPNewAndroidWindow(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, pipAct *arc.Activity, dev *ui.Device, dispMode *display.DisplayMode) error {
	const (
		settingPkgName = "com.android.settings"
		settingActName = ".Settings"
	)

	settingAct, err := arc.NewActivity(a, settingPkgName, settingActName)
	if err != nil {
		return errors.Wrap(err, "could not create Settings Activity")
	}
	defer settingAct.Close()

	if err := settingAct.Start(ctx); err != nil {
		return errors.Wrap(err, "could not start Settings Activity")
	}
	defer settingAct.Stop(ctx)

	if err := settingAct.WaitForResumed(ctx, 10*time.Second); err != nil {
		return errors.Wrap(err, "could not wait for Settings Activity to resume")
	}

	// Make sure the window will have an initial maximized state.
	if err := settingAct.SetWindowState(ctx, arc.WindowStateMaximized); err != nil {
		return errors.Wrap(err, "failed to set window state of Settings Activity to maximized")
	}

	if err := ash.WaitForARCAppWindowState(ctx, tconn, settingPkgName, ash.WindowStateMaximized); err != nil {
		return errors.Wrap(err, "did not maximize")
	}

	if err := settingAct.Stop(ctx); err != nil {
		return errors.Wrap(err, "could not stop Settings Activity while setting initial window state")
	}

	// Start the main activity that should enter PIP.
	if err := pipAct.Start(ctx); err != nil {
		return errors.Wrap(err, "could not start MainActivity")
	}

	if err := pipAct.WaitForResumed(ctx, 10*time.Second); err != nil {
		return errors.Wrap(err, "could not wait for MainActivity to resume")
	}

	// Start Settings Activity again, this time with the guaranteed correct window state.
	if err := settingAct.Start(ctx); err != nil {
		return errors.Wrap(err, "could not start Settings Activity")
	}

	// Wait for MainActivity to enter PIP.
	// TODO(edcourtney): Until we can identify multiple Android windows from the same package, just wait for
	// the Android state here. Ideally, we should wait for the Chrome side state, but we don't need to do anything after
	// this on the Chrome side so it's okay for now. See crbug.com/1010671.
	return waitForPIPWindow(ctx, tconn)
}

// testPIPAutoPIPNewChromeWindow verifies that creating a new Chrome window that occludes an auto-PIP window will trigger PIP.
func testPIPAutoPIPNewChromeWindow(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, pipAct *arc.Activity, dev *ui.Device, dispMode *display.DisplayMode) error {
	// Open a maximized Chrome window and close at the end of the test.
	if err := tconn.EvalPromise(ctx, `
new Promise((resolve, reject) => {
  chrome.windows.create({state: "maximized"}, () => {
		if (chrome.runtime.lastError) {
		  reject(new Error(chrome.runtime.lastError));
		} else {
		  resolve();
		}
	});
});
`, nil); err != nil {
		return errors.Wrap(err, "could not open maximized Chrome window")
	}
	defer tconn.EvalPromise(ctx, `
new Promise((resolve, reject) => {
  chrome.windows.getLastFocused({}, (window) => {
    chrome.windows.remove(window.id, () => {
			if (chrome.runtime.lastError) {
			  reject(new Error(chrome.runtime.lastError));
			} else {
			  resolve();
			}
		});
	});
});
`, nil)

	// Wait for MainActivity to enter PIP.
	// TODO(edcourtney): Until we can identify multiple Android windows from the same package, just wait for
	// the Android state here. Ideally, we should wait for the Chrome side state, but we don't need to do anything after
	// this on the Chrome side so it's okay for now. See crbug.com/1010671.
	return waitForPIPWindow(ctx, tconn)
}

// helper functions

// expandPIPViaMenuTouch injects touch events to the center of PIP window and expands PIP.
// The first touch event shows PIP menu and subsequent events expand PIP.
func expandPIPViaMenuTouch(ctx context.Context, tconn *chrome.TestConn, act *arc.Activity, dev *ui.Device, dispMode *display.DisplayMode) error {
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

	dispW := dispMode.WidthInNativePixels
	dispH := dispMode.HeightInNativePixels
	pixelToTuxelX := float64(tsw.Width()) / float64(dispW)
	pixelToTuxelY := float64(tsw.Height()) / float64(dispH)

	window, err := getPIPWindow(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "could not get PIP window bounds")
	}
	bounds := coords.ConvertBoundsFromDpToPx(window.BoundsInRoot, dispMode.DeviceScaleFactor)

	pixelX := float64(bounds.Left + bounds.Width/2)
	pixelY := float64(bounds.Top + bounds.Height/2)
	x := input.TouchCoord(pixelX * pixelToTuxelX)
	y := input.TouchCoord(pixelY * pixelToTuxelY)

	testing.ContextLogf(ctx, "Injecting touch event to {%f, %f} to expand PIP; display {%d, %d}, PIP bounds {(%d, %d), %dx%d}",
		pixelX, pixelY, dispW, dispH, bounds.Left, bounds.Top, bounds.Width, bounds.Height)

	return testing.Poll(ctx, func(ctx context.Context) error {
		if err := stw.Move(x, y); err != nil {
			return errors.Wrap(err, "failed to execute touch gesture")
		}
		if err := stw.End(); err != nil {
			return errors.Wrap(err, "failed to finish swipe gesture")
		}

		windowState, err := ash.GetARCAppWindowState(ctx, tconn, pipTestPkgName)
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to get Ash window state"))
		}
		if windowState != ash.WindowStateMaximized {
			return errors.New("the window isn't expanded yet")
		}

		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second, Interval: 500 * time.Millisecond})
}

// waitForPIPWindow keeps looking for a PIP window until it appears on the Chrome side.
func waitForPIPWindow(ctx context.Context, tconn *chrome.TestConn) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		_, err := getPIPWindow(ctx, tconn)
		if err != nil {
			return errors.Wrap(err, "The PIP window hasn't been created yet")
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second})
}

// getPIPWindow returns the PIP window if any.
func getPIPWindow(ctx context.Context, tconn *chrome.TestConn) (*ash.Window, error) {
	return ash.FindWindow(ctx, tconn, func(w *ash.Window) bool { return w.State == ash.WindowStatePIP })
}

// getSystemUIRect returns the rect whose window corresponds to className on the Chrome window hierarchy.
// As it's possible that it takes some time for the window to show up and get synced to API, we try a few times until we get a valid bounds.
func getSystemUIRect(ctx context.Context, tconn *chrome.TestConn, className string, timeout time.Duration) (coords.Rect, error) {
	// Get UI root.
	root, err := chromeui.Root(ctx, tconn)
	if err != nil {
		return coords.Rect{}, err
	}
	defer root.Release(ctx)
	// Find the node with className.
	window, err := root.DescendantWithTimeout(ctx, chromeui.FindParams{ClassName: className}, timeout)
	if err != nil {
		return coords.Rect{}, err
	}
	defer window.Release(ctx)
	return coords.Rect{window.Location.Left, window.Location.Top, window.Location.Width, window.Location.Height}, nil
}

// getStatusAreaRect returns Chrome OS's Status Area rect, in DPs.
// Returns error if Status Area is not present.
func getStatusAreaRect(ctx context.Context, tconn *chrome.TestConn, timeout time.Duration) (coords.Rect, error) {
	return getSystemUIRect(ctx, tconn, "BubbleFrameView", timeout)
}

// showSystemStatusArea shows the System Status Area in case it is not already shown.
func showSystemStatusArea(ctx context.Context, tconn *chrome.TestConn) error {
	// Already visible ?
	if _, err := getStatusAreaRect(ctx, tconn, time.Second); err == nil {
		return nil
	}

	if err := toggleSystemStatusArea(ctx, tconn); err != nil {
		return err
	}

	return testing.Poll(ctx, func(ctx context.Context) error {
		_, err := getStatusAreaRect(ctx, tconn, time.Second)
		if err != nil {
			return errors.Wrap(err, "The system status area hasn't been created yet")
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second})
}

// hideSystemStatusArea hides the System Status Area in case it is not already hidden.
func hideSystemStatusArea(ctx context.Context, tconn *chrome.TestConn) error {
	// Already hidden ?
	if _, err := getStatusAreaRect(ctx, tconn, time.Second); err != nil {
		return nil
	}

	if err := toggleSystemStatusArea(ctx, tconn); err != nil {
		return err
	}

	return testing.Poll(ctx, func(ctx context.Context) error {
		_, err := getStatusAreaRect(ctx, tconn, time.Second)
		// Once the window gets hidden, getStatusAreaRect should return error.
		if err == nil {
			return errors.Wrap(err, "The system status area hasn't been hidden yet")
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second})
}

// pressShelfIcon press the shelf icon of PIP window.
func pressShelfIcon(ctx context.Context, tconn *chrome.TestConn) error {
	root, err := chromeui.Root(ctx, tconn)
	if err != nil {
		return err
	}
	defer root.Release(ctx)

	var icon *chromeui.Node
	// Make sure that at least one shelf icon exists.
	// Depending the test order, the status area might not be ready at this point.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		icon, err = root.DescendantWithTimeout(ctx, chromeui.FindParams{Name: "ArcPipTastTest", ClassName: "ash/ShelfAppButton"}, 15*time.Second)
		if err != nil {
			return errors.Wrap(err, "no shelf icon has been created yet")
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		return errors.Wrap(err, "failed to locate shelf icons")
	}
	defer icon.Release(ctx)

	return icon.LeftClick(ctx)
}

// toggleSystemStatusArea toggles Chrome OS's system status area.
func toggleSystemStatusArea(ctx context.Context, tconn *chrome.TestConn) error {
	// A reliable way to toggle the status area is by injecting Alt+Shift+s. But on tablet mode
	// it doesn't work since the keyboard is disabled.
	// Instead, we click on the StatusAreaWidgetDelegate.
	root, err := chromeui.Root(ctx, tconn)
	if err != nil {
		return err
	}
	defer root.Release(ctx)
	widget, err := root.DescendantWithTimeout(ctx, chromeui.FindParams{ClassName: "ash/StatusAreaWidgetDelegate"}, 10*time.Second)
	if err != nil {
		return errors.Wrap(err, "failed to get status area widget")
	}
	defer widget.Release(ctx)
	return widget.LeftClick(ctx)
}

// waitForNewBoundsWithMargin waits until Chrome animation finishes completely and check the position of an edge of the PIP window.
// More specifically, this checks the edge of the window bounds specified by the border parameter matches the expectedValue parameter,
// allowing an error within the margin parameter.
// The window bounds is returned in DP, so dsf is used to convert it to PX.
func waitForNewBoundsWithMargin(ctx context.Context, tconn *chrome.TestConn, expectedValue int, border borderType, dsf float64, margin int) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		window, err := getPIPWindow(ctx, tconn)
		if err != nil {
			return errors.New("failed to Get PIP window")
		}
		bounds := window.BoundsInRoot
		isAnimating := window.IsAnimating

		if isAnimating {
			return errors.New("the window is still animating")
		}

		var currentValue int
		switch border {
		case left:
			currentValue = int(math.Round(float64(bounds.Left) * dsf))
		case top:
			currentValue = int(math.Round(float64(bounds.Top) * dsf))
		case right:
			currentValue = int(math.Round(float64(bounds.Left+bounds.Width) * dsf))
		case bottom:
			currentValue = int(math.Round(float64(bounds.Top+bounds.Height) * dsf))
		default:
			return testing.PollBreak(errors.Errorf("unknown border type %v", border))
		}
		if currentValue < expectedValue-margin || expectedValue+margin < currentValue {
			return errors.Errorf("the PIP window doesn't have the expected bounds yet; got %d, want %d", currentValue, expectedValue)
		}

		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second})
}
