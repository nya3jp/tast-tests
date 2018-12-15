// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package input supports injecting input events via kernel devices.
package input

import (
	"C"
	"context"
	"encoding/binary"
	"io"
	"math/rand"
	"os"
	"syscall"
	"time"
	"unsafe"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

//go:generate go run gen/gen_constants.go ../../../../../../../third_party/kernel/v4.14/include/uapi/linux/input-event-codes.h generated_constants.go
//go:generate go fmt generated_constants.go

// EventWriter supports injecting input events into a device.
type EventWriter struct {
	w       io.WriteCloser // device
	nowFunc func() time.Time
	fast    bool // if true, do not sleep after type; useful for unit tests

	// Touch related info
	TouchInfoX inputAbsInfo
	TouchInfoY inputAbsInfo
}

// Keyboard returns an EventWriter to inject events into an arbitrary keyboard device.
func Keyboard(ctx context.Context) (*EventWriter, error) {
	infos, err := readDevices(procDevices)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read %v", procDevices)
	}
	for _, info := range infos {
		if info.isKeyboard() {
			testing.ContextLogf(ctx, "Opening keyboard device %+v", info)
			return Device(ctx, info.path, inputAbsInfo{}, inputAbsInfo{})
		}
	}
	return nil, errors.New("didn't find keyboard device")
}

// Touchscreen returns an EventWriter to inject events into an arbitrary touchscreen device.
func Touchscreen(ctx context.Context) (*EventWriter, error) {
	infos, err := readDevices(procDevices)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read %v", procDevices)
	}
	for _, info := range infos {
		if info.isTouchscreen() {
			testing.ContextLogf(ctx, "Opening touchscreen device %+v", info)

			// get min,max,resolution values
			fd, err := os.Open(info.path)
			if err != nil {
				return nil, err
			}
			defer fd.Close()

			var infoX, infoY inputAbsInfo
			if err := Ioctl(fd.Fd(), evIOCGAbs(0), unsafe.Pointer(&infoX)); err != nil {
				return nil, err
			}
			testing.ContextLogf(ctx, "EV_ABS, ABS_X: %+v", infoX)

			if err := Ioctl(fd.Fd(), evIOCGAbs(1), unsafe.Pointer(&infoY)); err != nil {
				return nil, err
			}
			testing.ContextLogf(ctx, "EV_ABS, ABS_Y: %+v", infoY)

			return Device(ctx, info.path, infoX, infoY)
		}
	}
	return nil, errors.New("didn't find touchscreen device")
}

// Device returns an EventWriter for injecting input events into the input event device at path.
func Device(ctx context.Context, path string, infoX inputAbsInfo, infoY inputAbsInfo) (*EventWriter, error) {
	f, err := os.OpenFile(path, os.O_WRONLY, 0600)
	if err != nil {
		return nil, err
	}
	return &EventWriter{f, time.Now, false, infoX, infoY}, nil
}

// Close closes the device.
func (ew *EventWriter) Close() error {
	return ew.w.Close()
}

// Event injects an event containing the supplied values into the device.
func (ew *EventWriter) Event(et EventType, ec EventCode, val int32) error {
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
func (ew *EventWriter) Sync() error {
	return ew.Event(EV_SYN, SYN_REPORT, 0)
}

// sendKey writes a EV_KEY event containing the specified code and value, followed by a EV_SYN event.
// If firstErr points at a non-nil error, no events are written.
// If an error is encountered, it is saved to the address pointed to by firstErr.
func (ew *EventWriter) sendKey(ec EventCode, val int32, firstErr *error) {
	if *firstErr == nil {
		*firstErr = ew.Event(EV_KEY, ec, val)
	}
	if *firstErr == nil {
		*firstErr = ew.Sync()
	}
}

// Type injects key events suitable for generating the string s.
// Only characters that can be typed using a QWERTY keyboard are supported,
// and the current keyboard layout must be QWERTY. The left Shift key is automatically
// pressed and released for uppercase letters or other characters that can be typed
// using Shift.
func (ew *EventWriter) Type(ctx context.Context, s string) error {
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
			ew.sendKey(KEY_LEFTSHIFT, 1, &firstErr)
			shifted = true
		}

		ew.sendKey(k.code, 1, &firstErr)
		ew.sendKey(k.code, 0, &firstErr)

		if shifted && (i+1 == len(keys) || !keys[i+1].shifted) {
			ew.sendKey(KEY_LEFTSHIFT, 0, &firstErr)
			shifted = false
		}

		ew.sleepAfterType(ctx, &firstErr)
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
func (ew *EventWriter) Accel(ctx context.Context, s string) error {
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
		ew.sendKey(keys[i], 1, &firstErr)
	}
	for i := len(keys) - 1; i >= 0; i-- {
		ew.sendKey(keys[i], 0, &firstErr)
	}
	ew.sleepAfterType(ctx, &firstErr)
	return firstErr
}

