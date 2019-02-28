// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package input

import (
	"context"
	"math"
	"os"
	"time"
	"unsafe"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// TouchCoord describes an X or Y coordinate in touchscreen coordinates
// (rather than pixels).
type TouchCoord int32

// TouchscreenEventWriter supports injecting touch events into a touchscreen device.
// It supports multitouch as defined in "Protocol Example B" here:
//  https://www.kernel.org/doc/Documentation/input/multi-touch-protocol.txt
//  https://www.kernel.org/doc/Documentation/input/event-codes.txt
// This is partial implementation of the multi-touch specification. Each injected
// touch event contains the following codes:
//  - ABS_MT_TRACKING_ID
//  - ABS_MT_POSITION_X & ABS_X
//  - ABS_MT_POSITION_Y & ABS_Y
//  - ABS_MT_PRESSURE & ABS_PRESSURE
//  - ABS_MT_TOUCH_MAJOR
//  - ABS_MT_TOUCH_MINOR
//  - BTN_TOUCH
// Any other code, like MSC_TIMESTAMP, is not implemented.
type TouchscreenEventWriter struct {
	rw            *RawEventWriter
	nextTouchID   int32
	width         TouchCoord
	height        TouchCoord
	maxTouches    int
	maxTrackingID int
	maxPressure   int
}

// Touchscreen returns an TouchscreenEventWriter to inject events into an arbitrary touchscreen device.
func Touchscreen(ctx context.Context) (*TouchscreenEventWriter, error) {
	infos, err := readDevices("")
	if err != nil {
		return nil, errors.Wrap(err, "failed to read devices")
	}
	for _, info := range infos {
		if !info.isTouchscreen() {
			continue
		}
		testing.ContextLogf(ctx, "Opening touchscreen device %+v", info)

		// Get touchscreen properties: bounds, max touches, max pressure and max track id.
		f, err := os.Open(info.path)
		if err != nil {
			return nil, err
		}
		defer f.Close()

		var infoX, infoY, infoSlot, infoTrackingID, infoPressure absInfo
		for _, entry := range []struct {
			ec  EventCode
			dst *absInfo
		}{
			{ABS_X, &infoX},
			{ABS_Y, &infoY},
			{ABS_MT_SLOT, &infoSlot},
			{ABS_MT_TRACKING_ID, &infoTrackingID},
			{ABS_MT_PRESSURE, &infoPressure},
		} {
			if err := ioctl(int(f.Fd()), evIOCGAbs(uint(entry.ec)), uintptr(unsafe.Pointer(entry.dst))); err != nil {
				return nil, err
			}
		}

		if infoTrackingID.maximum < infoSlot.maximum {
			return nil, errors.Errorf("invalid MT tracking ID %d; should be >= max slots %d",
				infoTrackingID.maximum, infoSlot.maximum)
		}

		device, err := Device(ctx, info.path)
		if err != nil {
			return nil, err
		}

		return &TouchscreenEventWriter{
			rw:            device,
			width:         TouchCoord(infoX.maximum),
			height:        TouchCoord(infoY.maximum),
			maxTouches:    int(infoSlot.maximum),
			maxTrackingID: int(infoTrackingID.maximum),
			maxPressure:   int(infoPressure.maximum),
		}, nil
	}
	return nil, errors.New("didn't find touchscreen device")
}

// Close closes the touchscreen device.
func (tsw *TouchscreenEventWriter) Close() error {
	return tsw.rw.Close()
}

// NewMultiTouchWriter returns a new TouchEventWriter instance. numTouches is how many touches
// are going to be used by the TouchEventWriter.
func (tsw *TouchscreenEventWriter) NewMultiTouchWriter(numTouches int) (*TouchEventWriter, error) {
	if numTouches < 1 || numTouches > tsw.maxTouches {
		return nil, errors.Errorf("touch count %d not in range [1, %d]", numTouches, tsw.maxTouches)
	}

	tw := TouchEventWriter{tsw: tsw, touchStartTime: tsw.rw.nowFunc()}
	tw.initTouchState(numTouches)
	return &tw, nil
}

// NewSingleTouchWriter returns a new SingleTouchEventWriter instance.
// The difference between calling NewSingleTouchWriter() and NewMultiTouchWriter(1)
// is that NewSingleTouchWriter() has the extra helper Move() method.
func (tsw *TouchscreenEventWriter) NewSingleTouchWriter() (*SingleTouchEventWriter, error) {
	stw := SingleTouchEventWriter{TouchEventWriter{tsw: tsw, touchStartTime: tsw.rw.nowFunc()}}
	stw.initTouchState(1)
	return &stw, nil
}

// Width returns the width of the touchscreen device, in touchscreen coordinates.
func (tsw *TouchscreenEventWriter) Width() TouchCoord {
	return tsw.width
}

// Height returns the height of the touchscreen device, in touchscreen coordinates.
func (tsw *TouchscreenEventWriter) Height() TouchCoord {
	return tsw.height
}

// TouchEventWriter supports injecting touch events into a touchscreen device.
type TouchEventWriter struct {
	tsw            *TouchscreenEventWriter
	touches        []TouchState
	touchStartTime time.Time
	ended          bool
}

// SingleTouchEventWriter supports injecting a single touch into a touchscreen device.
type SingleTouchEventWriter struct {
	TouchEventWriter
}

// TouchState contains the state of a single touch event.
type TouchState struct {
	tsw         *TouchscreenEventWriter
	slot        int32
	touchID     int32
	touchMinor  int32
	touchMajor  int32
	absPressure int32
	x           TouchCoord
	y           TouchCoord
}

// SetPos sets TouchState X and Y coordinates.
func (ts *TouchState) SetPos(x, y TouchCoord) error {
	if x >= ts.tsw.width || y >= ts.tsw.height {
		return errors.Errorf("coordinates (%d, %d) outside valid bounds (%d, %d)",
			x, y, ts.tsw.width, ts.tsw.height)
	}
	ts.x = x
	ts.y = y
	return nil
}

// absInfo corresponds to a input_absinfo struct.
// Taken from: include/uapi/linux/input.h
type absInfo struct {
	value      uint32
	minimum    uint32
	maximum    uint32
	fuzz       uint32
	flat       uint32
	resolution uint32
}

// evIOCGAbs returns an encoded Event-Ioctl-Get-Absolute value to be used for ioctl().
// Similar to the EVIOCGABS found in include/uapi/linux/input.h
func evIOCGAbs(ev uint) uint {
	const sizeofAbsInfo = 0x24
	return ior('E', 0x40+ev, sizeofAbsInfo)
}

type kernelEventEntry struct {
	et  EventType
	ec  EventCode
	val int32
}

// Send sends all the multi-touch events to the kernel.
func (tw *TouchEventWriter) Send() error {
	// First send the multitouch event codes.
	for _, touch := range tw.touches {
		for _, e := range []kernelEventEntry{
			{EV_ABS, ABS_MT_SLOT, touch.slot},
			{EV_ABS, ABS_MT_TRACKING_ID, touch.touchID},
			{EV_ABS, ABS_MT_POSITION_X, int32(touch.x)},
			{EV_ABS, ABS_MT_POSITION_Y, int32(touch.y)},
			{EV_ABS, ABS_MT_PRESSURE, touch.absPressure},
			{EV_ABS, ABS_MT_TOUCH_MAJOR, touch.touchMajor},
			{EV_ABS, ABS_MT_TOUCH_MINOR, touch.touchMinor},
		} {
			if err := tw.tsw.rw.Event(e.et, e.ec, e.val); err != nil {
				return err
			}
		}
	}

	// Then send the rest of the event codes.
	for _, e := range []kernelEventEntry{
		{EV_KEY, BTN_TOUCH, 1},
		{EV_ABS, ABS_X, int32(tw.touches[0].x)},
		{EV_ABS, ABS_Y, int32(tw.touches[0].y)},
		{EV_ABS, ABS_PRESSURE, tw.touches[0].absPressure},
	} {
		if err := tw.tsw.rw.Event(e.et, e.ec, e.val); err != nil {
			return err
		}
	}
	tw.ended = false

	// And finally sync.
	return tw.tsw.rw.Sync()
}

// End injects a "touch lift" like if someone were lifting the finger or
// stylus from the surface. All active TouchStates are ended.
func (tw *TouchEventWriter) End() error {
	for _, touch := range tw.touches {
		for _, e := range []kernelEventEntry{
			{EV_ABS, ABS_MT_SLOT, touch.slot},
			{EV_ABS, ABS_MT_TRACKING_ID, -1},
		} {
			if err := tw.tsw.rw.Event(e.et, e.ec, e.val); err != nil {
				return err
			}
		}
	}

	for _, e := range []kernelEventEntry{
		{EV_ABS, ABS_PRESSURE, 0},
		{EV_KEY, BTN_TOUCH, 0},
	} {
		if err := tw.tsw.rw.Event(e.et, e.ec, e.val); err != nil {
			return err
		}
	}

	tw.ended = true
	return tw.tsw.rw.Sync()
}

// Close cleans up TouchEventWriter. This method must be called after using it,
// possibly with the "defer" statement.
func (tw *TouchEventWriter) Close() {
	if !tw.ended {
		tw.End()
	}
}

// Move injects a touch event at x and y touchscreen coordinates. This is applied
// only to the first TouchState. Calling this function is equivalent to:
//  ts := touchEventWriter.TouchState(0)
//  ts.SetPos(x, y)
//  ts.Send()
func (stw *SingleTouchEventWriter) Move(x, y TouchCoord) error {
	if err := stw.touches[0].SetPos(x, y); err != nil {
		return err
	}
	return stw.Send()
}

// Swipe performs a swipe movement from x0/y0 to x1/y1. The smoothness is defined in steps.
// If steps is less than two, two steps will be used.
// Each step execution is throttled to 5ms per step. So for a 100 steps, the swipe will take about 1/2 second to complete.
// Swipe() does not call End(), so it is possible to concatenate multiple swipes together.
func (stw *SingleTouchEventWriter) Swipe(ctx context.Context, x0, y0, x1, y1 TouchCoord, steps int) error {
	if steps < 2 {
		steps = 2
	}
	// steps-1 since we guarantee that one point will be at the beginning of
	// the swipe, and another one at the end.
	deltaX := float64(x1-x0) / float64(steps-1)
	deltaY := float64(y1-y0) / float64(steps-1)

	for i := 0; i < steps; i++ {
		x := float64(x0) + deltaX*float64(i)
		y := float64(y0) + deltaY*float64(i)
		stw.Move(TouchCoord(math.Round(x)), TouchCoord(math.Round(y)))

		// Small delay after each touch.
		select {
		case <-time.After(5 * time.Millisecond):
		case <-ctx.Done():
			return errors.Wrap(ctx.Err(), "timeout while doing sleep")
		}
	}
	return nil
}

// TouchState returns a TouchState. touchIndex is touch to get.
// One TouchState represents the state of a single touch.
func (tw *TouchEventWriter) TouchState(touchIndex int) *TouchState {
	return &tw.touches[touchIndex]
}

func (tw *TouchEventWriter) initTouchState(numTouches int) {
	// Values taken from "dumps" on an Eve device.
	// Spec says pressure is in arbitrary units. A value around 25% of the max value seems to be "normal".
	// TouchMajor and TouchMinor were also taken from "dumps".
	const (
		defaultTouchMajor = 5
		defaultTouchMinor = 5
	)
	defaultPressure := int32(tw.tsw.maxPressure/4) + 1

	tw.touches = make([]TouchState, numTouches)

	for i := 0; i < numTouches; i++ {
		tw.touches[i].tsw = tw.tsw
		tw.touches[i].absPressure = defaultPressure
		tw.touches[i].touchMajor = defaultTouchMajor
		tw.touches[i].touchMinor = defaultTouchMinor
		tw.touches[i].touchID = tw.tsw.nextTouchID
		tw.touches[i].slot = int32(i)

		tw.tsw.nextTouchID = (tw.tsw.nextTouchID + 1) % int32(tw.tsw.maxTrackingID)
	}
}
