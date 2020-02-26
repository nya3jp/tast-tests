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

			type testFunc func(context.Context, *chrome.Conn, *arc.ARC, *arc.Activity, *ui.Device, *display.DisplayMode) error

			for idx, test := range []struct {
				name       string
				fn         testFunc
				initMethod initializationType
			}{
				{name: "PIP Move", fn: testPIPMove, initMethod: enterPip},
				{name: "PIP Resize", fn: testPIPResize, initMethod: enterPip},
				// Disable testPIPFling as there's no reliable way to trigger swip from Tast.
				// TODO(crbug.com/1014832): Add a private autotest API for this and reenable the test.
				// {name: "PIP Fling", fn: testPIPFling, initMethod: enterPip},
				{name: "PIP GravityStatusArea", fn: testPIPGravityStatusArea, initMethod: enterPip},
				{name: "PIP GravityShelfAutoHide", fn: testPIPGravityShelfAutoHide, initMethod: enterPip},
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
					// Press button that triggers PIP mode in activity.
					const pipButtonID = pipTestPkgName + ":id/enter_pip_button"
					must(dev.Object(ui.ID(pipButtonID)).Click(ctx))
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
func testPIPMove(ctx context.Context, tconn *chrome.Conn, a *arc.ARC, pipAct *arc.Activity, dev *ui.Device, dispMode *display.DisplayMode) error {
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
	origBounds := ash.ConvertBoundsFromDpToPx(window.BoundsInRoot, dispMode.DeviceScaleFactor)
	testing.ContextLogf(ctx, "Initial PIP bounds: %+v", origBounds)

	deltaX := dispMode.WidthInNativePixels / (totalMovements + 1)
	for i := 0; i < totalMovements; i++ {
		newBounds := origBounds
		newBounds.Left -= deltaX * (i + 1)
		testing.ContextLogf(ctx, "Moving PIP window to %d,%d", newBounds.Left, newBounds.Top)
		if err := pipAct.MoveWindow(ctx, arc.NewPoint(newBounds.Left, newBounds.Top), movementDuration); err != nil {
			return errors.Wrap(err, "could not move PIP window")
		}

		if err = waitForNewBoundsWithMargin(ctx, tconn, newBounds.Left, left, dispMode.DeviceScaleFactor, pipPositionErrorMarginPX+missedByGestureControllerPX); err != nil {
			return errors.Wrap(err, "failed to move PIP to left")
		}
	}
	return nil
}

// testPIPResize verifies that resizing the PIP window works as expected.
// It performs a drag-resize from PIP's left-top corner and compares the resized-PIP size with the expected one.
func testPIPResize(ctx context.Context, tconn *chrome.Conn, a *arc.ARC, pipAct *arc.Activity, dev *ui.Device, dispMode *display.DisplayMode) error {
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
	bounds := ash.ConvertBoundsFromDpToPx(window.BoundsInRoot, dispMode.DeviceScaleFactor)
	testing.ContextLogf(ctx, "Bounds before resize: %+v", bounds)

	testing.ContextLog(ctx, "Resizing window to x=0, y=0")
	// Resizing PIP to x=0, y=0, but it should stop once it reaches its max size.
	if err := pipAct.ResizeWindow(ctx, arc.BorderTopLeft, arc.NewPoint(0, 0), time.Second); err != nil {
		return errors.Wrap(err, "could not resize PIP window")
	}

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
	right := bounds.Left + bounds.Width
	bottom := bounds.Top + bounds.Height

	if pipMaxSizeH < pipMaxSizeW {
		if err = waitForNewBoundsWithMargin(ctx, tconn, bottom-pipMaxSizeH, top, dispMode.DeviceScaleFactor, pipPositionErrorMarginPX); err != nil {
			return errors.Wrap(err, "the maximum size of the PIP window must be half of the display height")
		}
	} else {
		if err = waitForNewBoundsWithMargin(ctx, tconn, right-pipMaxSizeW, left, dispMode.DeviceScaleFactor, pipPositionErrorMarginPX); err != nil {
			return errors.Wrap(err, "the maximum size of the PIP window must be half of the display width")
		}
	}

	return nil
}

// testPIPFling tests that fling works as expected. It tests the fling gesture in four directions: left, up, right and down.
func testPIPFling(ctx context.Context, tconn *chrome.Conn, a *arc.ARC, act *arc.Activity, dev *ui.Device, dispMode *display.DisplayMode) error {
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
		info, err := ash.GetARCAppWindowInfo(ctx, tconn, pipTestPkgName)
		if err != nil {
			return errors.Wrap(err, "could not get PIP window bounds")
		}
		bounds := ash.ConvertBoundsFromDpToPx(info.BoundsInRoot, dispMode.DeviceScaleFactor)

		pipCenterX := float64(bounds.Left + bounds.Width/2)
		pipCenterY := float64(bounds.Top + bounds.Height/2)

		// (x0, y0) is the initial location of the window and (x2, y2) is the final destination of this series of swipe.
		// We first swipe the window slowly by 1/6 of the diplay length and then swipe fast by 1/3 of the display length.
		// So, in total, the window is swiped 1/2 of the display length in each direction.
		// (x0, y0) -- <First Swipe (slow)> --> (x1, y1) ------ <Second Swipe (fast)> ------> (x2, y2)
		x0 := input.TouchCoord(pipCenterX * pixelToTuxelX)
		y0 := input.TouchCoord(pipCenterY * pixelToTuxelY)
		x1 := input.TouchCoord((pipCenterX + float64(dir.x*dispW/6)) * pixelToTuxelX)
		y1 := input.TouchCoord((pipCenterY + float64(dir.y*dispH/6)) * pixelToTuxelY)
		x2 := input.TouchCoord((pipCenterX + float64(dir.x*(dispW/6+dispW/3))) * pixelToTuxelX)
		y2 := input.TouchCoord((pipCenterY + float64(dir.y*(dispH/6+dispH/3))) * pixelToTuxelY)

		testing.ContextLogf(ctx, "Running the first swipe gesture from {%d,%d} to {%d,%d} to ensure to start drag move", x0, y0, x1, y1)
		// Swipe the PIP window slowly first to ensure to start drag move.
		if err := stw.Swipe(ctx, x0, y0, x1, y1, time.Second); err != nil {
			return errors.Wrap(err, "failed to execute a swipe gesture")
		}

		testing.ContextLogf(ctx, "Running the second swipe gesture from {%d,%d} to {%d,%d} to fling PIP", x1, y1, x2, y2)
		// The swipe duration needs to be faster than 200 milliseconds. Otherwise, the swipe gesture could be handled as regular drag move.
		if err := stw.Swipe(ctx, x1, y1, x2, y2, 50*time.Millisecond); err != nil {
			return errors.Wrap(err, "failed to execute a swipe gesture")
		}
		if err := stw.End(); err != nil {
			return errors.Wrap(err, "failed to finish the swipe gesture")
		}

		switch dir.border {
		case left:
			if err = waitForNewBoundsWithMargin(ctx, tconn, collisionWindowWorkAreaInsetsPX, left, dispMode.DeviceScaleFactor, pipPositionErrorMarginPX); err != nil {
				return errors.Wrap(err, "failed swipe to left")
			}
		case right:
			if err = waitForNewBoundsWithMargin(ctx, tconn, dispW-collisionWindowWorkAreaInsetsPX, right, dispMode.DeviceScaleFactor, pipPositionErrorMarginPX); err != nil {
				return errors.Wrap(err, "failed swipe to right")
			}
		case top:
			if err = waitForNewBoundsWithMargin(ctx, tconn, collisionWindowWorkAreaInsetsPX, top, dispMode.DeviceScaleFactor, pipPositionErrorMarginPX); err != nil {
				return errors.Wrap(err, "failed swipe to top")
			}
		case bottom:
			rectDP, err := getShelfRect(ctx, tconn)
			if err != nil {
				return errors.Wrap(err, "failed to get shelf rect")
			}
			shelfTopPX := int(math.Round(float64(rectDP.Top) * dispMode.DeviceScaleFactor))
			if err = waitForNewBoundsWithMargin(ctx, tconn, shelfTopPX-collisionWindowWorkAreaInsetsPX, bottom, dispMode.DeviceScaleFactor, pipPositionErrorMarginPX); err != nil {
				return errors.Wrap(err, "failed swipe to bottom")
			}
		}
	}
	return nil
}

