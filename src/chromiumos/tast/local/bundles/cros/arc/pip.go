// Copyright 2019 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"fmt"
	"math"
	"time"

	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arc/wm"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/display"
	uiauto "chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/quicksettings"
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
	missedByGestureControllerDP = 50
)

type borderType int

const (
	left borderType = iota
	right
	top
	bottom
)

type initializationType uint

const (
	doNothing initializationType = iota
	startActivity
	enterPip
)

type pipTestFunc func(context.Context, *chrome.Chrome, *chrome.TestConn, *arc.ARC, *arc.Activity, *ui.Device, *display.DisplayMode) error

type pipTestParams struct {
	name       string
	fn         pipTestFunc
	initMethod initializationType
	bT         browser.Type
}

var pipVMTests = []pipTestParams{
	{name: "PIP Move", fn: testPIPMove, initMethod: enterPip, bT: browser.TypeAsh},
	{name: "PIP Resize To Max", fn: testPIPResizeToMax, initMethod: enterPip, bT: browser.TypeAsh},
	{name: "PIP GravityQuickSettings", fn: testPIPGravityQuickSettings, initMethod: enterPip, bT: browser.TypeAsh},
	{name: "PIP AutoPIP New Chrome Window", fn: testPIPAutoPIPNewChromeWindow, initMethod: startActivity, bT: browser.TypeAsh},
	{name: "PIP AutoPIP New Android Window", fn: testPIPAutoPIPNewAndroidWindow, initMethod: doNothing, bT: browser.TypeAsh},
	{name: "PIP AutoPIP Minimize", fn: testPIPAutoPIPMinimize, initMethod: startActivity, bT: browser.TypeAsh},
	{name: "PIP ExpandPIP Shelf Icon", fn: testPIPExpandViaShelfIcon, initMethod: startActivity, bT: browser.TypeAsh},
	{name: "PIP ExpandPIP Menu Touch", fn: testPIPExpandViaMenuTouch, initMethod: startActivity, bT: browser.TypeAsh},
	{name: "PIP Toggle Tablet mode", fn: testPIPToggleTabletMode, initMethod: enterPip, bT: browser.TypeAsh},
}
var pipVMLacrosTests = []pipTestParams{
	{name: "PIP AutoPIP New Chrome Window", fn: testPIPAutoPIPNewChromeWindow, initMethod: startActivity, bT: browser.TypeLacros},
}

// TODO(b/249015149): Fix flakiness and readd missing tests.
var pipContainerTests = []pipTestParams{
	{name: "PIP Move", fn: testPIPMove, initMethod: enterPip, bT: browser.TypeAsh},
	{name: "PIP Resize To Max", fn: testPIPResizeToMax, initMethod: enterPip, bT: browser.TypeAsh},
	{name: "PIP GravityQuickSettings", fn: testPIPGravityQuickSettings, initMethod: enterPip, bT: browser.TypeAsh},
	{name: "PIP AutoPIP New Chrome Window", fn: testPIPAutoPIPNewChromeWindow, initMethod: startActivity, bT: browser.TypeAsh},
	{name: "PIP AutoPIP Minimize", fn: testPIPAutoPIPMinimize, initMethod: startActivity, bT: browser.TypeAsh},
	{name: "PIP ExpandPIP Menu Touch", fn: testPIPExpandViaMenuTouch, initMethod: startActivity, bT: browser.TypeAsh},
	{name: "PIP Toggle Tablet mode", fn: testPIPToggleTabletMode, initMethod: enterPip, bT: browser.TypeAsh},
}
var pipContainerLacrosTests = []pipTestParams{
	{name: "PIP AutoPIP New Chrome Window", fn: testPIPAutoPIPNewChromeWindow, initMethod: startActivity, bT: browser.TypeLacros},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         PIP,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checks that ARC++ Picture-in-Picture works as expected",
		Contacts:     []string{"takise@chromium.org", "arc-framework+tast@google.com", "cros-arc-te@google.com"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:arc-functional"},
		Timeout:      4 * time.Minute,
		Params: []testing.Param{{
			Val:               pipContainerTests,
			ExtraAttr:         []string{"group:mainline", "informational"},
			ExtraSoftwareDeps: []string{"android_p"},
			Fixture:           "arcBooted",
		}, {
			Name:              "lacros",
			Val:               pipContainerLacrosTests,
			ExtraAttr:         []string{"group:mainline", "informational"},
			ExtraSoftwareDeps: []string{"android_p", "lacros"},
			Fixture:           "lacrosWithArcBooted",
		}, {
			Name:              "vm",
			Val:               pipVMTests,
			ExtraAttr:         []string{"group:mainline", "informational"},
			ExtraSoftwareDeps: []string{"android_vm"},
			Fixture:           "arcBooted",
		}, {
			Name:              "lacros_vm",
			Val:               pipVMLacrosTests,
			ExtraAttr:         []string{"group:mainline", "informational"},
			ExtraSoftwareDeps: []string{"android_vm", "lacros"},
			Fixture:           "lacrosWithArcBooted",
		}},
	})
}

