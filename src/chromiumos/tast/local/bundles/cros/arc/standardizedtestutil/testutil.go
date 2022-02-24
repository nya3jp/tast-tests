// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package standardizedtestutil provides helper functions to assist with running standardized arc tests
// for against android applications.
package standardizedtestutil

import (
	"context"
	"fmt"
	"math"
	"path/filepath"
	"time"

	"chromiumos/tast/common/android/adb"
	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arc/wm"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

// ShortUITimeout stores the time a UI action has before a timeout should occur.
const ShortUITimeout = 30 * time.Second

// RunTestCasesCleanupTime stores the amount of time a test has to clean up between runs.
const RunTestCasesCleanupTime = 20 * time.Second

// standardizedTestLayoutID stores the id of the layout of the standardized test. All tests
// are required to set the id of their layout to this.
const standardizedTestLayoutID = "layoutStandardizedTest"

// PointerButton abstracts the underlying pointer button implementation into a
// standard type that can be used by callers.
type PointerButton int

// Pointer buttons that can be used by tests.
const (
	LeftPointerButton PointerButton = iota
	RightPointerButton
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
type TestFunc func(ctx context.Context, testParameters TestFuncParams) error

// WindowState holds information about a window state that should be run as part of a test.
type WindowState struct {
	Name            string
	WindowStateType ash.WindowStateType
}

// Test holds information about a test that should run in a given mode, over multiple window states.
type Test struct {
	Fn           TestFunc
	InTabletMode bool
	WindowStates []WindowState
}

// ZoomType represents the zoom type to perform.
type ZoomType int

// Holds all of the zoom types that can be performed.
const (
	ZoomIn ZoomType = iota
	ZoomOut
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

// ScrollDirection represents the scroll direction.
type ScrollDirection int

// Variables used to determine the scroll direction.
const (
	DownScroll ScrollDirection = iota
	UpScroll
)

// StandardizedTouchscreen is a wrapper around the touchscreen that
// hides configuration which allows the touchscreen to work on all DUTs.
type StandardizedTouchscreen struct {
	ts *input.TouchscreenEventWriter
}

// Close closes the touchscreen device.
func (ts *StandardizedTouchscreen) Close() error {
	if ts.ts == nil {
		return errors.New("invalid touchscreen instance")
	}

	return ts.ts.Close()
}

// GetClamshellTest returns the test cases required for clamshell devices.
func GetClamshellTest(fn TestFunc) Test {
	return Test{
		Fn: fn,
		WindowStates: []WindowState{
			{Name: "Full Screen", WindowStateType: ash.WindowStateFullscreen},
			{Name: "Normal", WindowStateType: ash.WindowStateNormal},
			{Name: "Snapped left", WindowStateType: ash.WindowStateLeftSnapped},
			{Name: "Snapped right", WindowStateType: ash.WindowStateRightSnapped},
		},
		InTabletMode: false,
	}
}

// ClamshellHardwareDep returns the hardware dependencies all clamshell tests share.
var ClamshellHardwareDep = hwdep.SkipOnModel(TabletOnlyModels...)

// GetTabletTest returns the test cases required for tablet devices.
func GetTabletTest(fn TestFunc) Test {
	return Test{
		Fn: fn,
		WindowStates: []WindowState{
			{Name: "Full Screen", WindowStateType: ash.WindowStateFullscreen},
			{Name: "Maximized", WindowStateType: ash.WindowStateMaximized},
		},
		InTabletMode: true,
	}
}

// TabletHardwareDep returns the hardware dependencies all tablet tests share.
var TabletHardwareDep = hwdep.SkipOnModel(ClamshellOnlyModels...)

// RunTest runs the provided test cases and handles cleanup between tests.
func RunTest(ctx context.Context, s *testing.State, apkName, appPkgName, appActivity string, t Test) {
	runTest(ctx, s, apkName, appPkgName, appActivity, false /* fromPlayStore */, t)
}

// RunResizeLockTest runs the provided test cases with ResizeLock enabled, and handles cleanup between tests.
func RunResizeLockTest(ctx context.Context, s *testing.State, apkName, appPkgName, appActivity string, t Test) {
	runTest(ctx, s, apkName, appPkgName, appActivity, true /* fromPlayStore */, t)
}

func runTest(ctx context.Context, s *testing.State, apkName, appPkgName, appActivity string, fromPlayStore bool, t Test) {
	cr := s.FixtValue().(*arc.PreData).Chrome
	a := s.FixtValue().(*arc.PreData).ARC
	d := s.FixtValue().(*arc.PreData).UIDevice

	if fromPlayStore {
		if err := a.Install(ctx, arc.APKPath(apkName), adb.InstallOptionFromPlayStore); err != nil {
			s.Fatal("Failed to install the APK with Play Store install option: ", err)
		}
	} else {
		if err := a.Install(ctx, arc.APKPath(apkName)); err != nil {
			s.Fatal("Failed to install the APK: ", err)
		}
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Could not open Test API connection: ", err)
	}

	cleanupTabletMode, err := ash.EnsureTabletModeEnabled(ctx, tconn, t.InTabletMode)
	if err != nil {
		s.Fatalf("Failed to set tablet mode to %v: %v", t.InTabletMode, err)
	}
	defer cleanupTabletMode(ctx)

	// Run the different test cases.
	for idx, windowState := range t.WindowStates {
		s.Run(ctx, windowState.Name, func(cleanupCtx context.Context, s *testing.State) {
			// Save time for cleanup by working on a shortened context.
			workCtx, workCtxCancel := ctxutil.Shorten(cleanupCtx, RunTestCasesCleanupTime)
			defer workCtxCancel()

			// Launch the activity.
			act, err := arc.NewActivity(a, appPkgName, appActivity)
			if err != nil {
				s.Fatal("Failed to create new app activity: ", err)
			}

			defer func(ctx context.Context) {
				act.Close()
			}(cleanupCtx)

			if err := act.StartWithDefaultOptions(workCtx, tconn); err != nil {
				s.Fatal("Failed to start app: ", err)
			}
			defer act.Stop(cleanupCtx, tconn)

			// When an app is installed from Play Store and not allowlisted, ResizeLock will be enabled.
			// Close the ResizeLock splash screen.
			if fromPlayStore {
				if err := wm.CheckVisibility(ctx, tconn, wm.BubbleDialogClassName, true); err != nil {
					s.Fatal("Failed to wait for splash: ", err)
				}

				if err := wm.CloseSplash(ctx, tconn, wm.InputMethodClick, nil); err != nil {
					s.Fatal("Failed to close splash: ", err)
				}
			}

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

			// Wait for the activity to be ready.
			if err := ash.WaitForVisible(workCtx, tconn, act.PackageName()); err != nil {
				s.Fatal("Failed to wait for the app to be visible: ", err)
			}

			if err := d.WaitForIdle(workCtx, ShortUITimeout); err != nil {
				s.Fatal("Failed to wait for the app to be idle: ", err)
			}

			// All standardized tests have a layout with the same id. Wait for it to exist
			// to ensure the application is ready to be tested.
			if err := d.Object(ui.ID(StandardizedTestLayoutID(appPkgName))).WaitForExists(ctx, ShortUITimeout); err != nil {
				s.Fatal("Failed to wait for the app to render: ", err)
			}

			// Set the window state. Note that this can hang indefinitely as this is the
			// non-async version. The context is shortened for this call to make sure it
			// errors out early and additional tests can run.
			setWindowStateWorkCtx, setWindowStateCtxCancel := context.WithTimeout(workCtx, ShortUITimeout)
			defer setWindowStateCtxCancel()
			if _, err := ash.SetARCAppWindowStateAndWait(setWindowStateWorkCtx, tconn, appPkgName, windowState.WindowStateType); err != nil {
				s.Fatal("Failed to set window state: ", err)
			}

			// TODO(b/207691867): Pointer movements aren't consistent when in the 'Normal' window state unless the bounds are changed first.
			if windowState.WindowStateType == ash.WindowStateNormal {
				// Get the ARC window and its corresponding display info.
				w, err := ash.GetARCAppWindowInfo(workCtx, tconn, appPkgName)
				if err != nil {
					s.Fatal("Failed to get ARC window: ", err)
				}

				wInfo, err := display.FindInfo(ctx, tconn, func(info *display.Info) bool {
					return info.ID == w.DisplayID
				})

				if err != nil {
					s.Fatal("Failed to find the display: ", err)
				}

				// Adjust the window to fill up most of the screen.
				newBounds := wInfo.Bounds.WithInset(80, 80)
				if _, _, err := ash.SetWindowBounds(workCtx, tconn, w.ID, newBounds, w.DisplayID); err != nil {
					s.Fatal("Failed to set ARC window bounds: ", err)
				}
			}

			// The view may still be updating after the above window operations return so
			// poll on the layout one last time to make sure the app is in a steady state.
			if err := d.Object(ui.ID(StandardizedTestLayoutID(appPkgName))).WaitForExists(ctx, ShortUITimeout); err != nil {
				s.Fatal("Failed to wait for the app to render: ", err)
			}

			// Run the test.
			if err := t.Fn(workCtx, TestFuncParams{
				TestConn:        tconn,
				Arc:             a,
				Device:          d,
				AppPkgName:      appPkgName,
				AppActivityName: appActivity,
				Activity:        act,
			}); err != nil {
				s.Fatal("Test run failed: ", err)
			}
		})
	}
}

// StandardizedTestLayoutID returns the fully qualified path to the layout each
// standardized application is built with.
func StandardizedTestLayoutID(appPkgName string) string {
	return fmt.Sprintf("%s:id/%s", appPkgName, standardizedTestLayoutID)
}

// TouchscreenTap performs a tap on the touchscreen.
func TouchscreenTap(ctx context.Context, testParameters TestFuncParams, selector *ui.Object, tapType TouchscreenTapType) error {
	touchScreen, err := NewStandardizedTouchscreen(ctx, testParameters.TestConn)
	if err != nil {
		return errors.Wrap(err, "Unable to initialize touchscreen")
	}
	defer touchScreen.ts.Close()

	touchScreenSingleEventWriter, err := touchScreen.ts.NewSingleTouchWriter()
	if err != nil {
		return errors.Wrap(err, "Unable to initialize touchscreen single event writer")
	}
	defer touchScreenSingleEventWriter.Close()

	x, y, err := getTouchEventCoordinatesForElement(ctx, testParameters, touchScreen.ts, selector)
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
func TouchscreenScroll(ctx context.Context, touchScreen *StandardizedTouchscreen, testParameters TestFuncParams, selector *ui.Object, scrollDirection ScrollDirection) error {
	const (
		VerticalScrollAmount = 250
		ScrollDuration       = 500 * time.Millisecond
	)

	touchScreenSingleEventWriter, err := touchScreen.ts.NewSingleTouchWriter()
	if err != nil {
		return errors.Wrap(err, "Unable to initialize touchscreen single event writer")
	}
	defer touchScreenSingleEventWriter.Close()

	// Start the scroll on the provided element.
	x, y, err := getTouchEventCoordinatesForElement(ctx, testParameters, touchScreen.ts, selector)
	if err != nil {
		return errors.Wrap(err, "Unable to get touch screen coords")
	}

	// Calculate where to scroll to based on the provided direction.
	scrollToX := *x
	scrollToY := *y
	switch scrollDirection {
	case DownScroll:
		scrollToY = *y - VerticalScrollAmount
		break
	case UpScroll:
		scrollToY = *y + VerticalScrollAmount
		break
	default:
		return errors.Errorf("invalid scroll direction; got: %v", scrollDirection)
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
func TouchscreenZoom(ctx context.Context, testParameters TestFuncParams, selector *ui.Object, zoomType ZoomType) error {
	const (
		zoomDistanceAsProportionOfTouchscreen = .15
		zoomDuration                          = 250 * time.Millisecond
	)

	touchScreen, err := NewStandardizedTouchscreen(ctx, testParameters.TestConn)
	if err != nil {
		return errors.Wrap(err, "unable to initialize the touchscreen")
	}
	defer touchScreen.ts.Close()

	// Zoom is implemented as a two finger pinch so it requires two touches.
	mtw, err := touchScreen.ts.NewMultiTouchWriter(2)
	if err != nil {
		return errors.Wrap(err, "unable to initialize event writer")
	}
	defer mtw.Close()

	// Get the coordinates of the element to perform the zoom gesture on.
	x, y, err := getTouchEventCoordinatesForElement(ctx, testParameters, touchScreen.ts, selector)
	if err != nil {
		return errors.Wrap(err, "unable to get start coordinates")
	}

	// Start from the element and move fingers an amount relative to the size of the touchscreen
	// in order to trigger the pinch zoom. Attempt to keep it within the bounds of the smallest dimension.
	zoomDistancePerFinger := math.Min(float64(touchScreen.ts.Width()), float64(touchScreen.ts.Height())) * zoomDistanceAsProportionOfTouchscreen

	// Perform the appropriate zoom.
	switch zoomType {
	case ZoomIn:
		if err := mtw.Zoom(ctx, *x, *y, input.TouchCoord(zoomDistancePerFinger), zoomDuration, input.ZoomIn); err != nil {
			return errors.Wrap(err, "unable to zoom in")
		}
	case ZoomOut:
		if err := mtw.Zoom(ctx, *x, *y, input.TouchCoord(zoomDistancePerFinger), zoomDuration, input.ZoomOut); err != nil {
			return errors.Wrap(err, "unable to zoom in")
		}
	default:
		return errors.Errorf("invalid zoom type provided: %v", zoomType)
	}

	// End the gesture.
	if err := mtw.End(); err != nil {
		return errors.Wrap(err, "unable to end the zoom in")
	}

	return nil
}

// NewStandardizedTouchscreen returns a touchscreen that has been configured to
// work on all DUTs.
func NewStandardizedTouchscreen(ctx context.Context, tconn *chrome.TestConn) (*StandardizedTouchscreen, error) {
	touchScreen, err := input.Touchscreen(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "unable to initialize touchscreen")
	}

	// Adjust the orientation based on the display so that the coordinates
	// identified by element selectors are translated to the touchscreen
	// correctly.
	orientation, err := display.GetOrientation(ctx, tconn)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get display orientation")
	}

	if err := touchScreen.SetRotation(-orientation.Angle); err != nil {
		return nil, errors.Wrap(err, "failed to set rotation of touchscreen")
	}

	return &StandardizedTouchscreen{ts: touchScreen}, nil
}

// TouchscreenSwipe performs a swipe in a given direction, starting from the provided selector.
// Due to different device settings, the actual swipe distance will be imprecise but aims to be near 50 pixels.
func TouchscreenSwipe(ctx context.Context, testParameters TestFuncParams, selector *ui.Object, numTouches int, swipeDirection TouchscreenSwipeDirection) error {
	const (
		distanceBetweenTouches = input.TouchCoord(40)
		swipeDuration          = 500 * time.Millisecond
		swipeDistance          = input.TouchCoord(250)
	)

	touchScreen, err := NewStandardizedTouchscreen(ctx, testParameters.TestConn)
	if err != nil {
		return errors.Wrap(err, "unable to initialize touchscreen")
	}
	defer touchScreen.ts.Close()

	tsw, err := touchScreen.ts.NewMultiTouchWriter(numTouches)
	if err != nil {
		return errors.Wrap(err, "unable to initialize touchscreen event writer")
	}
	defer tsw.Close()

	x, y, err := getTouchEventCoordinatesForElement(ctx, testParameters, touchScreen.ts, selector)
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
	if err := selector.WaitForExists(ctx, ShortUITimeout); err != nil {
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
func MouseClickObject(ctx context.Context, testParameters TestFuncParams, selector *ui.Object, mew *input.MouseEventWriter, mouseButton PointerButton) error {
	if err := validatePointerCanBeUsed(ctx, testParameters); err != nil {
		return errors.Wrap(err, "mouse cannot be used")
	}

	// Move the mouse into position
	if err := centerPointerOnObject(ctx, testParameters, selector); err != nil {
		return errors.Wrap(err, "failed to move the mouse into position")
	}

	// Perform the correct click
	switch mouseButton {
	case LeftPointerButton:
		if err := mew.Click(); err != nil {
			return errors.Wrap(err, "unable to perform left mouse click")
		}

		break
	case RightPointerButton:
		if err := mew.RightClick(); err != nil {
			return errors.Wrap(err, "unable to perform right mouse click")
		}

		break
	default:
		return errors.Errorf("invalid button provided: %v", mouseButton)
	}

	return nil
}

// MouseMoveOntoObject moves the mouse onto the center of an object.
func MouseMoveOntoObject(ctx context.Context, testParameters TestFuncParams, selector *ui.Object, mew *input.MouseEventWriter) error {
	if err := validatePointerCanBeUsed(ctx, testParameters); err != nil {
		return errors.Wrap(err, "mouse cannot be used")
	}

	if err := centerPointerOnObject(ctx, testParameters, selector); err != nil {
		return errors.Wrap(err, "failed to move the mouse into position")
	}

	return nil
}

// MouseScroll performs a scroll on the mouse. Due to different device
// settings, the actual scroll amount in pixels will be imprecise. Therefore,
// multiple iterations should be run, with a check for the desired output
// between each call.
func MouseScroll(ctx context.Context, testParameters TestFuncParams, scrollDirection ScrollDirection, mew *input.MouseEventWriter) error {
	if err := validatePointerCanBeUsed(ctx, testParameters); err != nil {
		return errors.Wrap(err, "mouse cannot be used")
	}

	switch scrollDirection {
	case UpScroll:
		if err := mew.ScrollUp(); err != nil {
			return errors.Wrap(err, "unable to scroll up")
		}
		break
	case DownScroll:
		if err := mew.ScrollDown(); err != nil {
			return errors.Wrap(err, "unable to scroll down")
		}
		break
	default:
		return errors.Errorf("invalid scroll direction: %v", scrollDirection)
	}

	return nil
}

// Trackpad related constants. These values were derived experimentally and
// should work on both physical, and virtual trackpads.
const (
	TrackpadMajorSize                              = 240
	TrackpadMinorSize                              = 180
	TrackpadClickPressureAsProportionOfMaxPressure = .2
	TrackpadClickDefaultPressure                   = 50
	TrackpadGesturePressure                        = 10
	TrackpadVerticalScrollAmountAsProportionOfPad  = .2
	TrackpadScrollDuration                         = 250 * time.Millisecond
	TrackpadFingerSeparation                       = 350
)

// TrackpadClickObject implements a click on the trackpad.
func TrackpadClickObject(ctx context.Context, testParameters TestFuncParams, selector *ui.Object, tew *input.TrackpadEventWriter, pointerButton PointerButton) error {
	if err := validatePointerCanBeUsed(ctx, testParameters); err != nil {
		return errors.Wrap(err, "trackpad cannot be used")
	}

	if err := centerPointerOnObject(ctx, testParameters, selector); err != nil {
		return errors.Wrap(err, "failed to move the trackpad into position")
	}

	stw, err := tew.NewSingleTouchWriter()
	if err != nil {
		return errors.Wrap(err, "unable to setup the writer")
	}

	// Setup the trackpad to simulate a finger click on the next event.
	if err := stw.SetSize(ctx, TrackpadMajorSize, TrackpadMinorSize); err != nil {
		return errors.Wrap(err, "unable to set size")
	}

	// The pressure should be based on a proportion of the max pressure. If it's less than 0,
	// use the default value and report it.
	pressureToUse := int32(float64(tew.MaxPressure()) * TrackpadClickPressureAsProportionOfMaxPressure)
	if pressureToUse <= 0 {
		testing.ContextLog(ctx, "Pressure is less than 0, setting to: ", TrackpadClickDefaultPressure)
		pressureToUse = TrackpadClickDefaultPressure
	}

	if err := stw.SetPressure(pressureToUse); err != nil {
		return errors.Wrap(err, "unable to set pressure")
	}

	// Setup the action intent
	switch pointerButton {
	case LeftPointerButton:
		// A left click only requires a single touch.
		stw.SetIsBtnToolFinger(true)
		break
	case RightPointerButton:
		// A left click only requires a double tap.
		stw.SetIsBtnToolDoubleTap(true)
		break
	default:
		return errors.Errorf("invalid button provided: %v", pointerButton)
	}

	// Perform the 'tap' at the center of the touchpad. The pointer has already been positioned
	// so the move event is just signaling where on the trackpad the event takes place.
	centerX := tew.Width() / 2
	centerY := tew.Height() / 2
	if err := stw.Move(centerX, centerY); err != nil {
		return errors.Wrap(err, "unable to initiate click position")
	}

	if err := stw.End(); err != nil {
		return errors.Wrap(err, "unable to end click")
	}

	return nil
}

// TrackpadScroll performs a two-finger scroll gesture on the trackpad.
func TrackpadScroll(ctx context.Context, trackpad *input.TrackpadEventWriter, testParameters TestFuncParams, selector *ui.Object, scrollDirection ScrollDirection) error {
	if err := validatePointerCanBeUsed(ctx, testParameters); err != nil {
		return errors.Wrap(err, "pointer cannot cannot be used")
	}

	if err := centerPointerOnObject(ctx, testParameters, selector); err != nil {
		return errors.Wrap(err, "failed to move the trackpad into position")
	}

	mtw, err := trackpad.NewMultiTouchWriter(2)
	if err != nil {
		return errors.Wrap(err, "unable to initialize multi-touch writer")
	}
	defer mtw.Close()

	if err := initializeWriterForTwoFingerTrackpadGesture(ctx, mtw); err != nil {
		return errors.Wrap(err, "unable to setup writer for two finger gestures")
	}

	// The scroll can always begin at the middle of the trackpad.
	x := trackpad.Width() / 2
	y := trackpad.Height() / 2

	// The scroll should travel through a distance that is proportional to the
	// size of the trackpad. This ensures the event generates enough movement
	// to fire off the action, and that an out of bounds error does not occur.
	verticalScrollDistance := input.TouchCoord(float64(trackpad.Height()) * TrackpadVerticalScrollAmountAsProportionOfPad)

	scrollToX := x
	scrollToY := y
	switch scrollDirection {
	case DownScroll:
		scrollToY = y + verticalScrollDistance
	case UpScroll:
		scrollToY = y - verticalScrollDistance
	default:
		return errors.Errorf("invalid scroll direction: %v", scrollDirection)
	}

	// Move both fingers accordingly.
	if err := mtw.DoubleSwipe(ctx, x, y, scrollToX, scrollToY, input.TouchCoord(TrackpadFingerSeparation), TrackpadScrollDuration); err != nil {
		return errors.Wrap(err, "unable to perform the scroll")
	}

	if err := mtw.End(); err != nil {
		return errors.Wrap(err, "unable to end the scroll")
	}

	return nil
}

// TrackpadZoom performs a two-finger zoom gesture on the trackpad.
func TrackpadZoom(ctx context.Context, trackpad *input.TrackpadEventWriter, testParameters TestFuncParams, selector *ui.Object, zoomType ZoomType) error {
	const (
		zoomDuration = 200 * time.Millisecond
	)

	if err := validatePointerCanBeUsed(ctx, testParameters); err != nil {
		return errors.Wrap(err, "pointer cannot cannot be used")
	}

	if err := centerPointerOnObject(ctx, testParameters, selector); err != nil {
		return errors.Wrap(err, "failed to move the trackpad into position")
	}

	mtw, err := trackpad.NewMultiTouchWriter(2)
	if err != nil {
		return errors.Wrap(err, "unable to initialize multi-touch writer")
	}
	defer mtw.Close()

	if err := initializeWriterForTwoFingerTrackpadGesture(ctx, mtw); err != nil {
		return errors.Wrap(err, "unable to setup writer for two finger gestures")
	}

	// Perform the appropriate zoom.
	switch zoomType {
	case ZoomIn:
		if err := mtw.ZoomRelativeToSize(ctx, zoomDuration, input.ZoomIn); err != nil {
			return errors.Wrap(err, "unable to zoom in")
		}
	case ZoomOut:
		if err := mtw.ZoomRelativeToSize(ctx, zoomDuration, input.ZoomOut); err != nil {
			return errors.Wrap(err, "unable to zoom out")
		}
	default:
		return errors.Errorf("invalid zoom type provided: %v", zoomType)
	}

	// End the gesture.
	if err := mtw.End(); err != nil {
		return errors.Wrap(err, "unable to end the zoom")
	}

	return nil
}

// validatePointerCanBeUsed makes sure the pointer can be used in tests.
func validatePointerCanBeUsed(ctx context.Context, testParameters TestFuncParams) error {
	// The device cannot be in tablet mode.
	tabletModeEnabled, err := ash.TabletModeEnabled(ctx, testParameters.TestConn)
	if err != nil {
		return errors.Wrap(err, "unable to determine tablet mode")
	}

	if tabletModeEnabled {
		return errors.New("Device is in tablet mode, cannot click with a mouse")
	}

	return nil
}

// centerPointerOnObject is responsible for moving the pointer onto the center of the object.
func centerPointerOnObject(ctx context.Context, testParameters TestFuncParams, selector *ui.Object) error {
	// Get the center of the element to make sure the element is actually clicked.
	uiElementBounds, err := selector.GetBounds(ctx)
	if err != nil {
		return errors.Wrap(err, "unable to get the element bounds")
	}

	uiElementBoundsCenter := uiElementBounds.CenterPoint()

	// The coordinates returned by the selector need to be scaled back by the
	// device's scale factor in order to get the proper position.
	dispMode, err := ash.PrimaryDisplayMode(ctx, testParameters.TestConn)
	if err != nil {
		return errors.Wrap(err, "failed to get display mode")
	}
	dsf := dispMode.DeviceScaleFactor

	moveToCoordinates := coords.NewPoint(int(float64(uiElementBoundsCenter.X)/dsf), int(float64(uiElementBoundsCenter.Y)/dsf))

	// Move and return the results using the internal method. To ARC, this is the same
	// as moving a physical mouse.
	if err := mouse.Move(testParameters.TestConn, moveToCoordinates, 0)(ctx); err != nil {
		return errors.Wrap(err, "unable to move the mouse into position")
	}

	return nil
}

// initializeWriterForTwoFingerTrackpadGesture sets up an event writer
// to simulate two finger events on a trackpad.
func initializeWriterForTwoFingerTrackpadGesture(ctx context.Context, mtw *input.TouchEventWriter) error {
	// Setup the trackpad to simulate two fingers resting on the trackpad.
	if err := mtw.SetSize(ctx, TrackpadMajorSize, TrackpadMinorSize); err != nil {
		return errors.Wrap(err, "unable to set size")
	}

	if err := mtw.SetPressure(TrackpadGesturePressure); err != nil {
		return errors.Wrap(err, "unable to set pressure")
	}

	mtw.SetIsBtnToolDoubleTap(true)

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
