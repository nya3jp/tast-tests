// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"encoding/json"
	"math"

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
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name:              "vm",
			ExtraAttr:         []string{"informational"},
			ExtraSoftwareDeps: []string{"android_vm"},
			Pre:               arc.VMBooted(),
		}},
	})
}

type motionEventAction string
type motionEventAxis string
type inputDeviceSource string

const (
	// Constants from Android's MotionEvent.java
	actionDown      motionEventAction = "ACTION_DOWN"
	actionUp        motionEventAction = "ACTION_UP"
	actionMove      motionEventAction = "ACTION_MOVE"
	actionHoverMove motionEventAction = "ACTION_HOVER_MOVE"

	// Constants from Android's MotionEvent.java
	axisX        motionEventAxis = "AXIS_X"
	axisY        motionEventAxis = "AXIS_Y"
	axisPressure motionEventAxis = "AXIS_PRESSURE"

	// Must be kept in sync with ArcMotionInputTest.apk
	srcKeyboard      inputDeviceSource = "keyboard"
	srcDpad          inputDeviceSource = "dpad"
	srcTouchscreen   inputDeviceSource = "touchscreen"
	srcMouse         inputDeviceSource = "mouse"
	srcStylus        inputDeviceSource = "stylus"
	srcTrackball     inputDeviceSource = "trackball"
	srcMouseRelative inputDeviceSource = "mouse_relative"
	srcTouchpad      inputDeviceSource = "touchpad"
	srcJoystick      inputDeviceSource = "joystick"
	srcGamepad       inputDeviceSource = "gamepad"
)

// Information about a MotionEvent that was received by the Android application.
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
	motionInputTestApk = "ArcMotionInputTest.apk"
	motionInputTestPkg = "org.chromium.arc.testapp.motioninput"
	motionInputTestCls = ".MainActivity"
)

func readMotionEvent(ctx context.Context, d *ui.Device) (*motionEvent, error) {
	view := d.Object(ui.ID(motionInputTestPkg + ":id/motion_event"))
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

type motionEventMatcher func(*motionEvent) error

func expectMotionEvent(ctx context.Context, d *ui.Device, match motionEventMatcher) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		event, err := readMotionEvent(ctx, d)
		if err != nil {
			return err
		}
		return match(event)
	}, nil)
}

func singlePointMatcher(a motionEventAction, s inputDeviceSource, p coords.Point, pressure float64) motionEventMatcher {
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
				return errors.Errorf("value of axis %s did not match: got %.5f; wanted %.5f; epsilon %.5f", axis, v, expected, epsilon)
			}
			return nil
		}
		if !sourceMatches {
			return errors.Errorf("source does not match: wanted %s", s)
		} else if event.Action != a {
			return errors.Errorf("action does not match: got %s, wanted: %s", event.Action, a)
		} else if pointerCount := len(event.PointerAxes); pointerCount != 1 {
			return errors.Errorf("pointer count does not match: got: %d; wanted: %d", pointerCount, 1)
		} else if err := axisMatcher(axisX, float64(p.X), 1); err != nil {
			return err
		} else if err := axisMatcher(axisY, float64(p.Y), 1); err != nil {
			return err
		} else if err := axisMatcher(axisPressure, pressure, 1e-5); err != nil {
			return err
		}
		return nil
	}
}

func singleTouchMatcher(a motionEventAction, p coords.Point) motionEventMatcher {
	return singlePointMatcher(a, srcTouchscreen, p, 1)
}