func PIP(ctx context.Context, s *testing.State) {
	// For debugging, create a Chrome session with chrome.ExtraArgs("--show-taps")
	cr := s.FixtValue().(*arc.PreData).Chrome

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	a := s.FixtValue().(*arc.PreData).ARC
	dev := s.FixtValue().(*arc.PreData).UIDevice

	const apkName = "ArcPipTest.apk"
	if err := a.Install(ctx, arc.APKPath(apkName)); err != nil {
		s.Fatal("Failed installing PIP app: ", err)
	}
	defer a.Uninstall(ctx, pipTestPkgName)

	if err := a.Install(ctx, arc.APKPath(wm.APKNameArcWMTestApp24)); err != nil {
		s.Fatal("Failed installing WM24 app: ", err)
	}
	defer a.Uninstall(ctx, wm.Pkg24)

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

	dispInfo, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get primary display info: ", err)
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

	dispMode, err := ash.PrimaryDisplayMode(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get display mode: ", err)
	}

	tabletModes := []bool{false, true}
	enableMultiActivityPIP := []bool{true, false}
	sdkVer, err := arc.SDKVersion()
	if err != nil {
		s.Fatal("Failed to get the SDK version: ", err)
	}

	// Run all subtests twice. First, with tablet mode disabled. And then, with it enabled.
	for _, tabletMode := range tabletModes {
		s.Logf("Running tests with tablet mode enabled=%t", tabletMode)
		if err := ash.SetTabletModeEnabled(ctx, tconn, tabletMode); err != nil {
			s.Fatalf("Failed to set tablet mode enabled to %t: %v", tabletMode, err)
		}

		// There are two types of PIP: single activity PIP and multi activity PIP. Run each test with both types by default.
		for _, multiActivityPIP := range enableMultiActivityPIP {
			if !multiActivityPIP && tabletMode && sdkVer == arc.SDKR {
				// TODO(b:156685602) There are still some tests not yet working in tablet mode. Remove these checks once R is fully working.
				continue
			}

			s.Logf("Running tests with tablet mode enabled=%t and MAPIP enabled=%t", tabletMode, multiActivityPIP)
			for idx, test := range s.Param().([]pipTestParams) {
				testing.ContextLog(ctx, "About to run test: ", test.name)
				if err := testPIPInternal(ctx, s, cr, tconn, a, pipAct, maPIPBaseAct, dev, dispMode, test, tabletMode, multiActivityPIP, idx); err != nil {
					s.Errorf("%s test with tablet mode(%t) and multi-activity(%t) failed: %v", test.name, tabletMode, multiActivityPIP, err)
				}
			}
		}
	}
}

