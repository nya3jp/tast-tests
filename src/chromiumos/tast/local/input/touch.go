// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package input supports injecting input via kernel devices.
// This file in particular implements multi-touch event injecting.
// It implements "Protocol Example B" as defined here:
//	https://www.kernel.org/doc/Documentation/input/multi-touch-protocol.txt
// and here:
//  https://www.kernel.org/doc/Documentation/input/event-codes.txt
// This is partial implementation of the multi-touch specification. Only the
// parts needed for our tests were implemented.
package input

import (
	"context"
	"os"
	"time"
	"unsafe"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// TouchCoord describes an X or Y coordinate in touchscreen coordinates
// (rather than pixels).
type TouchCoord uint32

// TouchscreenEventWriter supports injecting touch events into a touchscreen device.
// TODO(ricardoq): Add support for Multitouch events. Only singletouch events are
// supported at the moment.
type TouchscreenEventWriter struct {
	rw               *RawEventWriter
	nextTouchID      int32
	nextSlot         int32
	width            int
	height           int
	absMaxTouches    int
	absMaxTrackingID int
	absMaxPressure   int
}

// Touchscreen returns an TouchscreenEventWriter to inject events into an arbitrary touchscreen device.
func Touchscreen(ctx context.Context) (*TouchscreenEventWriter, error) {
	infos, err := readDevices(procDevices)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read %v", procDevices)
	}
	for _, info := range infos {
		if !info.isTouchscreen() {
			continue
		}
		testing.ContextLogf(ctx, "Opening touchscreen device %+v", info)

		// get min,max,resolution values
		fd, err := os.Open(info.path)
		if err != nil {
			return nil, err
		}
		defer fd.Close()

		var infoX, infoY, infoSlot, infoTrackingID, infoPressure absInfo
		if err := Ioctl(fd.Fd(), evIOCGAbs(int(ABS_X)), unsafe.Pointer(&infoX)); err != nil {
			return nil, err
		}

		if err := Ioctl(fd.Fd(), evIOCGAbs(int(ABS_Y)), unsafe.Pointer(&infoY)); err != nil {
			return nil, err
		}

		if err := Ioctl(fd.Fd(), evIOCGAbs(int(ABS_MT_SLOT)), unsafe.Pointer(&infoSlot)); err != nil {
			return nil, err
		}

		if err := Ioctl(fd.Fd(), evIOCGAbs(int(ABS_MT_TRACKING_ID)), unsafe.Pointer(&infoTrackingID)); err != nil {
			return nil, err
		}

		if err := Ioctl(fd.Fd(), evIOCGAbs(int(ABS_MT_PRESSURE)), unsafe.Pointer(&infoPressure)); err != nil {
			return nil, err
		}

		device, err := Device(ctx, info.path)
		if err != nil {
			return nil, err
		}
		return &TouchscreenEventWriter{
			rw:               device,
			width:            int(infoX.maximum),
			height:           int(infoY.maximum),
			absMaxTouches:    int(infoSlot.maximum),
			absMaxTrackingID: int(infoTrackingID.maximum),
			absMaxPressure:   int(infoPressure.maximum),
		}, nil
	}
	return nil, errors.New("didn't find touchscreen device")
}

// Close closes the touchscreen device.
func (mw *TouchscreenEventWriter) Close() error {
	return mw.rw.Close()
}

// NewMultiTouchWriter returns a new TouchEventWriter instance. numberOfTouches is how many touches
// are going to be used by the TouchEventWriter.
func (mw *TouchscreenEventWriter) NewMultiTouchWriter(numberOfTouches int) (*TouchEventWriter, error) {

	if numberOfTouches < 1 || numberOfTouches >= mw.absMaxTouches {
		return nil, errors.Errorf("%d is higher than the maximum number of supported touches: %d", numberOfTouches, mw.absMaxTouches)
	}

	tw := TouchEventWriter{mw: mw, touchStartTime: mw.rw.nowFunc()}
	tw.initTouchState(numberOfTouches)
	return &tw, nil
}

// NewSingleTouchWriter returns a new TouchEventWriter instance for just a single touch.
// It is the same as calling NewMultiTouchWriter(1)
func (mw *TouchscreenEventWriter) NewSingleTouchWriter() (*SingleTouchEventWriter, error) {

	stw := SingleTouchEventWriter{TouchEventWriter{mw: mw, touchStartTime: mw.rw.nowFunc()}}
	stw.initTouchState(1)
	return &stw, nil
}

// Width returns the width of the touchscreen device, in touchscreen coordinates.
func (mw *TouchscreenEventWriter) Width() int {
	return mw.width
}

// Height returns the height of the touchscreen device, in touchscreen coordinates.
func (mw *TouchscreenEventWriter) Height() int {
	return mw.height
}

// TouchEventWriter supports injecting touch events into a touchscreen device.
type TouchEventWriter struct {
	mw             *TouchscreenEventWriter
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
	slot        int32
	touchID     int32
	touchMinor  int32
	touchMajor  int32
	absPressure int32
	x           int32
	y           int32
}

// SetX sets TouchState X coordinate.
func (ts *TouchState) SetX(x TouchCoord) {
	ts.x = int32(x)
}