// testPIPGravityStatusArea tests that PIP windows moves accordingly when the status area is hidden / displayed.
func testPIPGravityStatusArea(ctx context.Context, tconn *chrome.Conn, a *arc.ARC, pipAct *arc.Activity, dev *ui.Device, dispMode *display.DisplayMode) error {
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
	bounds := ash.ConvertBoundsFromDpToPx(window.BoundsInRoot, dispMode.DeviceScaleFactor)

	testing.ContextLog(ctx, "Hiding system status area")
	if err := hideSystemStatusArea(ctx, tconn); err != nil {
		return err
	}

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

	statusRectDP, err := getStatusAreaRect(ctx, tconn)
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

// testPIPGravityShelfAutoHide tests that PIP windows moves accordingly when the shelf is hidden / displayed.
func testPIPGravityShelfAutoHide(ctx context.Context, tconn *chrome.Conn, a *arc.ARC, pipAct *arc.Activity, dev *ui.Device, dispMode *display.DisplayMode) error {
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

	// 1) PIP window should be above the shelf on the Y-axis.

	window, err := getPIPWindow(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "could not get PIP window")
	}
	origBounds := ash.ConvertBoundsFromDpToPx(window.BoundsInRoot, dispMode.DeviceScaleFactor)

	collisionWindowWorkAreaInsetsPX := int(math.Round(collisionWindowWorkAreaInsetsDP * dispMode.DeviceScaleFactor))
	testing.ContextLog(ctx, "Using: collisionWindowWorkAreaInsetsPX = ", collisionWindowWorkAreaInsetsPX)

	if err = waitForNewBoundsWithMargin(ctx, tconn, shelfTopPX-collisionWindowWorkAreaInsetsPX, bottom, dispMode.DeviceScaleFactor, pipPositionErrorMarginPX); err != nil {
		return errors.Wrap(err, "the PIP window must be above the always-show shelf on the Y-axis")
	}

	// 2) PIP window should not fall down when the shelf disappears. Since by default it is "gravity-less".

	testing.ContextLogf(ctx, "Setting shelf auto hide = %q", ash.ShelfBehaviorAlwaysAutoHide)
	if err := ash.SetShelfBehavior(ctx, tconn, dispInfo.ID, ash.ShelfBehaviorAlwaysAutoHide); err != nil {
		return errors.Wrapf(err, "failed to set shelf behavior to %q", ash.ShelfBehaviorAlwaysAutoHide)
	}
	// On exit restore to NeverAutoHide no matter what.
	defer ash.SetShelfBehavior(ctx, tconn, dispInfo.ID, ash.ShelfBehaviorNeverAutoHide)

	// The PIP window shouldn't move as the initial gravity direction is "right".
	if err = waitForNewBoundsWithMargin(ctx, tconn, origBounds.Top+origBounds.Height, bottom, dispMode.DeviceScaleFactor, pipPositionErrorMarginPX); err != nil {
		return errors.Wrap(err, "the PIP window must not move when system status area gets hidden")
	}

	// 3) PIP should fall-down ('down' gravity) after being moved to the center of the screen.

	// Set shelf to visible again.
	if err := ash.SetShelfBehavior(ctx, tconn, dispInfo.ID, ash.ShelfBehaviorNeverAutoHide); err != nil {
		return errors.Wrapf(err, "failed to set shelf behavior to %q", ash.ShelfBehaviorNeverAutoHide)
	}
	newX := dispMode.WidthInNativePixels / 2
	if err := pipAct.MoveWindow(ctx, arc.NewPoint(newX, origBounds.Top), 2*time.Second); err != nil {
		return errors.Wrapf(err, "failed to move PIP window to (%d, %d)", newX, origBounds.Top)
	}

	if err = waitForNewBoundsWithMargin(ctx, tconn, origBounds.Top+origBounds.Height, bottom, dispMode.DeviceScaleFactor, pipPositionErrorMarginPX); err != nil {
		return errors.Wrap(err, "the PIP window must be above the always-show shelf on the Y-axis")
	}

	// Set shelf to auto-hide again, causing the PIP window to fall down.
	if err := ash.SetShelfBehavior(ctx, tconn, dispInfo.ID, ash.ShelfBehaviorAlwaysAutoHide); err != nil {
		return errors.Wrapf(err, "failed to set shelf behavior to %q", ash.ShelfBehaviorNeverAutoHide)
	}

	// Shelf takes up a few pixels at the bottom of the display even when it's hidden, so we need to take this into account.
	// TODO(takise): Remove this once the bug has been fixed.
	const hiddenShelfHeightDP = 3
	hiddenShelfHeightPX := int(math.Round(float64(hiddenShelfHeightDP) * dispMode.DeviceScaleFactor))
	testing.ContextLogf(ctx, "Setting shelf auto hide = %q", ash.ShelfBehaviorNeverAutoHide)
	if err = waitForNewBoundsWithMargin(ctx, tconn, dispMode.HeightInNativePixels-hiddenShelfHeightPX-collisionWindowWorkAreaInsetsPX, bottom, dispMode.DeviceScaleFactor, pipPositionErrorMarginPX); err != nil {
		return errors.Wrap(err, "the PIP window must be above the auto-hide shelf on the Z-axis")
	}

	// 4) PIP window should go up when the shelf reappears.

	testing.ContextLogf(ctx, "Setting shelf auto hide = %q", ash.ShelfBehaviorNeverAutoHide)
	if err := ash.SetShelfBehavior(ctx, tconn, dispInfo.ID, ash.ShelfBehaviorNeverAutoHide); err != nil {
		return errors.Wrapf(err, "failed to set shelf behavior to %q", ash.ShelfBehaviorNeverAutoHide)
	}

	if err = waitForNewBoundsWithMargin(ctx, tconn, origBounds.Top+origBounds.Height, bottom, dispMode.DeviceScaleFactor, pipPositionErrorMarginPX); err != nil {
		return errors.Wrap(err, "the PIP window must move upwords when shelf becomes always-shown")
	}

	return nil
}