// testPIPInternal ...
func testPIPInternal(ctx context.Context, s *testing.State, cr *chrome.Chrome, tconn *chrome.TestConn, a *arc.ARC, pipAct, maPIPBaseAct *arc.Activity, dev *ui.Device, dispMode *display.DisplayMode, test pipTestParams, tabletMode, multiActivityPIP bool, idx int) error {
	if test.initMethod == startActivity || test.initMethod == enterPip {
		if multiActivityPIP {
			if err := maPIPBaseAct.Start(ctx, tconn); err != nil {
				return errors.Wrapf(err, "failed to start %s", maPIPBaseAct.ActivityName())
			}
			defer maPIPBaseAct.Stop(ctx, tconn)
		}

		if err := pipAct.Start(ctx, tconn); err != nil {
			return errors.Wrapf(err, "failed to start %s", pipAct.ActivityName())
		}
		defer pipAct.Stop(ctx, tconn)

		if multiActivityPIP {
			// Wait for pipAct to finish settling on top of the base activity. Minimize could be called before on the base activity
			// otherwise.
			if err := testing.Sleep(ctx, time.Second); err != nil {
				return errors.Wrap(err, "failed to sleep waiting for MAPIP")
			}
		}
	}

	if test.initMethod == enterPip {
		// Make the app PIP via minimize.
		// We have some other ways to PIP an app, but for now this is the most reliable.
		testing.ContextLog(ctx, "Test requires PIP initial state, entering PIP via minimize")
		if err := minimizePIP(ctx, tconn, pipAct); err != nil {
			return errors.Wrap(err, "failed to minimize app into PIP")
		}
		if err := waitForPIPWindow(ctx, tconn); err != nil {
			return errors.Wrap(err, "did not enter PIP mode")
		}
	}

	if err := test.fn(ctx, cr, tconn, a, pipAct, dev, dispMode); err != nil {
		path := fmt.Sprintf("%s/screenshot-pip-failed-test-%d.png", s.OutDir(), idx)
		if err := screenshot.CaptureChrome(ctx, cr, path); err != nil {
			s.Log("Failed to capture screenshot: ", err)
			return err
		}
		return err
	}
	return nil
}

