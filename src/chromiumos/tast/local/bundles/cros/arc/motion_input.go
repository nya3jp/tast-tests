// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"encoding/json"
	"math"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MotionInput,
		Desc:         "Checks motion input (touch/mouse) works in various window states on Android",
		Contacts:     []string{"prabirmsp@chromium.org", "arc-framework@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
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

type motionEventAction string
type motionEventAxis string
type inputDeviceSource string

const (
	// These constants are from Android's MotionEvent.java.
	// See: https://cs.android.com/android/platform/superproject/+/master:frameworks/base/core/java/android/view/MotionEvent.java
	actionDown motionEventAction = "ACTION_DOWN"
	actionUp   motionEventAction = "ACTION_UP"
	actionMove motionEventAction = "ACTION_MOVE"

	// These constants are from Android's MotionEvent.java.
	// See: https://cs.android.com/android/platform/superproject/+/master:frameworks/base/core/java/android/view/MotionEvent.java
	axisX        motionEventAxis = "AXIS_X"
	axisY        motionEventAxis = "AXIS_Y"
	axisPressure motionEventAxis = "AXIS_PRESSURE"

	// These constants must be kept in sync with ArcMotionInputTest.apk.
	srcTouchscreen inputDeviceSource = "touchscreen"
)

// motionEvent represents a MotionEvent that was received by the Android application.
// For all motionEventAxis values that represent an absolute location, the values are in the
// coordinate space of the Android window (i.e. 0,0 is the top left corner of the application
// window in Android).
type motionEvent struct {
	Action      motionEventAction             `json:"action"`
	DeviceID    int                           `json:"device_id"`
	Sources     []inputDeviceSource           `json:"sources"`
	PointerAxes []map[motionEventAxis]float64 `json:"pointer_axes"`
}

const (
	motionInputTestAPK = "ArcMotionInputTest.apk"
	motionInputTestPKG = "org.chromium.arc.testapp.motioninput"
	motionInputTestCLS = ".MainActivity"
)

// readMotionEvent unmarshalls the JSON string in the TextView representing a MotionEvent
// received by ArcMotionInputTest.apk, and returns it as a motionEvent.
func readMotionEvent(ctx context.Context, d *ui.Device) (*motionEvent, error) {
	view := d.Object(ui.ID(motionInputTestPKG + ":id/motion_event"))
	text, err := view.GetText(ctx)
	if err != nil {
		return nil, err
	}

	var event motionEvent
	if err := json.Unmarshal([]byte(text), &event); err != nil {
		return nil, err
	}
	return &event, nil
}

// motionEventMatcher represents a matcher for motionEvent.
type motionEventMatcher func(*motionEvent) error

// expectMotionEvent polls readMotionEvent repeatedly until it receives a motionEvent that
// successfully matches the provided motionEventMatcher, or until it times out.
func expectMotionEvent(ctx context.Context, d *ui.Device, match motionEventMatcher) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		event, err := readMotionEvent(ctx, d)
		if err != nil {
			return err
		}
		return match(event)
	}, nil)
}

const (
	// coordinateAxisEpsilon is the epsilon value to be used when comparing axis values that
	// represent absolute display coordinates. Scaling and conversions from Chrome to Android's
	// display spaces means absolute coordinates can be off by up to one pixel.
	coordinateAxisEpsilon = 1e0

	// defaultAxisEpsilon is the epsilon value to be used when comparing axis values that do
	// not need to be scaled or converted, like pressure (which is in the range [0,1]). We
	// expect these values to be more precise.
	defaultAxisEpsilon = 1e-5
)

// singlePointerMatcher returns a motionEventMatcher that matches a motionEvent with a single
// pointer that has the following axes: axisX, axisY, and axisPressure.
func singlePointerMatcher(a motionEventAction, s inputDeviceSource, p coords.Point, pressure float64) motionEventMatcher {
	return func(event *motionEvent) error {
		sourceMatches := false
		for _, v := range event.Sources {
			if v == s {
				sourceMatches = true
				break
			}
		}
		axisMatcher := func(axis motionEventAxis, expected, epsilon float64) error {
			v := event.PointerAxes[0][axis]
			if math.Abs(v-expected) > epsilon {
				return errors.Errorf("value of axis %s did not match: got %.5f; want %.5f; epsilon %.5f", axis, v, expected, epsilon)
			}
			return nil
		}
		if !sourceMatches {
			return errors.Errorf("source does not match: got %v; want %s", event.Sources, s)
		} else if event.Action != a {
			return errors.Errorf("action does not match: got %s; want: %s", event.Action, a)
		} else if pointerCount := len(event.PointerAxes); pointerCount != 1 {
			return errors.Errorf("pointer count does not match: got: %d; want: %d", pointerCount, 1)
		} else if err := axisMatcher(axisX, float64(p.X), coordinateAxisEpsilon); err != nil {
			return err
		} else if err := axisMatcher(axisY, float64(p.Y), coordinateAxisEpsilon); err != nil {
			return err
		} else if err := axisMatcher(axisPressure, pressure, defaultAxisEpsilon); err != nil {
			return err
		}
		return nil
	}
}

