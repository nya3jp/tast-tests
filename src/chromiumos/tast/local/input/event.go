// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package input supports injecting input events via kernel devices.
package input

import (
	"context"
	"encoding/binary"
	"io"
	"os"
	"syscall"
	"time"
	"unsafe"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

//go:generate go run gen/gen_constants.go ../../../../../../../third_party/kernel/v4.14/include/uapi/linux/input-event-codes.h generated_constants.go
//go:generate go fmt generated_constants.go

// RawEventWriter supports injecting input events into a device.
type RawEventWriter struct {
	w       io.WriteCloser // device
	nowFunc func() time.Time
}

// KeyboardEventWriter supports injecting events into a keyboard device.
type KeyboardEventWriter struct {
	rw   *RawEventWriter
	fast bool // if true, do not sleep after type; useful for unit tests
}

// MultiTouchEventWriter supports injecting touch events into a touchscreen device.
type MultiTouchEventWriter struct {
	rw          *RawEventWriter
	nextTouchID int32
	touchInfoX  EventAbsInfo
	touchInfoY  EventAbsInfo
}

// Keyboard returns an EventWriter to inject events into an arbitrary keyboard device.
func Keyboard(ctx context.Context) (*KeyboardEventWriter, error) {
	infos, err := readDevices(procDevices)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read %v", procDevices)
	}
	for _, info := range infos {
		if !info.isKeyboard() {
			continue
		}
		testing.ContextLogf(ctx, "Opening keyboard device %+v", info)
		device, err := Device(ctx, info.path)
		if err != nil {
			return nil, err
		}
		return &KeyboardEventWriter{
			rw:   device,
			fast: false}, nil
	}
	return nil, errors.New("didn't find keyboard device")
}

// Close closes the keyboard device.
func (kw *KeyboardEventWriter) Close() error {
	return kw.rw.Close()
}

// Touchscreen returns an EventWriter to inject events into an arbitrary touchscreen device.
func Touchscreen(ctx context.Context) (*MultiTouchEventWriter, error) {
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

		var infoX, infoY EventAbsInfo
		if err := Ioctl(fd.Fd(), evIOCGAbs(0), unsafe.Pointer(&infoX)); err != nil {
			return nil, err
		}

		if err := Ioctl(fd.Fd(), evIOCGAbs(1), unsafe.Pointer(&infoY)); err != nil {
			return nil, err
		}

		device, err := Device(ctx, info.path)
		if err != nil {
			return nil, err
		}
		return &MultiTouchEventWriter{
			rw:          device,
			nextTouchID: 0,
			touchInfoX:  infoX,
			touchInfoY:  infoY}, nil
	}
	return nil, errors.New("didn't find touchscreen device")
}

// Close closes the touchscreen device.
func (mw *MultiTouchEventWriter) Close() error {
	return mw.rw.Close()
}

// Device returns a RawEventWriter for injecting input events into the input event device at path.
func Device(ctx context.Context, path string) (*RawEventWriter, error) {
	f, err := os.OpenFile(path, os.O_WRONLY, 0600)
	if err != nil {
		return nil, err
	}
	return &RawEventWriter{f, time.Now}, nil
}

// Close closes the device.
func (ew *RawEventWriter) Close() error {
	return ew.w.Close()
}

// Event injects an event containing the supplied values into the device.
func (ew *RawEventWriter) Event(et EventType, ec EventCode, val int32) error {
	tv := syscall.NsecToTimeval(ew.nowFunc().UnixNano())

	// input_event contains a timeval struct, which uses "long" for its members.
	// binary.Write wants explicitly-sized data, so we need to pass a different
	// struct depending on the system's int size.
	var ev interface{}
	switch intSize := unsafe.Sizeof(int(1)); intSize {
	case 4:
		ev = &event32{int32(tv.Sec), int32(tv.Usec), uint16(et), uint16(ec), val}
	case 8:
		ev = &event64{tv, uint16(et), uint16(ec), val}
	default:
		return errors.Errorf("unexpected int size of %d byte(s)", intSize)
	}

	// Little-endian is appropriate regardless of the system's underlying endianness.
	return binary.Write(ew.w, binary.LittleEndian, ev)
}

// event32 corresponds to a 32-bit input_event struct.
type event32 struct {
	Sec, Usec  int32
	Type, Code uint16
	Val        int32
}

// event64 corresponds to a 64-bit input_event struct.
type event64 struct {
	Tv         syscall.Timeval
	Type, Code uint16
	Val        int32
}

// Sync writes a synchronization event delineating a packet of input data occurring at a single point in time.
// It's shorthand for Event(t, EV_SYN, SYN_REPORT, 0).
func (ew *RawEventWriter) Sync() error {
	return ew.Event(EV_SYN, SYN_REPORT, 0)
}