// testPIPMove verifies that drag-moving the PIP window works as expected.
// It does that by drag-moving that PIP window horizontally 3 times and verifying that the position is correct.
func testPIPMove(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn, a *arc.ARC, pipAct *arc.Activity, dev *ui.Device, dispMode *display.DisplayMode) error {
	const (
		movementDuration = 2 * time.Second
		totalMovements   = 3
	)

	missedByGestureControllerPX := int(math.Round(missedByGestureControllerDP * dispMode.DeviceScaleFactor))
	testing.ContextLog(ctx, "Using: missedByGestureControllerPX = ", missedByGestureControllerPX)

	if err := waitForPIPWindow(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to wait for PIP window")
	}
	window, err := getPIPWindow(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "could not get PIP window")
	}
	origBounds := coords.ConvertBoundsFromDPToPX(window.BoundsInRoot, dispMode.DeviceScaleFactor)
	testing.ContextLogf(ctx, "Initial PIP bounds: %+v", origBounds)

	deltaX := dispMode.WidthInNativePixels / (totalMovements + 1)
	for i := 0; i < totalMovements; i++ {
		newWindow, err := getPIPWindow(ctx, tconn)
		movedBounds := coords.ConvertBoundsFromDPToPX(newWindow.BoundsInRoot, dispMode.DeviceScaleFactor)
		newBounds := movedBounds
		newBounds.Left -= deltaX
		if err := pipAct.MoveWindow(ctx, tconn, movementDuration, newBounds, movedBounds); err != nil {
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
func testPIPResizeToMax(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn, a *arc.ARC, pipAct *arc.Activity, dev *ui.Device, dispMode *display.DisplayMode) error {
	// Activate PIP "resize handler", otherwise resize will fail. See:
	// https://android.googlesource.com/platform/frameworks/base/+/refs/heads/pie-release/services/core/java/com/android/server/policy/PhoneWindowManager.java#6387
	if err := dev.PressKeyCode(ctx, ui.KEYCODE_WINDOW, 0); err != nil {
		return errors.Wrap(err, "could not activate PIP menu")
	}

	window, err := getPIPWindow(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "could not get PIP window")
	}
	bounds := coords.ConvertBoundsFromDPToPX(window.BoundsInRoot, dispMode.DeviceScaleFactor)
	testing.ContextLogf(ctx, "Bounds before resize: %+v", bounds)

	testing.ContextLog(ctx, "Resizing window to x=0, y=0")
	// Resizing PIP to x=0, y=0, but it should stop once it reaches its max size.
	if err := pipAct.ResizeWindow(ctx, tconn, arc.BorderTopLeft, coords.NewPoint(0, 0), time.Second); err != nil {
		return errors.Wrap(err, "could not resize PIP window")
	}

	// Retrieve the PIP bounds again.
	window, err = getPIPWindow(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "could not get PIP window")
	}
	bounds = coords.ConvertBoundsFromDPToPX(window.BoundsInRoot, dispMode.DeviceScaleFactor)

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

// testPIPGravityQuickSettings tests that PIP windows moves accordingly when Quick Settings is hidden / displayed.
func testPIPGravityQuickSettings(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn, a *arc.ARC, pipAct *arc.Activity, dev *ui.Device, dispMode *display.DisplayMode) error {
	// testPIPGravityQuickSettings verifies that:
	// 1) The PIP window moves to the left of the Quick Settings area when it is shown.
	// 2) The PIP window returns close the right border when the Quick Settings area is dismissed.

	collisionWindowWorkAreaInsetsPX := int(math.Round(collisionWindowWorkAreaInsetsDP * dispMode.DeviceScaleFactor))
	testing.ContextLog(ctx, "Using: collisionWindowWorkAreaInsetsPX = ", collisionWindowWorkAreaInsetsPX)

	// 0) Validity check. Verify that PIP window is in the expected initial position and that Quick Settings is hidden.

	if err := waitForPIPWindow(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to wait for PIP window")
	}
	window, err := getPIPWindow(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "could not get PIP window")
	}
	bounds := coords.ConvertBoundsFromDPToPX(window.BoundsInRoot, dispMode.DeviceScaleFactor)

	dispInfo, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "could not get primary display info")
	}
	dispBounds := coords.ConvertBoundsFromDPToPX(dispInfo.Bounds, dispMode.DeviceScaleFactor)

	if err = waitForNewBoundsWithMargin(ctx, tconn, dispBounds.Width-collisionWindowWorkAreaInsetsPX, right, dispMode.DeviceScaleFactor, pipPositionErrorMarginPX); err != nil {
		return errors.Wrap(err, "the PIP window must be along the right edge of the display")
	}

	// 1) The PIP window should move to the left of the Quick Settings area.

	testing.ContextLog(ctx, "Showing Quick Settings area")
	if err := quicksettings.Show(ctx, tconn); err != nil {
		return err
	}
	// Be nice, and no matter what happens, hide Quick Settings on exit.
	defer quicksettings.Hide(ctx, tconn)

	statusRectDP, err := quicksettings.Rect(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get quick settings rect")
	}
	statusLeftPX := int(math.Round(float64(statusRectDP.Left) * dispMode.DeviceScaleFactor))

	if err = waitForNewBoundsWithMargin(ctx, tconn, statusLeftPX-collisionWindowWorkAreaInsetsPX, right, dispMode.DeviceScaleFactor, pipPositionErrorMarginPX); err != nil {
		return errors.Wrap(err, "the PIP window must move to the left when Quick Settings gets shown")
	}

	// 2) The PIP window should move close the right border when Quick Settings is dismissed.

	testing.ContextLog(ctx, "Dismissing Quick Settings")
	if err := quicksettings.Hide(ctx, tconn); err != nil {
		return err
	}

	if err = waitForNewBoundsWithMargin(ctx, tconn, bounds.Left+bounds.Width, right, dispMode.DeviceScaleFactor, pipPositionErrorMarginPX); err != nil {
		return errors.Wrap(err, "the PIP window must go back to the original position when Quick Settings gets hidden")
	}

	return nil
}