// MotionInput installs and runs an Android application, injects various input events into ChromeOS
// through uinput, and verifies that those events were received by the Android application in the
// correct screen location.
func MotionInput(ctx context.Context, s *testing.State) {
	p := s.PreValue().(arc.PreData)
	cr := p.Chrome
	a := p.ARC

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}

	runTest := func(ctx context.Context, s *testing.State, wmParams *motionInputWmParams) {

		args := motionInputTestArgs{}
		args.wmParams = wmParams

		deviceMode := "clamshell"
		if wmParams.TabletMode {
			deviceMode = "tablet"
		}
		s.Logf("Setting device to %v mode", deviceMode)
		cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, wmParams.TabletMode)
		if err != nil {
			s.Fatal("Failed to ensure tablet mode enabled: ", err)
		}
		defer cleanup(ctx)

		args.d, err = ui.NewDevice(ctx, a)
		if err != nil {
			s.Fatal("Failed initializing UI Automator: ", err)
		}
		defer args.d.Close()

		infos, err := display.GetInfo(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to get display info: ", err)
		}
		if len(infos) == 0 {
			s.Fatal("No display found")
		}
		for i := range infos {
			if infos[i].IsInternal {
				args.dispInfo = &infos[i]
			}
		}
		if args.dispInfo == nil {
			s.Log("No internal display found. Default to the first display")
			args.dispInfo = &infos[0]
		}

		args.scale, err = args.dispInfo.GetEffectiveDeviceScaleFactor()
		if err != nil {
			s.Fatal("Failed to get effective device scale factor: ", err)
		}

		s.Log("Installing apk ", motionInputTestApk)
		if err := a.Install(ctx, arc.APKPath(motionInputTestApk)); err != nil {
			s.Fatalf("Failed installing %s: %v", motionInputTestApk, err)
		}

		act, err := arc.NewActivity(a, motionInputTestPkg, motionInputTestCls)
		if err != nil {
			s.Fatal("Failed to create an activity: ", err)
		}
		defer act.Close()

		if err := act.Start(ctx, tconn); err != nil {
			s.Fatal("Failed to start an activity: ", err)
		}
		defer act.Stop(ctx)

		if wmParams.WmEventToSend != nil {
			event := wmParams.WmEventToSend.(ash.WMEventType)
			s.Log("Sending wm event: ", event)
			if _, err := ash.SetARCAppWindowState(ctx, tconn, motionInputTestPkg, event); err != nil {
				s.Fatalf("Failed to set ARC app window state with event %s: %v", event, err)
			}
		}

		args.w, err = ash.GetARCAppWindowInfo(ctx, tconn, motionInputTestPkg)
		if err != nil {
			s.Fatal("Failed to get ARC app window info: ", err)
		}

		verifyTouchscreen(ctx, s, args)
	}

	for _, params := range []motionInputWmParams{
		{
			Name:          "Clamshell Normal",
			TabletMode:    false,
			WmEventToSend: ash.WMEventNormal,
		}, {
			Name:          "Clamshell Fullscreen",
			TabletMode:    false,
			WmEventToSend: ash.WMEventFullscreen,
		}, {
			Name:          "Clamshell Maximized",
			TabletMode:    false,
			WmEventToSend: ash.WMEventMaximize,
		},
		// TODO: Determine why the first motion event has incorrect coordinates in tablet mode.
		//{
		//	Name:          "Tablet Mode",
		//	TabletMode:    true,
		//	WmEventToSend: nil,
		//},
	} {
		s.Run(ctx, params.Name, func(ctx context.Context, s *testing.State) {
			runTest(ctx, s, &params)
		})
	}
}

type wmEventToSend interface{}

type motionInputWmParams struct {
	Name          string
	TabletMode    bool          // If true, the device will be put in tablet mode
	WmEventToSend wmEventToSend // Must be of type ash.WMEventType, can be nil
}

type motionInputTestArgs struct {
	dispInfo *display.Info
	scale    float64
	w        *ash.Window
	d        *ui.Device
	wmParams *motionInputWmParams
}

func getExpectedPointPx(args motionInputTestArgs, p coords.Point) coords.Point {
	insetLeft := args.w.BoundsInRoot.Left
	insetTop := args.w.BoundsInRoot.Top
	// If the caption is visible, its height should be added to the inset.
	if !args.wmParams.TabletMode && args.wmParams.WmEventToSend.(ash.WMEventType) != ash.WMEventFullscreen {
		insetTop += args.w.CaptionHeight
	}
	return coords.Point{X: int(float64(p.X-insetLeft) * args.scale), Y: int(float64(p.Y-insetTop) * args.scale)}
}

func verifyTouchscreen(ctx context.Context, s *testing.State, args motionInputTestArgs) {
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

	tcc := tew.NewTouchCoordConverter(args.dispInfo.Bounds.Size())

	pointDp := args.w.BoundsInRoot.CenterPoint()
	expectedPoint := getExpectedPointPx(args, pointDp)

	s.Log("Verifying touch down event at ", expectedPoint)
	x, y := tcc.ConvertLocation(pointDp)
	if err := stw.Move(x, y); err != nil {
		s.Fatalf("Could not inject move at (%d, %d)", x, y)
	}
	if err := expectMotionEvent(ctx, args.d, singleTouchMatcher(actionDown, expectedPoint)); err != nil {
		s.Fatal("Could not verify expected event: ", err)
	}

	for i := 0; i < 5; i++ {
		pointDp.X += 5
		pointDp.Y += 5
		expectedPoint = getExpectedPointPx(args, pointDp)

		s.Log("Verifying touch move event at ", expectedPoint)
		x, y := tcc.ConvertLocation(pointDp)
		if err := stw.Move(x, y); err != nil {
			s.Fatalf("Could not inject move at (%d, %d): %v", x, y, err)
		}
		if err := expectMotionEvent(ctx, args.d, singleTouchMatcher(actionMove, expectedPoint)); err != nil {
			s.Fatal("Could not verify expected event: ", err)
		}
	}

	s.Log("Verifying touch up event at ", expectedPoint)
	x, y = tcc.ConvertLocation(pointDp)
	if err := stw.End(); err != nil {
		s.Fatalf("Could not inject end at (%d, %d)", x, y)
	}
	if err := expectMotionEvent(ctx, args.d, singleTouchMatcher(actionUp, expectedPoint)); err != nil {
		s.Fatal("Could not verify expected event: ", err)
	}
}