// sendKey writes a EV_KEY event containing the specified code and value, followed by a EV_SYN event.
// If firstErr points at a non-nil error, no events are written.
// If an error is encountered, it is saved to the address pointed to by firstErr.
func (kw *KeyboardEventWriter) sendKey(ec EventCode, val int32, firstErr *error) {
	if *firstErr == nil {
		*firstErr = kw.rw.Event(EV_KEY, ec, val)
	}
	if *firstErr == nil {
		*firstErr = kw.rw.Sync()
	}
}

// Type injects key events suitable for generating the string s.
// Only characters that can be typed using a QWERTY keyboard are supported,
// and the current keyboard layout must be QWERTY. The left Shift key is automatically
// pressed and released for uppercase letters or other characters that can be typed
// using Shift.
func (kw *KeyboardEventWriter) Type(ctx context.Context, s string) error {
	// Look up runes first so we can report an error before we start injecting events.
	type key struct {
		code    EventCode
		shifted bool
	}
	var keys []key
	for i, r := range []rune(s) {
		if code, ok := runeKeyCodes[r]; ok {
			keys = append(keys, key{code, false})
		} else if code, ok := shiftedRuneKeyCodes[r]; ok {
			keys = append(keys, key{code, true})
		} else {
			return errors.Errorf("unsupported rune %v at position %d", r, i)
		}
	}

	var firstErr error

	shifted := false
	for i, k := range keys {
		if k.shifted && !shifted {
			kw.sendKey(KEY_LEFTSHIFT, 1, &firstErr)
			shifted = true
		}

		kw.sendKey(k.code, 1, &firstErr)
		kw.sendKey(k.code, 0, &firstErr)

		if shifted && (i+1 == len(keys) || !keys[i+1].shifted) {
			kw.sendKey(KEY_LEFTSHIFT, 0, &firstErr)
			shifted = false
		}

		kw.sleepAfterType(ctx, &firstErr)
	}

	return firstErr
}

// Accel injects a sequence of key events simulating the accelerator (a.k.a. hotkey) described by s being typed.
// Accelerators are described as a sequence of '+'-separated, case-insensitive key characters or names.
// In addition to non-whitespace characters that are present on a QWERTY keyboard, the following key names may be used:
//	Modifiers:     "Ctrl", "Alt", "Search", "Shift"
//	Whitespace:    "Enter", "Space", "Tab", "Backspace"
//	Function keys: "F1", "F2", ..., "F12"
// "Shift" must be included for keys that are typed using Shift; for example, use "Ctrl+Shift+/" rather than "Ctrl+?".
func (kw *KeyboardEventWriter) Accel(ctx context.Context, s string) error {
	keys, err := parseAccel(s)
	if err != nil {
		return errors.Wrapf(err, "failed to parse %q", s)
	}
	if len(keys) == 0 {
		return errors.Errorf("no keys found in %q", s)
	}

	// Press the keys in forward order and then release them in reverse order.
	var firstErr error
	for i := 0; i < len(keys); i++ {
		kw.sendKey(keys[i], 1, &firstErr)
	}
	for i := len(keys) - 1; i >= 0; i-- {
		kw.sendKey(keys[i], 0, &firstErr)
	}
	kw.sleepAfterType(ctx, &firstErr)
	return firstErr
}

// sleepAfterType sleeps for short time. It is supposed to be called after key strokes.
// TODO(derat): Without sleeping between keystrokes, the omnibox seems to produce scrambled text.
// Figure out why. Presumably there's a bug in Chrome's input stack or the omnibox code.
func (kw *KeyboardEventWriter) sleepAfterType(ctx context.Context, firstErr *error) {
	if kw.fast {
		return
	}
	if *firstErr != nil {
		return
	}

	select {
	case <-time.After(50 * time.Millisecond):
	case <-ctx.Done():
		*firstErr = errors.Wrap(ctx.Err(), "timeout while typing")
	}
}

// NewWriter returns a new TouchTouchEvent instance.
func (mw *MultiTouchEventWriter) NewWriter() (*TouchEventWriter, error) {
	const defaultPressure = 60
	const defaultTouchMajor = 5
	const defaultTouchMinor = 5
	touchID := mw.nextTouchID
	mw.nextTouchID++
	return &TouchEventWriter{
		mw:          mw,
		touchID:     touchID,
		absPressure: defaultPressure,
		touchMajor:  defaultTouchMajor,
		touchMinor:  defaultTouchMinor,
		timestampUS: 0}, nil
}

// GetEventAbsInfo returns the Touchscreen's "Input abs information" both for
// x and y coordinates.
func (mw *MultiTouchEventWriter) GetEventAbsInfo() (EventAbsInfo, EventAbsInfo) {
	return mw.touchInfoX, mw.touchInfoY
}