// testPIPToggleTabletMode verifies that the window position is the same after toggling tablet mode.
func testPIPToggleTabletMode(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn, a *arc.ARC, act *arc.Activity, dev *ui.Device, dispMode *display.DisplayMode) error {
	window, err := getPIPWindow(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "could not get PIP window")
	}
	origBounds := coords.ConvertBoundsFromDPToPX(window.BoundsInRoot, dispMode.DeviceScaleFactor)
	destBounds := coords.NewRect(origBounds.Left, 0, origBounds.Width, origBounds.Height)

	// Move the PIP window upwards as much as possible to avoid possible interaction with shelf.
	if err := act.MoveWindow(ctx, tconn, time.Second, destBounds, origBounds); err != nil {
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

	origBounds = coords.ConvertBoundsFromDPToPX(window.BoundsInRoot, dispMode.DeviceScaleFactor)
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
func testPIPAutoPIPMinimize(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn, a *arc.ARC, pipAct *arc.Activity, dev *ui.Device, dispMode *display.DisplayMode) error {
	// TODO(edcourtney): Test minimize via shelf icon, keyboard shortcut (alt-minus), and caption.
	if err := minimizePIP(ctx, tconn, pipAct); err != nil {
		return errors.Wrap(err, "failed to set window state to minimized")
	}

	if err := waitForPIPWindow(ctx, tconn); err != nil {
		return errors.Wrap(err, "did not enter PIP")
	}

	return nil
}

func minimizePIP(ctx context.Context, tconn *chrome.TestConn, pipAct *arc.Activity) error {
	if err := ash.WaitForVisible(ctx, tconn, pipAct.PackageName()); err != nil {
		return errors.Wrap(err, "failed to wait for PIP activity to be visible")
	}
	window, err := ash.GetARCAppWindowInfo(ctx, tconn, pipAct.PackageName())
	if err != nil {
		return errors.Wrapf(err, "failed to get ARC window infomation for package name %s", pipAct.ActivityName())
	}
	// The window is minimized here, but the expected state is PIP, so the async API must used.
	if _, err := ash.SetWindowState(ctx, tconn, window.ID, ash.WMEventMinimize, false /* waitForStateChange */); err != nil {
		return errors.Wrapf(err, "failed to minimize %s", pipAct.ActivityName())
	}
	return waitForPIPWindow(ctx, tconn)
}

// testPIPExpandViaMenuTouch verifies that PIP window is properly expanded by touching menu.
func testPIPExpandViaMenuTouch(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn, a *arc.ARC, pipAct *arc.Activity, dev *ui.Device, dispMode *display.DisplayMode) error {
	isTabletModeEnabled, err := ash.TabletModeEnabled(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get tablet mode")
	}

	initialWindowState := arc.WindowStateNormal
	initialWMEvent := ash.WMEventNormal
	if isTabletModeEnabled {
		initialWindowState = arc.WindowStateMaximized
		initialWMEvent = ash.WMEventMaximize
	}

	initialAshWindowState, err := initialWindowState.ToAshWindowState()
	if err != nil {
		return errors.Wrap(err, "failed to get ash window state")
	}

	if _, err := ash.SetARCAppWindowState(ctx, tconn, pipAct.PackageName(), initialWMEvent); err != nil {
		return errors.Wrap(err, "failed to set initial window state")
	}
	if err := ash.WaitForARCAppWindowState(ctx, tconn, pipAct.PackageName(), initialAshWindowState); err != nil {
		return errors.Wrap(err, "did not enter initial window state")
	}

	// Enter PIP via minimize.
	if err := minimizePIP(ctx, tconn, pipAct); err != nil {
		return errors.Wrap(err, "failed to minimize app into PIP")
	}
	if err := waitForPIPWindow(ctx, tconn); err != nil {
		return errors.Wrap(err, "did not enter PIP mode")
	}

	if err := expandPIPViaMenuTouch(ctx, cr, tconn, pipAct, dev, dispMode, initialAshWindowState); err != nil {
		return errors.Wrap(err, "could not expand PIP")
	}

	return ash.WaitForARCAppWindowState(ctx, tconn, pipAct.PackageName(), initialAshWindowState)
}

// testPIPExpandViaShelfIcon verifies that PIP window is properly expanded by pressing shelf icon.
func testPIPExpandViaShelfIcon(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn, a *arc.ARC, pipAct *arc.Activity, dev *ui.Device, dispMode *display.DisplayMode) error {
	isTabletModeEnabled, err := ash.TabletModeEnabled(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get tablet mode")
	}

	initialWindowState := arc.WindowStateNormal
	initialWMEvent := ash.WMEventNormal
	if isTabletModeEnabled {
		initialWindowState = arc.WindowStateMaximized
		initialWMEvent = ash.WMEventMaximize
	}

	initialAshWindowState, err := initialWindowState.ToAshWindowState()
	if err != nil {
		return errors.Wrap(err, "failed to get ash window state")
	}

	if _, err := ash.SetARCAppWindowState(ctx, tconn, pipAct.PackageName(), initialWMEvent); err != nil {
		return errors.Wrap(err, "failed to set initial window state")
	}
	if err := ash.WaitForARCAppWindowState(ctx, tconn, pipAct.PackageName(), initialAshWindowState); err != nil {
		return errors.Wrap(err, "did not enter initial window state")
	}

	// Enter PIP via minimize.
	if err := minimizePIP(ctx, tconn, pipAct); err != nil {
		return errors.Wrap(err, "failed to minimize app into PIP")
	}
	if err := waitForPIPWindow(ctx, tconn); err != nil {
		return errors.Wrap(err, "did not enter PIP mode")
	}

	if err := pressShelfIcon(ctx, tconn); err != nil {
		return errors.Wrap(err, "could not expand PIP")
	}

	return ash.WaitForARCAppWindowState(ctx, tconn, pipAct.PackageName(), initialAshWindowState)
}

// testPIPAutoPIPNewAndroidWindow verifies that creating a new Android window that occludes an auto-PIP window will trigger PIP.
func testPIPAutoPIPNewAndroidWindow(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn, a *arc.ARC, pipAct *arc.Activity, dev *ui.Device, dispMode *display.DisplayMode) error {
	// Start the main activity that should enter PIP.
	if err := pipAct.Start(ctx, tconn); err != nil {
		return errors.Wrap(err, "could not start MainActivity")
	}
	defer pipAct.Stop(ctx, tconn)

	if err := ash.WaitForVisible(ctx, tconn, pipAct.PackageName()); err != nil {
		return errors.Wrap(err, "could not wait for PIP to be visible")
	}
	if err := dev.WaitForIdle(ctx, 10*time.Second); err != nil {
		return errors.Wrap(err, "could not wait for event thread to be idle")
	}
	if err := waitForPIPToBeGone(ctx, tconn); err != nil {
		return errors.Wrap(err, "could not wait for PIP to be gone")
	}

	maxAct, err := arc.NewActivity(a, wm.Pkg24, wm.NonResizableUnspecifiedActivity)
	if err != nil {
		return errors.Wrap(err, "could not create maximized activity")
	}
	defer maxAct.Close()

	// Start maximized activity again, this time with the guaranteed correct window state.
	if err := maxAct.Start(ctx, tconn); err != nil {
		return errors.Wrap(err, "could not start maximized Activity")
	}
	defer maxAct.Stop(ctx, tconn)

	if err := ash.WaitForARCAppWindowState(ctx, tconn, wm.Pkg24, ash.WindowStateMaximized); err != nil {
		return errors.Wrap(err, "did not maximize")
	}

	// Wait for MainActivity to enter PIP.
	// TODO(edcourtney): Until we can identify multiple Android windows from the same package, just wait for
	// the Android state here. Ideally, we should wait for the Chrome side state, but we don't need to do anything after
	// this on the Chrome side so it's okay for now. See crbug.com/1010671.
	return waitForPIPWindow(ctx, tconn)
}

// testPIPAutoPIPNewChromeWindow verifies that creating a new Chrome window that occludes an auto-PIP window will trigger PIP.
func testPIPAutoPIPNewChromeWindow(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn, a *arc.ARC, pipAct *arc.Activity, dev *ui.Device, dispMode *display.DisplayMode) error {
	// Open a maximized Chrome window and close at the end of the test.
	if err := tconn.Eval(ctx, `tast.promisify(chrome.windows.create)({state: "maximized"})`, nil); err != nil {
		return errors.Wrap(err, "could not open maximized Chrome window")
	}
	defer tconn.Call(ctx, nil, `async () => {
	  let window = await tast.promisify(chrome.windows.getLastFocused)({});
	  await tast.promisify(chrome.windows.remove)(window.id);
	}`)

	// Wait for MainActivity to enter PIP.
	// TODO(edcourtney): Until we can identify multiple Android windows from the same package, just wait for
	// the Android state here. Ideally, we should wait for the Chrome side state, but we don't need to do anything after
	// this on the Chrome side so it's okay for now. See crbug.com/1010671.
	return waitForPIPWindow(ctx, tconn)
}

// helper functions

// expandPIPViaMenuTouch expands PIP.
func expandPIPViaMenuTouch(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn, act *arc.Activity, dev *ui.Device, dispMode *display.DisplayMode, restoreWindowState ash.WindowStateType) error {
	sdkVer, err := arc.SDKVersion()
	if err != nil {
		return errors.Wrap(err, "failed to get the SDK version")
	}

	switch sdkVer {
	case arc.SDKP:
		return expandPIPViaMenuTouchP(ctx, tconn, act, dev, dispMode, restoreWindowState)
	case arc.SDKR:
		return expandPIPViaMenuTouchR(ctx, cr, tconn, dispMode, restoreWindowState)
	case arc.SDKT:
		return expandPIPViaMenuTouchT(ctx, cr, tconn, dispMode, restoreWindowState)
	default:
		return errors.Errorf("unsupported SDK version: %d", sdkVer)
	}
}

// expandPIPViaMenuTouchP injects touch events to the center of PIP window and expands PIP.
// The first touch event shows PIP menu and subsequent events expand PIP.
func expandPIPViaMenuTouchP(ctx context.Context, tconn *chrome.TestConn, act *arc.Activity, dev *ui.Device, dispMode *display.DisplayMode, restoreWindowState ash.WindowStateType) error {
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
	bounds := coords.ConvertBoundsFromDPToPX(window.BoundsInRoot, dispMode.DeviceScaleFactor)

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
		if windowState != restoreWindowState {
			return errors.New("the window isn't expanded yet")
		}

		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second, Interval: 500 * time.Millisecond})

}