// testPIPToggleTabletMode verifies that the window position is the same after toggling tablet mode.
func testPIPToggleTabletMode(ctx context.Context, tconn *chrome.Conn, a *arc.ARC, act *arc.Activity, dev *ui.Device, dispMode *display.DisplayMode) error {
	info, err := ash.GetARCAppWindowInfo(ctx, tconn, pipTestPkgName)
	if err != nil {
		return errors.Wrap(err, "could not get PIP window bounds")
	}
	origBounds := ash.ConvertBoundsFromDpToPx(info.BoundsInRoot, dispMode.DeviceScaleFactor)
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
func testPIPAutoPIPMinimize(ctx context.Context, tconn *chrome.Conn, a *arc.ARC, pipAct *arc.Activity, dev *ui.Device, dispMode *display.DisplayMode) error {
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
func testPIPExpandViaMenuTouch(ctx context.Context, tconn *chrome.Conn, a *arc.ARC, pipAct *arc.Activity, dev *ui.Device, dispMode *display.DisplayMode) error {
	if err := expandPIPViaMenuTouch(ctx, tconn, pipAct, dev, dispMode); err != nil {
		return errors.Wrap(err, "could not expand PIP")
	}

	return ash.WaitForARCAppWindowState(ctx, tconn, pipTestPkgName, ash.WindowStateMaximized)
}

// testPIPExpandViaShelfIcon verifies that PIP window is properly expanded by pressing shelf icon.
func testPIPExpandViaShelfIcon(ctx context.Context, tconn *chrome.Conn, a *arc.ARC, pipAct *arc.Activity, dev *ui.Device, dispMode *display.DisplayMode) error {
	if err := pressShelfIcon(ctx, tconn); err != nil {
		return errors.Wrap(err, "could not expand PIP")
	}

	return ash.WaitForARCAppWindowState(ctx, tconn, pipTestPkgName, ash.WindowStateMaximized)
}

// testPIPAutoPIPNewAndroidWindow verifies that creating a new Android window that occludes an auto-PIP window will trigger PIP.
func testPIPAutoPIPNewAndroidWindow(ctx context.Context, tconn *chrome.Conn, a *arc.ARC, pipAct *arc.Activity, dev *ui.Device, dispMode *display.DisplayMode) error {
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
func testPIPAutoPIPNewChromeWindow(ctx context.Context, tconn *chrome.Conn, a *arc.ARC, pipAct *arc.Activity, dev *ui.Device, dispMode *display.DisplayMode) error {
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
func expandPIPViaMenuTouch(ctx context.Context, tconn *chrome.Conn, act *arc.Activity, dev *ui.Device, dispMode *display.DisplayMode) error {
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
	bounds := ash.ConvertBoundsFromDpToPx(window.BoundsInRoot, dispMode.DeviceScaleFactor)

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
func waitForPIPWindow(ctx context.Context, tconn *chrome.Conn) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		_, err := getPIPWindow(ctx, tconn)
		if err != nil {
			return errors.Wrap(err, "The PIP window hasn't been created yet")
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second})
}

// getPIPWindow returns the PIP window if any.
func getPIPWindow(ctx context.Context, tconn *chrome.Conn) (*ash.Window, error) {
	return ash.FindWindow(ctx, tconn, func(w *ash.Window) bool { return w.State == ash.WindowStatePIP })
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

// pressShelfIcon press the shelf icon of PIP window.
func pressShelfIcon(ctx context.Context, tconn *chrome.Conn) error {
	// There can be multiple icons on the shelf, but currently there's no way to find a specific icon.
	// So here we assume the shelf icon of the PIP window is located on the right-most position on the shelf.
	// This is currently true given that our apk is launched last, our apk is not pinned, and there are not too many apps opened.
	err := tconn.EvalPromise(ctx,
		`new Promise((resolve, reject) => {
		  chrome.automation.getDesktop(function(root) {
		    const icons = root.findAll({ attributes: { className: 'ash/ShelfAppButton'}});
		    if (!icons || icons.length === 0) {
		      reject("Failed to locate icon");
		      return;
		    }
		    icons[icons.length - 1].doDefault();
		    resolve();
		  })
		})`, nil)
	if err != nil {
		return errors.Wrap(err, "failed to find ShelfAppButton")
	}
	return nil
}

// toggleSystemStatusArea toggles Chrome OS's system status area.
func toggleSystemStatusArea(ctx context.Context, tconn *chrome.Conn) error {
	// A reliable way to toggle the status area is by injecting Alt+Shift+s. But on tablet mode
	// it doesn't work since the keyboard is disabled.
	// Instead, we click ("doDefault()") on the StatusAreaWidget's button. The problem is that in tablet mode
	// there are two buttons and we cannot identify them in a reliable way. We assume that the first button
	// in the StatusAreaWidget hierarchy is the one that toggles the status area.
	// TODO(ricardoq): Find a reliable way to find "status tray" button.
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
		    button.doDefault();
		    resolve();
		  })
		})`, nil)
	if err != nil {
		return errors.Wrap(err, "failed to find StatusAreaWidget")
	}
	return nil
}

// waitForNewBoundsWithMargin waits until Chrome animation finishes completely and check the position of an edge of the PIP window.
// More specifically, this checks the edge of the window bounds specified by the border parameter matches the expectedValue parameter,
// allowing an error within the margin parameter.
// The window bounds is returned in DP, so dsf is used to convert it to PX.
func waitForNewBoundsWithMargin(ctx context.Context, tconn *chrome.Conn, expectedValue int, border borderType, dsf float64, margin int) error {
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