// TouchEventWriter supports injecting touch events into a touchscreen device.
// Multi touch can be achieved by using multiple TouchEvent instances.
type TouchEventWriter struct {
	mw          *MultiTouchEventWriter
	touchID     int32
	absPressure int32
	touchMajor  int32
	touchMinor  int32
	timestampUS int32 // in microseconds
}

// EventAbsInfo corresponds to a input_absinfo struct.
// Taken from: include/uapi/linux/input.h
type EventAbsInfo struct {
	value      uint32
	minimum    uint32
	maximum    uint32
	fuzz       uint32
	flat       uint32
	resolution uint32
}

// GetMinimum returns the minimum value of the touchscreen for the given
// coordinate. In touchscreen units, and not pixels.
func (info *EventAbsInfo) GetMinimum() uint32 {
	return info.minimum
}

// GetMaximum returns the miximum value of the touchscreen for the given
// coordinate. In touchscreen units, and not pixels.
func (info *EventAbsInfo) GetMaximum() uint32 {
	return info.maximum
}

// evIOCGAbs returns an encoded Event-Ioctl-Get-Absolute value to be used for Ioclt().
// Similar to the EVIOCGABS found in include/uapi/linux/input.h
func evIOCGAbs(ev int) int {
	const sizeofEventAbsInfo = 0x24
	return IOR('E', 0x40+ev, sizeofEventAbsInfo)
}

// TouchAt generates a touch event at x and y touchscreen coordinates. Calling the same TouchEvent
// multiple times generates a continuos movement since the event has the same multitouch Id.
// x and y should be in touchscreen coordinates, not pixel coordiantes.
// See: https://www.kernel.org/doc/Documentation/input/multi-touch-protocol.txt
// See: https://www.kernel.org/doc/Documentation/input/event-codes.txt
func (tw *TouchEventWriter) TouchAt(x, y int32) error {

	if x < int32(tw.mw.touchInfoX.minimum) || x >= int32(tw.mw.touchInfoX.maximum) ||
		y < int32(tw.mw.touchInfoY.minimum) || y >= int32(tw.mw.touchInfoY.maximum) {
		return errors.Errorf("Cooridnates (x=%d, y=%d) outside the valid range.", x, y)
	}

	for _, e := range []struct {
		et  EventType
		ec  EventCode
		val int32
	}{
		{EV_ABS, ABS_MT_TRACKING_ID, tw.touchID},
		{EV_ABS, ABS_MT_POSITION_X, x},
		{EV_ABS, ABS_MT_POSITION_Y, y},
		{EV_ABS, ABS_MT_PRESSURE, tw.absPressure},
		{EV_ABS, ABS_MT_TOUCH_MAJOR, tw.touchMajor},
		{EV_ABS, ABS_MT_TOUCH_MINOR, tw.touchMinor},
		{EV_KEY, BTN_TOUCH, 1},
		{EV_ABS, ABS_X, x},
		{EV_ABS, ABS_Y, y},
		{EV_ABS, ABS_PRESSURE, tw.absPressure},
		{EV_MSC, MSC_TIMESTAMP, tw.timestampUS},
	} {
		if err := tw.mw.rw.Event(e.et, e.ec, e.val); err != nil {
			return err
		}
	}

	// In microseconds. According to dumps, it is safe to fake the timestamp
	// and increment it by 10000 each call. No problem if it wraps around
	// according to the spec.
	tw.timestampUS += 10000

	return tw.mw.rw.Sync()
}

// End sends the "End of life" touch event to the device. The TouchEvent can be
// reused. If so, it will generate a new event with the same old multitouch Id.
// Regarless of how TouchEventWriter is used, this method should be called after
// using it, possibly with the "defer" statement.
func (tw *TouchEventWriter) End() error {

	for _, e := range []struct {
		et  EventType
		ec  EventCode
		val int32
	}{
		{EV_ABS, ABS_MT_TRACKING_ID, -1},
		{EV_ABS, ABS_PRESSURE, 0},
		{EV_KEY, BTN_TOUCH, 0},
		{EV_MSC, MSC_TIMESTAMP, tw.timestampUS},
	} {
		if err := tw.mw.rw.Event(e.et, e.ec, e.val); err != nil {
			return err
		}
	}

	// Reset timestamp to 0 in case this event is reused.
	tw.timestampUS = 0

	return tw.mw.rw.Sync()
}

// SetAbsPressure sets the absolute pressure to be used starting from the
// next call to TouchAt(). "pressure", as defined in the spec, uses arbitrary
// units. According to tests "0" means no preassure, and "70" means 'normal'
// preassure, at least on Eve devices.
func (tw *TouchEventWriter) SetAbsPressure(pressure int32) {
	tw.absPressure = pressure
}