// expandPIPViaMenuTouchR performs a mouse click to the center of PIP window and expands PIP.
// After moving the mouse to the center of the PIP window, it clicks on the center of the window several times until PIP gets expanded.
func expandPIPViaMenuTouchR(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn, dispMode *display.DisplayMode, restoreWindowState ash.WindowStateType) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		window, err := getPIPWindow(ctx, tconn)
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "could not get PIP window bounds"))
		}

		bounds := window.BoundsInRoot

		// Move the cursor away from the PIP window and then to the center of the PIP window slowly, otherwise
		// the PIP menu won't activate.
		if err := mouse.Move(tconn, coords.NewPoint(0, 0), time.Second)(ctx); err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to move the mouse to the top-left corner of the screen"))
		}
		if err := mouse.Move(tconn, coords.NewPoint(bounds.Left+bounds.Width/2, bounds.Top+bounds.Height/2), time.Second)(ctx); err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to move the mouse to center of the PIP window"))
		}
		// Try clicking the center of the window several times until the PIP window gets expanded.
		// The PIP menu has a bit of delay until it gets shown after the mouse hovers on the window.
		return testing.Poll(ctx, func(ctx context.Context) error {
			// Click on the expand button.
			if err := mouse.Press(tconn, mouse.LeftButton)(ctx); err != nil {
				return testing.PollBreak(errors.Wrap(err, "failed to press the left button"))
			}
			if err := mouse.Release(tconn, mouse.LeftButton)(ctx); err != nil {
				return testing.PollBreak(errors.Wrap(err, "failed to release the left button"))
			}
			// Check that it restored to the correct window state.
			if err := ash.WaitForARCAppWindowStateWithPollOptions(ctx, tconn, pipTestPkgName, restoreWindowState,
				&testing.PollOptions{Timeout: 3 * time.Second}); err != nil {
				return errors.Wrap(err, "did not expand to restore window state")
			}
			return nil
		}, &testing.PollOptions{Timeout: 10 * time.Second, Interval: 500 * time.Millisecond})
	}, &testing.PollOptions{Timeout: 20 * time.Second})
}

