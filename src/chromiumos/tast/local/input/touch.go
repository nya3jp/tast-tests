// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package input

import (
	"context"
	"fmt"
	"math"
	"math/big"
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
	virt          *os.File // if non-nil, used to hold a virtual device open
	dev           string   // path to underlying device in /dev/input
	nextTouchID   int32
	width         TouchCoord
	height        TouchCoord
	maxTouchSlot  int
	maxTrackingID int
	maxPressure   int

	// clockwise rotation in degree to translate event location. It only supports
	// 0, 90, 180, or 270 degrees.
	rotation int
}

var nextVirtTouchNum = 1 // appended to virtual touchscreen device name

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
			maxTouchSlot:  int(infoSlot.maximum),
			maxTrackingID: int(infoTrackingID.maximum),
			maxPressure:   int(infoPressure.maximum),
		}, nil
	}

	// If we didn't find a real touchscreen, create a virtual one.
	return VirtualTouchscreen(ctx)
}

// VirtualTouchscreen creates a virtual touchscreen device and returns an EventWriter that injects events into it.
func VirtualTouchscreen(ctx context.Context) (*TouchscreenEventWriter, error) {
	const (
		// Most touchscreens use I2C bus. But hardcoding to USB since it is supported
		// in all Chromebook devices.
		busType = 0x3 // BUS_USB from input.h

		// Device constants taken from Chromebook Slate.
		vendor  = 0x2d1f
		product = 0x5143
		version = 0x100

		// Input characteristics.
		props   = 1 << INPUT_PROP_DIRECT
		evTypes = 1<<EV_KEY | 1<<EV_ABS | 1<<EV_MSC

		// Abs axes supported in our virtual device.
		absSupportedAxes = 1<<ABS_X | 1<<ABS_Y | 1<<ABS_PRESSURE | 1<<ABS_MT_SLOT |
			1<<ABS_MT_TOUCH_MAJOR | 1<<ABS_MT_TOUCH_MINOR | 1<<ABS_MT_ORIENTATION |
			1<<ABS_MT_POSITION_X | 1<<ABS_MT_POSITION_Y | 1<<ABS_MT_TOOL_TYPE |
			1<<ABS_MT_TRACKING_ID | 1<<ABS_MT_PRESSURE

		// Abs axis constants. Taken from Chromebook Slate.
		axisMaxX            = 10404
		axisMaxY            = 6936
		axisMaxTracking     = 65535
		axisMaxPressure     = 255
		axisCoordResolution = 40
	)
	axisMaxTouchSlot := 9

	// Include our PID in the device name to be extra careful in case an old bundle process hasn't exited.
	name := fmt.Sprintf("Tast virtual touchscreen %d.%d", os.Getpid(), nextVirtTouchNum)
	nextVirtTouchNum++
	testing.ContextLogf(ctx, "Creating virtual touchscreen device %q", name)

	dev, virt, err := createVirtual(name, devID{busType, vendor, product, version}, props, evTypes,
		map[EventType]*big.Int{
			EV_KEY: makeBigInt([]uint64{0x400, 0, 0, 0, 0, 0}), // BTN_TOUCH
			EV_ABS: big.NewInt(absSupportedAxes),
			EV_MSC: big.NewInt(1 << MSC_TIMESTAMP),
		}, nil)
	if err != nil {
		return nil, err
	}

	f, err := os.Open(dev)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	fd := int(f.Fd())

	for _, entry := range []struct {
		ec   EventCode
		info absInfo
	}{
		{ABS_X, absInfo{0, 0, axisMaxX, 0, 0, axisCoordResolution}},
		{ABS_Y, absInfo{0, 0, axisMaxY, 0, 0, axisCoordResolution}},
		{ABS_PRESSURE, absInfo{0, 0, axisMaxPressure, 0, 0, 0}},
		{ABS_MT_SLOT, absInfo{0, 0, uint32(axisMaxTouchSlot), 0, 0, 0}},
		{ABS_MT_TOUCH_MAJOR, absInfo{0, 0, 255, 0, 0, 1}},
		{ABS_MT_TOUCH_MINOR, absInfo{0, 0, 255, 0, 0, 1}},
		{ABS_MT_ORIENTATION, absInfo{0, 0, 1, 0, 0, 0}},
		{ABS_MT_POSITION_X, absInfo{0, 0, axisMaxX, 0, 0, axisCoordResolution}},
		{ABS_MT_POSITION_Y, absInfo{0, 0, axisMaxY, 0, 0, axisCoordResolution}},
		{ABS_MT_TOOL_TYPE, absInfo{0, 0, 2, 0, 0, 0}},
		{ABS_MT_TRACKING_ID, absInfo{0, 0, axisMaxTracking, 0, 0, 0}},
		{ABS_MT_PRESSURE, absInfo{0, 0, axisMaxPressure, 0, 0, 0}},
	} {
		if err := ioctl(fd, evIOCSAbs(uint(entry.ec)), uintptr(unsafe.Pointer(&entry.info))); err != nil {
			if entry.ec == ABS_MT_SLOT {
				// TODO(ricardoq): ABS_MT_SLOT fails, preventing multitouch support. Further research needed.
				testing.ContextLogf(ctx, "Failed to set ABS_MT_SLOT to %+v. Multitouch disabled", entry.info)
				axisMaxTouchSlot = 0
			} else {
				return nil, errors.Wrapf(err, "failed to set ABS value %d to %+v", entry.ec, entry.info)
			}
		}
	}

	device, err := Device(ctx, dev)
	if err != nil {
		return nil, err
	}
	return &TouchscreenEventWriter{
		rw:            device,
		dev:           dev,
		virt:          virt,
		width:         axisMaxX,
		height:        axisMaxY,
		maxTouchSlot:  axisMaxTouchSlot,
		maxTrackingID: axisMaxTracking,
		maxPressure:   axisMaxPressure,
	}, nil
}