// SetY sets TouchState Y coordinate.
func (ts *TouchState) SetY(y TouchCoord) {
	ts.y = int32(y)
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

// evIOCGAbs returns an encoded Event-Ioctl-Get-Absolute value to be used for Ioclt().
// Similar to the EVIOCGABS found in include/uapi/linux/input.h
func evIOCGAbs(ev int) int {
	const sizeofAbsInfo = 0x24
	return IOR('E', 0x40+ev, sizeofAbsInfo)
}

type kernelEventEntry struct {
	et  EventType
	ec  EventCode
	val int32
}

// Sync sends all the multi-touch events to the kernel.
func (tw *TouchEventWriter) Sync() error {

	// First send the multitouch events
	for _, touch := range tw.touches {
		for _, e := range []kernelEventEntry{
			{EV_ABS, ABS_MT_SLOT, touch.slot},
			{EV_ABS, ABS_MT_TRACKING_ID, touch.touchID},
			{EV_ABS, ABS_MT_POSITION_X, touch.x},
			{EV_ABS, ABS_MT_POSITION_Y, touch.y},
			{EV_ABS, ABS_MT_PRESSURE, touch.absPressure},
			{EV_ABS, ABS_MT_TOUCH_MAJOR, touch.touchMajor},
			{EV_ABS, ABS_MT_TOUCH_MINOR, touch.touchMinor},
		} {
			if err := tw.mw.rw.Event(e.et, e.ec, e.val); err != nil {
				return err
			}
		}
	}

	// Then send the rest of the events
	for _, e := range []kernelEventEntry{
		{EV_KEY, BTN_TOUCH, 1},
		{EV_ABS, ABS_X, tw.touches[0].x},
		{EV_ABS, ABS_Y, tw.touches[0].y},
		{EV_ABS, ABS_PRESSURE, tw.touches[0].absPressure},
	} {
		if err := tw.mw.rw.Event(e.et, e.ec, e.val); err != nil {
			return err
		}
	}
	tw.ended = false

	// And finally sync
	return tw.mw.rw.Sync()
}

// End injects a "touch lift" like if someone were lifting the finger or
// stylus from the surface. "End()" is applied to all active TouchState.
func (tw *TouchEventWriter) End() error {

	for _, touch := range tw.touches {
		for _, e := range []kernelEventEntry{
			{EV_ABS, ABS_MT_SLOT, touch.slot},
			{EV_ABS, ABS_MT_TRACKING_ID, -1},
		} {
			if err := tw.mw.rw.Event(e.et, e.ec, e.val); err != nil {
				return err
			}
		}
	}

	for _, e := range []kernelEventEntry{
		{EV_ABS, ABS_PRESSURE, 0},
		{EV_KEY, BTN_TOUCH, 0},
	} {
		if err := tw.mw.rw.Event(e.et, e.ec, e.val); err != nil {
			return err
		}
	}

	tw.ended = true
	return tw.mw.rw.Sync()
}

// Close cleans-up TouchEventWriter.  This method must be called after using it,
// possibly with the "defer" statement.
func (tw *TouchEventWriter) Close() {
	if !tw.ended {
		tw.End()
	}
}

// Move injects a touch event at x and y touchscreen coordinates. This is applied
// only to the first TouchState. Calling this function is equivalent to:
//  ts := touchEventWriter.TouchState(0)
//  ts.SetX(x)
//  ts.SetY(y)
//  ts.Sync()
func (stw *SingleTouchEventWriter) Move(x, y TouchCoord) error {
	if int(x) >= stw.mw.width || int(y) >= stw.mw.height {
		return errors.Errorf("coordinates (%d, %d) outside valid bounds (%d, %d)",
			x, y,
			stw.mw.width, stw.mw.height)
	}

	stw.touches[0].SetX(x)
	stw.touches[0].SetY(y)
	return stw.Sync()
}

// TouchState returns a TouchState. touchIndex is touch to get.
// One TouchState represents the state of a single touch.
func (tw *TouchEventWriter) TouchState(touchIndex int) *TouchState {
	if touchIndex < 0 || touchIndex >= len(tw.touches) {
		return nil
	}
	return &tw.touches[touchIndex]
}

func (tw *TouchEventWriter) initTouchState(numberOfTouches int) {
	// Values taken from "dumps" on an Eve device.
	// Spec says pressure is in arbitrary units. A value around 25% of the max value
	// seems to be "normal".
	// TouchMajor and TouchMinor were also taken from "dumps".
	const (
		defaultTouchMajor = 5
		defaultTouchMinor = 5
	)
	defaultPressure := int32(tw.mw.absMaxPressure / 4)

	tw.touches = make([]TouchState, numberOfTouches, numberOfTouches)

	for i := 0; i < numberOfTouches; i++ {
		tw.touches[i].absPressure = defaultPressure
		tw.touches[i].touchMajor = defaultTouchMajor
		tw.touches[i].touchMinor = defaultTouchMinor
		tw.touches[i].touchID = tw.mw.nextTouchID
		tw.touches[i].slot = int32(i)

		tw.mw.nextTouchID = (tw.mw.nextTouchID + 1) % int32(tw.mw.absMaxTrackingID)
	}
}