// expandPIPViaMenuTouchT performs a mouse click to the center of PIP window and expands PIP.
// After moving the mouse to the center of the PIP window it waits until the PIP menu is visible
// before the expand icon is clicked.
func expandPIPViaMenuTouchT(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn, dispMode *display.DisplayMode, restoreWindowState ash.WindowStateType) error {
	// Delegate to R version because there isn't a significant difference.
	return expandPIPViaMenuTouchR(ctx, cr, tconn, dispMode, restoreWindowState)
}

// waitForPIPWindow keeps looking for a PIP window until it appears on the Chrome side.
func waitForPIPWindow(ctx context.Context, tconn *chrome.TestConn) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		_, err := getPIPWindow(ctx, tconn)
		if err != nil {
			return errors.Wrap(err, "the PIP window hasn't been created yet")
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second})
}

// waitForPIPToBeGone keeps looking for a PIP window until it disappears on the Chrome side.
func waitForPIPToBeGone(ctx context.Context, tconn *chrome.TestConn) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		pip, err := getPIPWindow(ctx, tconn)
		if err == nil && pip != nil {
			return errors.Wrap(err, "the PIP window exists")
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second})
}

// getPIPWindow returns the PIP window if any.
func getPIPWindow(ctx context.Context, tconn *chrome.TestConn) (*ash.Window, error) {
	return ash.FindWindow(ctx, tconn, func(w *ash.Window) bool { return w.State == ash.WindowStatePIP })
}

// pressShelfIcon press the shelf icon of PIP window.
func pressShelfIcon(ctx context.Context, tconn *chrome.TestConn) error {
	// Make sure that at least one shelf icon exists.
	// Depending the test order, the status area might not be ready at this point.
	ui := uiauto.New(tconn)
	finder := nodewith.Name("ArcPipTest").ClassName("ash/ShelfAppButton").First()

	if err := ui.WaitUntilExists(finder)(ctx); err != nil {
		return errors.Wrap(err, "failed to locate shelf icons")
	}

	return ui.LeftClick(finder)(ctx)
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
		if int(math.Abs(float64(expectedValue-currentValue))) > margin {
			return errors.Errorf("the PIP window doesn't have the expected bounds yet; got %d, want %d", currentValue, expectedValue)
		}

		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second})
}