// Close closes the touchscreen device.
func (tsw *TouchscreenEventWriter) Close() error {
	firstErr := tsw.rw.Close()

	// Let go the virtual device if any.
	if tsw.virt != nil {
		if err := tsw.virt.Close(); firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// NewMultiTouchWriter returns a new TouchEventWriter instance. numTouches is how many touches
// are going to be used by the TouchEventWriter.
func (tsw *TouchscreenEventWriter) NewMultiTouchWriter(numTouches int) (*TouchEventWriter, error) {
	if numTouches < 1 || numTouches > tsw.maxTouchSlot {
		return nil, errors.Errorf("requested %d touches; device only supports a max of %d touches", numTouches, tsw.maxTouchSlot+1)
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
// This is affected by the rotation of the screen.
func (tsw *TouchscreenEventWriter) Width() TouchCoord {
	if tsw.rotation == 90 || tsw.rotation == 270 {
		return tsw.height
	}
	return tsw.width
}

// Height returns the height of the touchscreen device, in touchscreen coordinates.
// This is affected by the rotation of the screen.
func (tsw *TouchscreenEventWriter) Height() TouchCoord {
	if tsw.rotation == 90 || tsw.rotation == 270 {
		return tsw.width
	}
	return tsw.height
}

// SetRotation changes the orientation of the touch screen's event to the
// specified degree. The locations of further touch events will be rotated by
// the specified rotation. It will return an error if the specified rotation is
// not supported.
func (tsw *TouchscreenEventWriter) SetRotation(rotation int) error {
	rotation = rotation % 360
	if rotation < 0 {
		rotation += 360
	}
	if rotation != 0 && rotation != 90 && rotation != 180 && rotation != 270 {
		return errors.Errorf("unsupported rotation: %d", rotation)
	}
	tsw.rotation = rotation
	return nil
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
// X and Y must be between [0, touchscreen width) and [0, touchscreen height).
func (ts *TouchState) SetPos(x, y TouchCoord) error {
	if x < 0 || x >= ts.tsw.Width() || y < 0 || y >= ts.tsw.Height() {
		return errors.Errorf("coordinates (%d, %d) outside valid bounds [0, %d), [0, %d)",
			x, y, ts.tsw.Width(), ts.tsw.Height())
	}
	switch ts.tsw.rotation {
	case 90:
		x, y = ts.tsw.width-1-y, x
	case 180:
		x, y = ts.tsw.width-1-x, ts.tsw.height-1-y
	case 270:
		x, y = y, ts.tsw.height-1-x
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

// evIOCSAbs sets an encoded Event-Ioctl-Set-Absolute value to be used for ioctl().
// Similar to the EVIOCSABS found in include/uapi/linux/input.h
func evIOCSAbs(ev uint) uint {
	const sizeofAbsInfo = 0x24
	return iow('E', 0xc0+ev, sizeofAbsInfo)
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

// LongPressAt injects a touch event at (x, y) touchscreen coordinates and wait
// a bit to simulate a touch long press. The wait time should be longer than
// chrome's default long press wait time, which is 500ms.
// See ui/events/gesture_detection/gesture_detector.cc in chromium.
func (stw *SingleTouchEventWriter) LongPressAt(ctx context.Context, x, y TouchCoord) error {
	if err := stw.Move(x, y); err != nil {
		return err
	}

	return testing.Sleep(ctx, 1*time.Second)
}

// DoubleTap injects touch events at (x, y) touchscreen cordinates to simulate a
// double tap.
func (stw *SingleTouchEventWriter) DoubleTap(ctx context.Context, x, y TouchCoord) error {
	for i := 0; i < 2; i++ {
		if err := stw.Move(x, y); err != nil {
			return err
		}
		if err := testing.Sleep(ctx, 100*time.Millisecond); err != nil {
			return err
		}
		if err := stw.End(); err != nil {
			return err
		}
		if err := testing.Sleep(ctx, 100*time.Millisecond); err != nil {
			return err
		}
	}
	return nil
}

// Swipe performs a swipe movement from x0/y0 to x1/y1.
// t represents how long the swipe should last.
// If t is less than 5 milliseconds, 5 milliseconds will be used instead.
// Swipe() does not call End(), allowing the user to concatenate multiple swipes together.
func (stw *SingleTouchEventWriter) Swipe(ctx context.Context, x0, y0, x1, y1 TouchCoord, t time.Duration) error {
	const touchFrequency = 5 * time.Millisecond
	steps := int(t/touchFrequency) + 1
	// A minimum of two touches are needed. One for the start point and another one for the end point.
	if steps < 2 {
		steps = 2
	}
	deltaX := float64(x1-x0) / float64(steps-1)
	deltaY := float64(y1-y0) / float64(steps-1)

	for i := 0; i < steps; i++ {
		x := x0 + TouchCoord(math.Round(deltaX*float64(i)))
		y := y0 + TouchCoord(math.Round(deltaY*float64(i)))
		if err := stw.Move(x, y); err != nil {
			return err
		}

		if err := testing.Sleep(ctx, touchFrequency); err != nil {
			return errors.Wrap(err, "timeout while doing sleep")
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
