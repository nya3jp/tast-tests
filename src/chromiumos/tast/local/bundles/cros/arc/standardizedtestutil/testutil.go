// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package standardizedtestutil provides helper functions to assist with running standardized arc tests
// for against android applications.
package standardizedtestutil

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

// Variables used by other tast tests
const (
	defaultTestCaseTimeout = 2 * time.Minute
	ShortUITimeout         = 30 * time.Second
)

// MouseButton abstracts the underlying mouse button implementation into a
// standard type that can be used by callers.
type MouseButton int

// Mouse buttons that can be used by tests.
const (
	LeftMouseButton MouseButton = iota
	RightMouseButton
)

// TestFuncParams contains parameters that can be used by the tests.
type TestFuncParams struct {
	TestConn        *chrome.TestConn
	Arc             *arc.ARC
	Device          *ui.Device
	AppPkgName      string
	AppActivityName string
	Activity        *arc.Activity
}

// TestFunc represents the test function.
type TestFunc func(ctx context.Context, s *testing.State, testParameters TestFuncParams)

// TestCase holds information about a test to run.
type TestCase struct {
	Name            string
	Fn              TestFunc
	Timeout         time.Duration
	WindowStateType ash.WindowStateType
}

// TouchscreenZoomType represents the touchscreen zoom type to perform.
type TouchscreenZoomType int

// Holds all of the zoom types that can be performed on the touchscreen.
const (
	TouchscreenZoomIn TouchscreenZoomType = iota
	TouchscreenZoomOut
)

// TouchscreenTapType represents the touch screen tap type to perform.
type TouchscreenTapType int

// Holds all the tap types that can be performed on the touchscreen.
const (
	ShortTouchscreenTap TouchscreenTapType = iota
	LongTouchscreenTap
)

// TouchscreenSwipeDirection represents the touchscreen swipe direction.
type TouchscreenSwipeDirection int

// Holds all the swipe directions that can be performed on the touchscreen.
const (
	DownTouchscreenSwipe TouchscreenSwipeDirection = iota
	UpTouchscreenSwipe
	LeftTouchscreenSwipe
	RightTouchscreenSwipe
)

// TouchscreenScrollDirection represents the scroll direction.
type TouchscreenScrollDirection int

// Variables used to determine the scroll direction.
const (
	DownTouchscreenScroll TouchscreenScrollDirection = iota
	UpTouchscreenScroll
)

// GetClamshellTests returns the test cases required for clamshell devices.
func GetClamshellTests(fn TestFunc) []TestCase {
	return []TestCase{
		{Name: "Full Screen", Fn: fn, WindowStateType: ash.WindowStateFullscreen},
		{Name: "Normal", Fn: fn, WindowStateType: ash.WindowStateNormal},
		{Name: "Snapped left", Fn: fn, WindowStateType: ash.WindowStateLeftSnapped},
		{Name: "Snapped right", Fn: fn, WindowStateType: ash.WindowStateRightSnapped},
	}
}

// GetClamshellHardwareDeps returns the hardware dependencies all clamshell tests share.
func GetClamshellHardwareDeps() hwdep.Deps {
	return hwdep.D(hwdep.SkipOnModel(TabletOnlyModels...))
}

// GetTabletTests returns the test cases required for tablet devices.
func GetTabletTests(fn TestFunc) []TestCase {
	return []TestCase{
		{Name: "Full Screen", Fn: fn, WindowStateType: ash.WindowStateFullscreen},
		{Name: "Maximized", Fn: fn, WindowStateType: ash.WindowStateMaximized},
	}
}

// GetTabletHardwareDeps returns the hardware dependencies all tablet tests share.
func GetTabletHardwareDeps() hwdep.Deps {
	return hwdep.D(hwdep.SkipOnModel(ClamshellOnlyModels...))
}