// singleTouchMatcher returns a motionEventMatcher that matches events from a Touchscreen device.
func singleTouchMatcher(a motionEventAction, p coords.Point) motionEventMatcher {
	return singlePointerMatcher(a, srcTouchscreen, p, 1)
}

// MotionInput runs several sub-tests, where each sub-test sets up the Chrome WM environment as
// specified by the motionInputSubtestParams. Each sub-test installs and runs an Android application
// (ArcMotionInputTest.apk), injects various input events into ChromeOS through uinput devices,
// and verifies that those events were received by the Android application in the expected screen
// locations.
func MotionInput(ctx context.Context, s *testing.State) {
	p := s.PreValue().(arc.PreData)
	cr := p.Chrome
	a := p.ARC

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}

	// runSubtest runs a single sub-test by setting up the WM environment as specified by the
	// provided motionInputSubtestParams.
	runSubtest := func(ctx context.Context, s *testing.State, params *motionInputSubtestParams) {

		test := motionInputSubtestState{}
		test.params = params

		deviceMode := "clamshell"
		if params.TabletMode {
			deviceMode = "tablet"
		}
		s.Logf("Setting device to %v mode", deviceMode)
		cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, params.TabletMode)
		if err != nil {
			s.Fatal("Failed to ensure tablet mode enabled: ", err)
		}
		defer cleanup(ctx)

		test.d, err = ui.NewDevice(ctx, a)
		if err != nil {
			s.Fatal("Failed initializing UI Automator: ", err)
		}
		defer test.d.Close()

		infos, err := display.GetInfo(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to get display info: ", err)
		}
		if len(infos) == 0 {
			s.Fatal("No display found")
		}
		for i := range infos {
			if infos[i].IsInternal {
				test.displayInfo = &infos[i]
				break
			}
		}
		if test.displayInfo == nil {
			s.Log("No internal display found. Default to the first display")
			test.displayInfo = &infos[0]
		}

		test.scale, err = test.displayInfo.GetEffectiveDeviceScaleFactor()
		if err != nil {
			s.Fatal("Failed to get effective device scale factor: ", err)
		}

		s.Log("Installing apk ", motionInputTestAPK)
		if err := a.Install(ctx, arc.APKPath(motionInputTestAPK)); err != nil {
			s.Fatalf("Failed installing %s: %v", motionInputTestAPK, err)
		}

		act, err := arc.NewActivity(a, motionInputTestPKG, motionInputTestCLS)
		if err != nil {
			s.Fatal("Failed to create an activity: ", err)
		}
		defer act.Close()

		if err := act.Start(ctx, tconn); err != nil {
			s.Fatal("Failed to start an activity: ", err)
		}
		defer act.Stop(ctx)

		if err := ash.WaitForVisible(ctx, tconn, motionInputTestPKG); err != nil {
			s.Fatal("Failed to wait for activity to be visible: ", err)
		}

		if params.WmEventToSend != nil {
			event := params.WmEventToSend.(ash.WMEventType)
			s.Log("Sending wm event: ", event)
			if _, err := ash.SetARCAppWindowState(ctx, tconn, motionInputTestPKG, event); err != nil {
				s.Fatalf("Failed to set ARC app window state with event %s: %v", event, err)
			}
		}

		if params.WmStateToVerify != nil {
			windowState := params.WmStateToVerify.(ash.WindowStateType)
			s.Log("Verifying window state: ", windowState)
			if err := ash.WaitForARCAppWindowState(ctx, tconn, motionInputTestPKG, windowState); err != nil {
				s.Fatal("Failed to verify app window state: ", windowState)
			}
		}

		if err := test.d.WaitForIdle(ctx, 10*time.Second); err != nil {
			s.Log("Failed to wait for idle, ignoring: ", err)
		}

		test.w, err = ash.GetARCAppWindowInfo(ctx, tconn, motionInputTestPKG)
		if err != nil {
			s.Fatal("Failed to get ARC app window info: ", err)
		}

		verifyTouchscreen(ctx, s, test)
	}

	for _, params := range []motionInputSubtestParams{
		{
			Name:            "Clamshell Normal",
			TabletMode:      false,
			WmEventToSend:   ash.WMEventNormal,
			WmStateToVerify: ash.WindowStateNormal,
		}, {
			Name:            "Clamshell Fullscreen",
			TabletMode:      false,
			WmEventToSend:   ash.WMEventFullscreen,
			WmStateToVerify: ash.WindowStateFullscreen,
		}, {
			Name:            "Clamshell Maximized",
			TabletMode:      false,
			WmEventToSend:   ash.WMEventMaximize,
			WmStateToVerify: ash.WindowStateMaximized,
		},
		// TODO(b/155500968): Investigate why the first motion event received by the test app has
		//  incorrect coordinates in tablet mode, and add test params for tablet mode when resolved.
	} {
		s.Run(ctx, params.Name, func(ctx context.Context, s *testing.State) {
			runSubtest(ctx, s, &params)
		})
	}
}