// sleepAfterType sleeps for short time. It is supposed to be called after key strokes.
// TODO(derat): Without sleeping between keystrokes, the omnibox seems to produce scrambled text.
// Figure out why. Presumably there's a bug in Chrome's input stack or the omnibox code.
func (ew *EventWriter) sleepAfterType(ctx context.Context, firstErr *error) {
	if ew.fast {
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

// TouchEvent returns a new TouchEvent instance.
func (ew *EventWriter) TouchEvent() (*TouchEvent, error) {
	const defaultPressure = 60
	const defaultTouchMajor = 5
	const defaultTouchMinor = 5
	return &TouchEvent{defaultPressure, defaultTouchMajor, defaultTouchMinor, rand.Int31(), 0, ew}, nil
}

// TouchEvent supports injecting touch events into a touchscreen device.
// Multi touch can be achieved by using multiple TouchEvent instances.
type TouchEvent struct {
	AbsPressure  int32
	TouchMajor   int32
	TouchMinor   int32
	MultiTouchID int32
	timestamp    int32
	ew           *EventWriter
}

// inputAbsInfo corresponds to a input_absinfo struct.
// Taken from: include/uapi/linux/input.h
type inputAbsInfo struct {
	Value      uint32
	Minimum    uint32
	Maximum    uint32
	Fuzz       uint32
	Flat       uint32
	Resolution uint32
}

// evIOCGAbs returns an enconded Event-Ioctl-Get-Absolute value to be used for Ioclt().
// Similar to the EVIOCGABS found in include/uapi/linux/input.h
func evIOCGAbs(ev int) int {
	const sizeofInputAbsInfo = 0x24
	return IOR('E', 0x40+ev, sizeofInputAbsInfo)
}

// MoveTo generates a touch event at x and y touchscreen coordinates. Moving the same TouchEvent
// multiple times generates a continuos movement since the event has the same multitouch Id.
// x and y should be in touchscreen coordinates, not pixel coordiantes.
// See: https://www.kernel.org/doc/Documentation/input/multi-touch-protocol.txt
func (te *TouchEvent) MoveTo(x int32, y int32) error {

	if x < int32(te.ew.TouchInfoX.Minimum) || x >= int32(te.ew.TouchInfoX.Maximum) ||
		y < int32(te.ew.TouchInfoY.Minimum) || y >= int32(te.ew.TouchInfoY.Maximum) {
		return errors.Errorf("Cooridnates (x=%d, y=%d) outside the valid range.", x, y)
	}

	var err error
	err = te.ew.Event(EV_ABS, ABS_MT_TRACKING_ID, te.MultiTouchID)

	if err == nil {
		err = te.ew.Event(EV_ABS, ABS_MT_POSITION_X, x)
	}

	if err == nil {
		err = te.ew.Event(EV_ABS, ABS_MT_POSITION_Y, y)
	}

	if err == nil {
		err = te.ew.Event(EV_ABS, ABS_MT_PRESSURE, te.AbsPressure)
	}

	if err == nil {
		err = te.ew.Event(EV_ABS, ABS_MT_TOUCH_MAJOR, te.TouchMajor)
	}

	if err == nil {
		err = te.ew.Event(EV_ABS, ABS_MT_TOUCH_MINOR, te.TouchMinor)
	}

	if err == nil {
		err = te.ew.Event(EV_KEY, BTN_TOUCH, 1)
	}

	if err == nil {
		err = te.ew.Event(EV_ABS, ABS_X, x)
	}

	if err == nil {
		err = te.ew.Event(EV_ABS, ABS_Y, y)
	}

	if err == nil {
		err = te.ew.Event(EV_ABS, ABS_PRESSURE, te.AbsPressure)
	}

	if err == nil {
		err = te.ew.Event(EV_MSC, MSC_TIMESTAMP, te.timestamp*10000)
		te.timestamp++
	}

	if err == nil {
		err = te.ew.Sync()
	}
	return err
}

// End sends the "End of life" touch event to the device. The TouchEvent can
// be reused. If so, it will generate a new event with the same old multitouch Id.
func (te *TouchEvent) End() error {

	var err error
	err = te.ew.Event(EV_ABS, ABS_MT_TRACKING_ID, -1)

	if err == nil {
		err = te.ew.Event(EV_ABS, ABS_PRESSURE, 0)
	}

	if err == nil {
		err = te.ew.Event(EV_KEY, BTN_TOUCH, 0)
	}

	if err == nil {
		err = te.ew.Event(EV_MSC, MSC_TIMESTAMP, te.timestamp*10000)
		// Reset timestamp to 0 in case this event is reused.
		te.timestamp = 0
	}

	if err == nil {
		err = te.ew.Sync()
	}

	return err
}