// RunTestCases runs the provided test cases and handles cleanup between tests.
func RunTestCases(ctx context.Context, s *testing.State, apkName, appPkgName, appActivity string, testCases []TestCase) {
	cr := s.FixtValue().(*arc.PreData).Chrome
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Could not open Test API connection: ", err)
	}

	a := s.FixtValue().(*arc.PreData).ARC
	d := s.FixtValue().(*arc.PreData).UIDevice
	if err := a.Install(ctx, arc.APKPath(apkName)); err != nil {
		s.Fatal("Failed to install the APK: ", err)
	}

	act, err := arc.NewActivity(a, appPkgName, appActivity)
	if err != nil {
		s.Fatal("Failed to create new app activity: ", err)
	}
	defer act.Close()

	// Run the different test cases.
	for idx, test := range testCases {
		// If a timeout is not specified, limited individual test cases to the default.
		// This makes sure that one test case doesn't use all of the time when it fails.
		timeout := defaultTestCaseTimeout
		if test.Timeout != 0 {
			timeout = test.Timeout
		}
		testCaseCtx, cancel := ctxutil.Shorten(ctx, timeout)
		defer cancel()

		s.Run(testCaseCtx, test.Name, func(cleanupCtx context.Context, s *testing.State) {
			// Save time for cleanup
			ctx, cancel := ctxutil.Shorten(cleanupCtx, 20*time.Second)
			defer cancel()

			// Launch the app.
			if err := act.Start(ctx, tconn); err != nil {
				s.Fatal("Failed to start app: ", err)
			}

			// Close the app between iterations.
			defer func(ctx context.Context) {
				if err := act.Stop(ctx, tconn); err != nil {
					s.Fatal("Failed to stop app: ", err)
				}
			}(cleanupCtx)

			// Take screenshot and dump ui info on failure.
			defer func(ctx context.Context) {
				if s.HasError() {
					filename := fmt.Sprintf("screenshot-standardized-arcappcompat-failed-test-%d.png", idx)
					path := filepath.Join(s.OutDir(), filename)
					if err := screenshot.CaptureChrome(ctx, cr, path); err != nil {
						testing.ContextLog(ctx, "Failed to capture screenshot, info: ", err)
					} else {
						testing.ContextLogf(ctx, "Saved screenshot to %s", filename)
					}
				}
			}(cleanupCtx)

			if _, err := ash.SetARCAppWindowStateAndWait(ctx, tconn, appPkgName, test.WindowStateType); err != nil {
				s.Fatal("Failed to set window state: ", err)
			}

			test.Fn(ctx, s, TestFuncParams{
				TestConn:        tconn,
				Arc:             a,
				Device:          d,
				AppPkgName:      appPkgName,
				AppActivityName: appActivity,
				Activity:        act,
			})
		})
		cancel()
	}
}

// TouchscreenTap performs a tap on the touchscreen.
func TouchscreenTap(ctx context.Context, testParameters TestFuncParams, selector *ui.Object, tapType TouchscreenTapType) error {
	touchScreen, err := input.Touchscreen(ctx)
	if err != nil {
		return errors.Wrap(err, "Unable to initialize touchscreen")
	}
	defer touchScreen.Close()

	touchScreenSingleEventWriter, err := touchScreen.NewSingleTouchWriter()
	if err != nil {
		return errors.Wrap(err, "Unable to initialize touchscreen single event writer")
	}
	defer touchScreenSingleEventWriter.Close()

	x, y, err := getTouchEventCoordinatesForElement(ctx, testParameters, touchScreen, selector)
	if err != nil {
		return errors.Wrap(err, "Unable to get touch screen coords")
	}

	switch tapType {
	case LongTouchscreenTap:
		if err := touchScreenSingleEventWriter.LongPressAt(ctx, *x, *y); err != nil {
			return errors.Wrap(err, "Unable to perform a long tap")
		}

		break
	case ShortTouchscreenTap:
		// Move to the given point and end the write to simulate a click.
		if err := touchScreenSingleEventWriter.Move(*x, *y); err != nil {
			return errors.Wrap(err, "Unable to move into position")
		}

		if err := touchScreenSingleEventWriter.End(); err != nil {
			return errors.Wrap(err, "Unable to end tap")
		}

		break
	default:
		return errors.Errorf("invalid tap type: %v", tapType)
	}

	return nil
}

