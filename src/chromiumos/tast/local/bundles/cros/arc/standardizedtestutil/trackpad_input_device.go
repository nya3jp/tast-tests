// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package standardizedtestutil

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/input"
)

// Trackpad related constants. These values were derived experimentally and
// should work on both physical, and virtual trackpads.
const (
	TrackpadMajorSize             = 240
	TrackpadMinorSize             = 180
	TrackpadClickPressure         = 25
	TrackpadGesturePressure       = 10
	TrackpadVerticalScrollAmount  = 900
	TrackpadScrollDuration        = 200 * time.Millisecond
	TrackpadFingerSeparation      = 350
	TrackpadZoomDistancePerFinger = 450
	TrackpadZoomDuration          = 200 * time.Millisecond
)

// TrackpadInputDevice abstracts away trackpad related implementation details
// and provides an interface for sending high level trackpad actions to Android
// activities in a standardized, and reliable way.
type TrackpadInputDevice struct {
	ctx            context.Context
	testParameters TestFuncParams
	tew            *input.TrackpadEventWriter
}

// NewTrackpadInputDevice creates a new TrackpadInputDevice instance.
func NewTrackpadInputDevice(ctx context.Context, testParameters TestFuncParams) (*TrackpadInputDevice, error) {
	if err := validatePointerCanBeUsed(ctx, testParameters); err != nil {
		return nil, errors.Wrap(err, "the trackpad cannot be used")
	}

	// Setup the trackpad.
	tew, err := input.Trackpad(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed initialize the trackpad device")
	}

	return &TrackpadInputDevice{
		ctx:            ctx,
		testParameters: testParameters,
		tew:            tew,
	}, nil
}

// Close closes the trackpad input device and frees all resources.
func (tid *TrackpadInputDevice) Close() error {
	if tid.tew != nil {
		return tid.tew.Close()
	}

	return nil
}

// ClickObject performs a click on the provided element.
func (tid *TrackpadInputDevice) ClickObject(selector *ui.Object, trackpadButton PointerButton) error {
	if err := centerPointerOnObject(tid.ctx, tid.testParameters, selector); err != nil {
		return errors.Wrap(err, "failed to move the trackpad into position")
	}

	stw, err := tid.tew.NewSingleTouchWriter()
	if err != nil {
		return errors.Wrap(err, "unable to initialize the touch writer")
	}

	// Setup the trackpad to simulate a finger click on the next event.
	if err := stw.SetSize(tid.ctx, TrackpadMajorSize, TrackpadMinorSize); err != nil {
		return errors.Wrap(err, "unable to set size")
	}

	if err := stw.SetPressure(TrackpadClickPressure); err != nil {
		return errors.Wrap(err, "unable to set pressure")
	}

	// Setup the action intent
	switch trackpadButton {
	case LeftPointerButton:
		// A left click only requires a single touch.
		stw.SetIsBtnToolFinger(true)
	case RightPointerButton:
		// A left click only requires a double tap.
		stw.SetIsBtnToolDoubleTap(true)
	default:
		return errors.Errorf("invalid button provided: %v", trackpadButton)
	}

	// Perform the 'tap' at the center of the touchpad. The pointer has already been positioned
	// so the move event is just signaling where on the trackpad the event takes place.
	centerX := tid.tew.Width() / 2
	centerY := tid.tew.Height() / 2
	if err := stw.Move(centerX, centerY); err != nil {
		return errors.Wrap(err, "unable to initiate click position")
	}

	if err := stw.End(); err != nil {
		return errors.Wrap(err, "unable to end click")
	}

	return nil
}

// Scroll moves the pointer on to the provided element, and performs
// a two-finger scroll gesture.
func (tid *TrackpadInputDevice) Scroll(selector *ui.Object, scrollDirection ScrollDirection) error {
	if err := centerPointerOnObject(tid.ctx, tid.testParameters, selector); err != nil {
		return errors.Wrap(err, "failed to move the trackpad into position")
	}

	mtw, err := tid.tew.NewMultiTouchWriter(2)
	if err != nil {
		return errors.Wrap(err, "unable to initialize multi-touch writer")
	}
	defer mtw.Close()

	if err := initializeWriterForTwoFingerTrackpadGesture(tid.ctx, mtw); err != nil {
		return errors.Wrap(err, "unable to setup writer for two finger gestures")
	}

	// Calculate where to scroll to based on the provided direction.
	x := tid.tew.Width() / 2
	y := tid.tew.Height() / 2

	scrollToX := x
	scrollToY := y
	switch scrollDirection {
	case DownScroll:
		scrollToY = y + TrackpadVerticalScrollAmount
	case UpScroll:
		scrollToY = y - TrackpadVerticalScrollAmount
	default:
		return errors.Errorf("invalid scroll direction: %v", scrollDirection)
	}

	// Move both fingers accordingly.
	if err := mtw.DoubleSwipe(tid.ctx, x, y, scrollToX, scrollToY, input.TouchCoord(TrackpadFingerSeparation), TrackpadScrollDuration); err != nil {
		return errors.Wrap(err, "unable to perform the scroll")
	}

	if err := mtw.End(); err != nil {
		return errors.Wrap(err, "unable to end the scroll")
	}

	return nil
}

// Zoom moves the pointer on to the provided element, and performs
// a two-finger zoom gesture.
func (tid *TrackpadInputDevice) Zoom(selector *ui.Object, zoomType ZoomType) error {
	if err := centerPointerOnObject(tid.ctx, tid.testParameters, selector); err != nil {
		return errors.Wrap(err, "failed to move the trackpad into position")
	}

	mtw, err := tid.tew.NewMultiTouchWriter(2)
	if err != nil {
		return errors.Wrap(err, "unable to initialize multi-touch writer")
	}
	defer mtw.Close()

	if err := initializeWriterForTwoFingerTrackpadGesture(tid.ctx, mtw); err != nil {
		return errors.Wrap(err, "unable to setup writer for two finger gestures")
	}

	// The zoom can start at the middle of the trackpad.
	x := tid.tew.Width() / 2
	y := tid.tew.Height() / 2

	// Perform the appropriate zoom.
	switch zoomType {
	case ZoomIn:
		if err := mtw.ZoomIn(tid.ctx, x, y, TrackpadZoomDistancePerFinger, TrackpadZoomDuration); err != nil {
			return errors.Wrap(err, "unable to zoom in")
		}
	case ZoomOut:
		if err := mtw.ZoomOut(tid.ctx, x, y, TrackpadZoomDistancePerFinger, TrackpadZoomDuration); err != nil {
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
