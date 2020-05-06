// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package motioninput provides a representation of Android's MotionEvent, and allows communication
// with the test application ArcMotionInputTest.apk via a Tester.
package motioninput

import (
	"context"
	"encoding/json"
	"math"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/testing"
)

// Action represents a MotionEvent's action key.
// The values are from Android's MotionEvent.java.
// See: https://cs.android.com/android/platform/superproject/+/master:frameworks/base/core/java/android/view/MotionEvent.java
type Action string

// Axis represents a MotionEvent's axis key.
// The values are from Android's MotionEvent.java.
// See: https://cs.android.com/android/platform/superproject/+/master:frameworks/base/core/java/android/view/MotionEvent.java
type Axis string

// Source represents an input device's source.
// The values should be kept in sync with ArcMotionInputTest.apk.
type Source string

// Constant definitions that should be kept in sync with its respective sources.
const (
	Down          Action = "ACTION_DOWN"
	Up            Action = "ACTION_UP"
	Move          Action = "ACTION_MOVE"
	HoverMove     Action = "ACTION_HOVER_MOVE"
	HoverEnter    Action = "ACTION_HOVER_ENTER"
	HoverExit     Action = "ACTION_HOVER_EXIT"
	ButtonPress   Action = "ACTION_BUTTON_PRESS"
	ButtonRelease Action = "ACTION_BUTTON_RELEASE"

	X        Axis = "AXIS_X"
	Y        Axis = "AXIS_Y"
	Pressure Axis = "AXIS_PRESSURE"

	Touchscreen Source = "touchscreen"
	Mouse       Source = "mouse"
)

// MotionEvent represents a MotionEvent that was received by the Android application.
// For all Axis values that represent an absolute location, the values are in the
// coordinate space of the Android window (i.e. 0,0 is the top left corner of the application
// window in Android).
type MotionEvent struct {
	Action      Action             `json:"action"`
	DeviceID    int                `json:"device_id"`
	Sources     []Source           `json:"sources"`
	PointerAxes []map[Axis]float64 `json:"pointer_axes"`
}

// ActivityCLS represents the java class of the test Activities.
type ActivityCLS string

// Constants for the test application ArcMotionInputTest.apk.
const (
	APK             = "ArcMotionInputTest.apk"
	PKG             = "org.chromium.arc.testapp.motioninput"
	CLS ActivityCLS = ".MainActivity"

	intentActionClearEvents = PKG + ".ACTION_CLEAR_EVENTS"
)

// Tester holds resources associated with ArcMotionInputTest activity.
type Tester struct {
	arc *arc.ARC
	d   *ui.Device
	act *arc.Activity
	cls ActivityCLS
}

// NewTester creates a new instance of a tester.
// The provided activity should be started before any of the tester's methods are called.
func NewTester(arc *arc.ARC, d *ui.Device, cls ActivityCLS, act *arc.Activity) *Tester {
	return &Tester{
		arc: arc,
		d:   d,
		act: act,
		cls: cls,
	}
}

// Matcher represents a matcher for motionEvent.
type Matcher func(*MotionEvent) error

// ExpectMotionEvents polls readMotionEvents repeatedly until it receives motionEvents that
// successfully match all of the provided motionEventMatchers in order, or until it times out.
func (t *Tester) ExpectMotionEvents(ctx context.Context, matchers ...Matcher) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		events, err := t.readMotionEvents(ctx)
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to read motion event"))
		}

		if len(events) != len(matchers) {
			return errors.Errorf("did not receive the exact number of events as expected; got: %d, want: %d", len(events), len(matchers))
		}

		for i := 0; i < len(matchers); i++ {
			if err := matchers[i](&events[i]); err != nil {
				return err
			}
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second})
}

// readMotionEvents unmarshalls the JSON string in the TextView representing the MotionEvents
// received by ArcMotionInputTest.apk, and returns it as a slice of motionEvent.
func (t *Tester) readMotionEvents(ctx context.Context) ([]MotionEvent, error) {
	view := t.d.Object(ui.ID(PKG + ":id/motion_event"))
	text, err := view.GetText(ctx)
	if err != nil {
		return nil, err
	}

	var events []MotionEvent
	if err := json.Unmarshal([]byte(text), &events); err != nil {
		return nil, err
	}
	return events, nil
}

// ClearMotionEvents tells the test application to clear the events that it is currently reporting,
// and verifies that no events are reported. This is done by sending an intent with the appropriate
// action to Android, which is subsequently picked up by the MotionInputTest application and handled
// appropriately.
func (t *Tester) ClearMotionEvents(ctx context.Context) error {
	componentName := PKG + "/" + string(t.cls)
	if err := t.arc.Command(ctx, "am", "start", "-a", intentActionClearEvents, "-n", componentName).Run(); err != nil {
		return errors.Wrap(err, "failed to send the clear events intent")
	}
	if err := t.ExpectMotionEvents(ctx); err != nil {
		return errors.Wrap(err, "failed to verify that the reported MotionEvents were cleared")
	}
	return nil
}

// ExpectEventsAndClear is a convenience function that verifies expected events and clears the
// events to be ready for the next assertions.
func (t *Tester) ExpectEventsAndClear(ctx context.Context, matchers ...Matcher) error {
	if err := t.ExpectMotionEvents(ctx, matchers...); err != nil {
		return errors.Wrap(err, "failed to verify expected events")
	}
	if err := t.ClearMotionEvents(ctx); err != nil {
		return errors.Wrap(err, "failed to clear events")
	}
	return nil
}

const (
	// CoordinateAxisEpsilon is the epsilon value to be used when comparing axis values that
	// represent absolute display coordinates. Scaling and conversions from Chrome to Android's
	// display spaces means absolute coordinates can be off by up to one pixel.
	CoordinateAxisEpsilon = 1e0

	// DefaultAxisEpsilon is the epsilon value to be used when comparing axis values that do
	// not need to be scaled or converted, like pressure (which is in the range [0,1]). We
	// expect these values to be more precise.
	DefaultAxisEpsilon = 1e-5
)

// SinglePointerMatcher returns a motionEventMatcher that matches a motionEvent with a single
// pointer that has the following axes: axisX, axisY, and axisPressure.
func SinglePointerMatcher(a Action, s Source, p coords.Point, pressure float64) Matcher {
	return func(event *MotionEvent) error {
		sourceMatches := false
		for _, v := range event.Sources {
			if v == s {
				sourceMatches = true
				break
			}
		}
		axisMatcher := func(axis Axis, expected, epsilon float64) error {
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
		} else if err := axisMatcher(X, float64(p.X), CoordinateAxisEpsilon); err != nil {
			return err
		} else if err := axisMatcher(Y, float64(p.Y), CoordinateAxisEpsilon); err != nil {
			return err
		} else if err := axisMatcher(Pressure, pressure, DefaultAxisEpsilon); err != nil {
			return err
		}
		return nil
	}
}