// TouchscreenScroll performs a scroll on the touchscreen. Due to
// different device settings, the actual scroll amount in pixels will be imprecise.
// Therefore, multiple iterations should be run, with a check for the desired output between each call.
func TouchscreenScroll(ctx context.Context, touchScreen *input.TouchscreenEventWriter, testParameters TestFuncParams, selector *ui.Object, scrollDirection TouchscreenScrollDirection) error {
	const (
		VerticalScrollAmount = 250
		ScrollDuration       = 500 * time.Millisecond
	)

	touchScreenSingleEventWriter, err := touchScreen.NewSingleTouchWriter()
	if err != nil {
		return errors.Wrap(err, "Unable to initialize touchscreen single event writer")
	}
	defer touchScreenSingleEventWriter.Close()

	// Start the scroll on the provided element.
	x, y, err := getTouchEventCoordinatesForElement(ctx, testParameters, touchScreen, selector)
	if err != nil {
		return errors.Wrap(err, "Unable to get touch screen coords")
	}

	// Calculate where to scroll to based on the provided direction.
	scrollToX := *x
	scrollToY := *y
	switch scrollDirection {
	case DownTouchscreenScroll:
		scrollToY = *y - VerticalScrollAmount
		break
	case UpTouchscreenScroll:
		scrollToY = *y + VerticalScrollAmount
		break
	default:
		return errors.Errorf("invalid scroll direction: %v", scrollDirection)
	}

	// Perform the scroll movement.
	if err := touchScreenSingleEventWriter.Swipe(ctx, *x, *y, scrollToX, scrollToY, ScrollDuration); err != nil {
		return errors.Wrap(err, "unable to perform the scroll")
	}

	if err := touchScreenSingleEventWriter.End(); err != nil {
		return errors.Wrap(err, "unable to end the scroll")
	}

	return nil
}

// TouchscreenZoom performs a zoom on the touchscreen. Zoom in distance
// varies per device but the function aims to zoom by 2x (i.e. a scale factor of 2.0
// when zooming in, or .5 when zooming out).
func TouchscreenZoom(ctx context.Context, touchScreen *input.TouchscreenEventWriter, testParameters TestFuncParams, selector *ui.Object, zoomType TouchscreenZoomType) error {
	const (
		zoomDistancePerFinger = 900
		zoomDuration          = 250 * time.Millisecond
	)

	// Zoom is implemented as a two finger pinch so it requires two touches.
	mtw, err := touchScreen.NewMultiTouchWriter(2)
	if err != nil {
		return errors.Wrap(err, "unable to initialize event writer")
	}
	defer mtw.Close()

	// Get the coordinates of the element to perform the zoom gesture on.
	x, y, err := getTouchEventCoordinatesForElement(ctx, testParameters, touchScreen, selector)
	if err != nil {
		return errors.Wrap(err, "unable to get start coordinates")
	}

	// Perform the appropriate zoom.
	switch zoomType {
	case TouchscreenZoomIn:
		if err := mtw.ZoomIn(ctx, *x, *y, zoomDistancePerFinger, zoomDuration); err != nil {
			return errors.Wrap(err, "unable to zoom in")
		}

		break
	case TouchscreenZoomOut:
		if err := mtw.ZoomOut(ctx, *x, *y, zoomDistancePerFinger, zoomDuration); err != nil {
			return errors.Wrap(err, "unable to zoom in")
		}

		break
	default:
		return errors.Errorf("invalid zoom type provided: %v", zoomType)
	}

	// End the gesture.
	if err := mtw.End(); err != nil {
		return errors.Wrap(err, "unable to end the zoom in")
	}

	return nil
}