// wmEventToSend holds an ash.WMEventType or nil.
type wmEventToSend interface{}

// wmWindowState holds an ash.WindowStateType or nil.
type wmWindowState interface{}

// motionInputSubtestParams holds the test parameters used to set up the WM environment in Chrome, and
// represents a single sub-test.
type motionInputSubtestParams struct {
	Name            string        // A description of the subtest.
	TabletMode      bool          // If true, the device will be put in tablet mode.
	WmEventToSend   wmEventToSend // This must be of type ash.WMEventType, and can be nil.
	WmStateToVerify wmWindowState // This must be of type ash.WindowStateType, and can be nil.
}

// isCaptionVisible returns true if the caption should be visible for the Android application when
// the respective params are applied.
func (params *motionInputSubtestParams) isCaptionVisible() bool {
	return !params.TabletMode && params.WmEventToSend.(ash.WMEventType) != ash.WMEventFullscreen
}

// motionInputSubtestState holds various values that represent the test state for each sub-test.
// It is created for convenience to reduce the number of function parameters.
type motionInputSubtestState struct {
	displayInfo *display.Info
	scale       float64
	w           *ash.Window
	d           *ui.Device
	params      *motionInputSubtestParams
}

// expectedPoint takes a coords.Point representing the coordinate where an input event is injected
// in DP in the display space, and returns a coords.Point representing where it is expected to be
// injected in the Android application window's coordinate space.
func (test *motionInputSubtestState) expectedPoint(p coords.Point) coords.Point {
	insetLeft := test.w.BoundsInRoot.Left
	insetTop := test.w.BoundsInRoot.Top
	if test.params.isCaptionVisible() {
		insetTop += test.w.CaptionHeight
	}
	return coords.NewPoint(int(float64(p.X-insetLeft)*test.scale), int(float64(p.Y-insetTop)*test.scale))
}

// verifyTouchscreen tests the behavior of events injected from a uinput touchscreen device. It
// injects a down event, followed by several move events, and finally an up event with a single
// touch pointer.
func verifyTouchscreen(ctx context.Context, s *testing.State, test motionInputSubtestState) {
	s.Log("Creating Touchscreen input device")
	tew, err := input.Touchscreen(ctx)
	if err != nil {
		s.Fatal("Failed to create touchscreen: ", err)
	}
	defer tew.Close()

	stw, err := tew.NewSingleTouchWriter()
	if err != nil {
		s.Fatal("Failed to create SingleTouchEventWriter: ", err)
	}
	defer stw.Close()

	tcc := tew.NewTouchCoordConverter(test.displayInfo.Bounds.Size())

	pointDP := test.w.BoundsInRoot.CenterPoint()
	expected := test.expectedPoint(pointDP)

	s.Log("Verifying touch down event at ", expected)
	x, y := tcc.ConvertLocation(pointDP)
	if err := stw.Move(x, y); err != nil {
		s.Fatalf("Could not inject move at (%d, %d)", x, y)
	}
	if err := expectMotionEvent(ctx, test.d, singleTouchMatcher(actionDown, expected)); err != nil {
		s.Fatal("Could not verify expected event: ", err)
	}

	for i := 0; i < 5; i++ {
		pointDP.X += 5
		pointDP.Y += 5
		expected = test.expectedPoint(pointDP)

		s.Log("Verifying touch move event at ", expected)
		x, y := tcc.ConvertLocation(pointDP)
		if err := stw.Move(x, y); err != nil {
			s.Fatalf("Could not inject move at (%d, %d): %v", x, y, err)
		}
		if err := expectMotionEvent(ctx, test.d, singleTouchMatcher(actionMove, expected)); err != nil {
			s.Fatal("Could not verify expected event: ", err)
		}
	}

	s.Log("Verifying touch up event at ", expected)
	x, y = tcc.ConvertLocation(pointDP)
	if err := stw.End(); err != nil {
		s.Fatalf("Could not inject end at (%d, %d)", x, y)
	}
	if err := expectMotionEvent(ctx, test.d, singleTouchMatcher(actionUp, expected)); err != nil {
		s.Fatal("Could not verify expected event: ", err)
	}
}
