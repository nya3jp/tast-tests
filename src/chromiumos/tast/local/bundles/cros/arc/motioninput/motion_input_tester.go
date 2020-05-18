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
	"chromiumos/tast/local/chrome"
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

// Constant definitions for MotionEvent that should be kept in sync with its respective sources.
const (
	ActionDown          Action = "ACTION_DOWN"
	ActionUp            Action = "ACTION_UP"
	ActionMove          Action = "ACTION_MOVE"
	ActionHoverMove     Action = "ACTION_HOVER_MOVE"
	ActionHoverEnter    Action = "ACTION_HOVER_ENTER"
	ActionHoverExit     Action = "ACTION_HOVER_EXIT"
	ActionButtonPress   Action = "ACTION_BUTTON_PRESS"
	ActionButtonRelease Action = "ACTION_BUTTON_RELEASE"

	AxisX        Axis = "AXIS_X"
	AxisY        Axis = "AXIS_Y"
	AxisPressure Axis = "AXIS_PRESSURE"

	SourceTouchscreen   Source = "touchscreen"
	SourceMouse         Source = "mouse"
	SourceMouseRelative Source = "mouse_relative"
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
	// Batched is true if this event was included in the history of another MotionEvent in Android,
	// and false otherwise. See more information about batching at:
	// https://cs.android.com/android/platform/superproject/+/master:frameworks/base/core/java/android/view/MotionEvent.java;l=93
	Batched bool `json:"batched"`
}

// Constants for the test application ArcMotionInputTest.apk.
const (
	APK     = "ArcMotionInputTest.apk"
	Package = "org.chromium.arc.testapp.motioninput"

	EventReportingActivity = ".MotionEventReportingActivity"
	PointerCaptureActivity = ".PointerCaptureActivity"

	intentActionClearEvents = Package + ".ACTION_CLEAR_EVENTS"
)

// Tester holds resources associated with ArcMotionInputTest activity.
type Tester struct {
	tconn *chrome.TestConn
	d     *ui.Device
	act   *arc.Activity
}

// NewTester creates a new instance of a tester.
// The provided activity should be started before any of the tester's methods are called.
// All provided arguments must outlive the Tester.
func NewTester(tconn *chrome.TestConn, d *ui.Device, act *arc.Activity) *Tester {
	return &Tester{
		tconn: tconn,
		d:     d,
		act:   act,
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

		// TODO(b/156655077): Remove filtering of batched events after the bug is fixed.
		//  We filter out batched events because in some cases, we observe extraneous events that
		//  are automatically generated in the input pipeline. Since the extraneous events are
		//  generated immediately after real events, they are batched, so we skip batched events.
		for i := 0; i < len(events); {
			if events[i].Batched {
				events = append(events[:i], events[i+1:]...)
				continue
			}
			i++
		}

		if len(events) != len(matchers) {
			return errors.Errorf("did not receive the exact number of events as expected; got: %d, want: %d", len(events), len(matchers))
		}

		for i := 0; i < len(matchers); i++ {
			if err := matchers[i](&events[i]); err != nil {
				return testing.PollBreak(err)
			}
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second})
}

// readMotionEvents unmarshalls the JSON string in the TextView representing the MotionEvents
// received by ArcMotionInputTest.apk, and returns it as a slice of motionEvent.
func (t *Tester) readMotionEvents(ctx context.Context) ([]MotionEvent, error) {
	view := t.d.Object(ui.ID(Package + ":id/motion_event"))
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
	prefixes := []string{"-a", intentActionClearEvents}
	if err := t.act.StartWithArgs(ctx, t.tconn, prefixes, nil); err != nil {
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

// ActionSourceMatcher returns a motionEventMatcher that matches a motionEvent with the provided
// action and source.
func ActionSourceMatcher(a Action, s Source) Matcher {
	return func(event *MotionEvent) error {
		sourceMatches := false
		for _, v := range event.Sources {
			if v == s {
				sourceMatches = true
				break
			}
		}
		if !sourceMatches {
			return errors.Errorf("source does not match: got %v; want %s", event.Sources, s)
		}
		if event.Action != a {
			return errors.Errorf("action does not match: got %s; want: %s", event.Action, a)
		}
		return nil
	}
}

// SinglePointerMatcher returns a motionEventMatcher that matches a motionEvent with a single
// pointer that has the following axes: axisX, axisY, and axisPressure.
func SinglePointerMatcher(a Action, s Source, p coords.Point, pressure float64) Matcher {
	return func(event *MotionEvent) error {
		if err := ActionSourceMatcher(a, s)(event); err != nil {
			return err
		}
		if pointerCount := len(event.PointerAxes); pointerCount != 1 {
			return errors.Errorf("pointer count does not match: got: %d; want: %d", pointerCount, 1)
		}
		axisMatcher := func(axis Axis, expected, epsilon float64) error {
			v := event.PointerAxes[0][axis]
			if math.Abs(v-expected) > epsilon {
				return errors.Errorf("value of axis %s did not match: got %.5f; want %.5f; epsilon %.5f", axis, v, expected, epsilon)
			}
			return nil
		}
		const (
			// coordinateAxisEpsilon is the epsilon value to be used when comparing axis values that
			// represent absolute display coordinates. Scaling and conversions from Chrome to Android's
			// display spaces means absolute coordinates can be off by up to two pixels.
			coordinateAxisEpsilon = 2e0

			// defaultAxisEpsilon is the epsilon value to be used when comparing axis values that do
			// not need to be scaled or converted, like pressure (which is in the range [0,1]). We
			// expect these values to be more precise.
			defaultAxisEpsilon = 1e-5
		)
		if err := axisMatcher(AxisX, float64(p.X), coordinateAxisEpsilon); err != nil {
			return err
		}
		if err := axisMatcher(AxisY, float64(p.Y), coordinateAxisEpsilon); err != nil {
			return err
		}
		if err := axisMatcher(AxisPressure, pressure, defaultAxisEpsilon); err != nil {
			return err
		}
		return nil
	}
}