// TouchscreenSwipe performs a swipe in a given direction, starting from the provided selector.
// Due to different device settings, the actual swipe distance will be imprecise but aims to be near 50 pixels.
func TouchscreenSwipe(ctx context.Context, testParameters TestFuncParams, selector *ui.Object, numTouches int, swipeDirection TouchscreenSwipeDirection) error {
	const (
		distanceBetweenTouches = input.TouchCoord(40)
		swipeDuration          = 500 * time.Millisecond
		swipeDistance          = input.TouchCoord(250)
	)

	touchScreen, err := input.Touchscreen(ctx)
	if err != nil {
		return errors.Wrap(err, "unable to initialize touchscreen")
	}
	defer touchScreen.Close()

	tsw, err := touchScreen.NewMultiTouchWriter(numTouches)
	if err != nil {
		return errors.Wrap(err, "unable to initialize touchscreen event writer")
	}
	defer tsw.Close()

	x, y, err := getTouchEventCoordinatesForElement(ctx, testParameters, touchScreen, selector)
	if err != nil {
		return errors.Wrap(err, "unable to get touch screen coords")
	}

	// Get the destination coordinates based on the direction.
	endX := input.TouchCoord(0)
	endY := input.TouchCoord(0)

	switch swipeDirection {
	case UpTouchscreenSwipe:
		endX = *x
		endY = *y - swipeDistance
		break
	case DownTouchscreenSwipe:
		endX = *x
		endY = *y + swipeDistance
		break
	case LeftTouchscreenSwipe:
		endX = *x - swipeDistance
		endY = *y
		break
	case RightTouchscreenSwipe:
		endX = *x + swipeDistance
		endY = *y
		break
	default:
		return errors.Errorf("invalid direction provided: %v", swipeDirection)
	}

	// Perform the swipe.
	return tsw.Swipe(ctx, *x, *y, endX, endY, distanceBetweenTouches, numTouches, swipeDuration)
}

// getTouchEventCoordinatesForElement converts the points of an element to the corresponding Touchscreen coordinates.
func getTouchEventCoordinatesForElement(ctx context.Context, testParameters TestFuncParams, touchScreen *input.TouchscreenEventWriter, selector *ui.Object) (*input.TouchCoord, *input.TouchCoord, error) {
	// Get the center of the element to make sure the element is actually clicked.
	uiElementBounds, err := selector.GetBounds(ctx)
	if err != nil {
		return nil, nil, err
	}

	uiElementBoundsCenter := uiElementBounds.CenterPoint()

	// Get the size of the display, according to the activity as that's what 'GetBounds' is referring to.
	actSize, err := testParameters.Activity.DisplaySize(ctx)
	if err != nil {
		return nil, nil, err
	}

	tcc := touchScreen.NewTouchCoordConverter(actSize)
	xCord, yCord := tcc.ConvertLocation(uiElementBoundsCenter)
	return &xCord, &yCord, nil
}

// ClickInputAndGuaranteeFocus makes sure an input exists, clicks it, and ensures it is focused.
func ClickInputAndGuaranteeFocus(ctx context.Context, selector *ui.Object) error {
	if err := selector.Exists(ctx); err != nil {
		return errors.Wrap(err, "unable to find the input")
	}

	if err := selector.Click(ctx); err != nil {
		return errors.Wrap(err, "unable to click the input")
	}

	isFocused, err := selector.IsFocused(ctx)
	if err != nil {
		return errors.Wrap(err, "unable to check the inputs focus state")
	}

	if isFocused == false {
		return errors.Wrap(err, "unable to focus the input")
	}

	return nil
}

// MouseClickObject implements a standard way to click the mouse button on an object.
func MouseClickObject(ctx context.Context, testParameters TestFuncParams, selector *ui.Object, mew *input.MouseEventWriter, mouseButton MouseButton) error {
	// The device cannot be in tablet mode.
	tabletModeEnabled, err := ash.TabletModeEnabled(ctx, testParameters.TestConn)
	if err != nil {
		return errors.Wrap(err, "unable to determine tablet mode")
	}

	if tabletModeEnabled {
		return errors.New("Device is in tablet mode, cannot click with a mouse")
	}

	// Move the mouse into position
	if err := centerMouseOnObject(ctx, testParameters, mew, selector); err != nil {
		return errors.Wrap(err, "failed to move the mouse into position")
	}

	// Perform the correct click
	switch mouseButton {
	case LeftMouseButton:
		if err := mew.Click(); err != nil {
			return errors.Wrap(err, "unable to perform left mouse click")
		}

		break
	case RightMouseButton:
		if err := mew.RightClick(); err != nil {
			return errors.Wrap(err, "unable to perform right mouse click")
		}

		break
	default:
		return errors.Errorf("invalid button provided: %v", mouseButton)
	}

	return nil
}

// centerMouseOnObject is responsible for moving the mouse onto the center of the object.
func centerMouseOnObject(ctx context.Context, testParameters TestFuncParams, mew *input.MouseEventWriter, selector *ui.Object) error {
	// Get the center of the element to make sure the element is actually clicked.
	uiElementBounds, err := selector.GetBounds(ctx)
	if err != nil {
		return errors.Wrap(err, "unable to get the element bounds")
	}

	uiElementBoundsCenter := uiElementBounds.CenterPoint()

	// The coordinates returned by the selector are scaled up by the physical density
	// of the screen the activity is on. In order to determine the correct mouse coordinates,
	// that adjustment must be removed.
	physicalDensity, err := testParameters.Activity.DisplayDensity(ctx)
	if err != nil {
		return errors.Wrap(err, "unable to determine physical density of the activity")
	}

	moveToCoordinates := coords.NewPoint(int(float64(uiElementBoundsCenter.X)/physicalDensity), int(float64(uiElementBoundsCenter.Y)/physicalDensity))

	// Move and return the results
	return moveMouseToAbsoluteCoordinates(ctx, mew, moveToCoordinates)
}

// moveMouseToAbsoluteCoordinates moves the mouse to a set of absolute coordinates.
func moveMouseToAbsoluteCoordinates(ctx context.Context, mew *input.MouseEventWriter, absoluteCoordinates coords.Point) error {
	const (
		MouseRelMovePerIteration     = 1
		MouseResetPositionIterations = 10
		MouseResetPositionRelX       = -1000
		MouseResetPositionRelY       = -1000
		MouseTimeBetweenMoveCommands = 5 * time.Millisecond
	)

	// It's not obvious where the mouse is when the test starts because mice rely on
	// relative movements. The rest of this method assumes the mouse is starting at 0,0
	// so perform a few iterations of moving the mouse up to the top left corner of the screen.
	// TODO(davidwelling): adding a reset to the event writer may be beneficial as -1000,-1000 is being used in multiple places to reset the mouse.
	for i := 0; i < MouseResetPositionIterations; i++ {
		if err := mew.Move(MouseResetPositionRelX, MouseResetPositionRelY); err != nil {
			return errors.Wrap(err, "unable to reset the mouse position")
		}

		// Add a small sleep between move commands so the OS can sync.
		if err := testing.Sleep(ctx, MouseTimeBetweenMoveCommands); err != nil {
			return errors.Wrap(err, "unable to delay after mouse movement")
		}
	}

	// Move the mouse into position by performing a series of relative movements
	// along the necessary axis. Small iterations are preferred to make sure the mouse
	// doesn't move further than the specified amount (as noted in the mouse.Move method).
	curX := 0
	curY := 0

	for curX < absoluteCoordinates.X || curY < absoluteCoordinates.Y {
		dx := 0
		if curX < absoluteCoordinates.X {
			dx = MouseRelMovePerIteration
		}

		dy := 0
		if curY < absoluteCoordinates.Y {
			dy = MouseRelMovePerIteration
		}

		if err := mew.Move(int32(dx), int32(dy)); err != nil {
			return errors.Wrap(err, "unable to move the mouse into position")
		}

		// Add a small sleep between move commands so the OS can sync.
		if err := testing.Sleep(ctx, MouseTimeBetweenMoveCommands); err != nil {
			return errors.Wrap(err, "unable to delay after mouse movement")
		}

		curX += dx
		curY += dy
	}

	return nil
}

// TabletOnlyModels is a list of tablet only models to be skipped from clamshell mode runs.
var TabletOnlyModels = []string{
	"dru",
	"krane",
}

// ClamshellOnlyModels is a list of clamshell only models to be skipped from tablet mode runs.
var ClamshellOnlyModels = []string{
	"sarien",
	"elemi",
	"berknip",
	"dratini",

	// grunt:
	"careena",
	"kasumi",
	"treeya",
	"grunt",
	"barla",
	"aleena",
	"liara",
	"nuwani",

	// octopus:
	"bluebird",
	"apel",
	"blooglet",
	"blorb",
	"bobba",
	"casta",
	"dorp",
	"droid",
	"fleex",
	"foob",
	"garfour",
	"garg",
	"laser14",
	"lick",
	"mimrock",
	"nospike",
	"orbatrix",
	"phaser",
	"sparky",
}
