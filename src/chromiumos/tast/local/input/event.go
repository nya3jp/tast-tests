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

// TODO(derat): Define some constants for event values, e.g. 0 for key up, 1 for key down, 2 for key repeat.

// EventWriter supports injecting input events into a device.
//
// TODO(derat): Add a method for typing accelerators.
type EventWriter struct {
	w       io.WriteCloser // device
	nowFunc func() time.Time
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
			return Device(ctx, info.path)
		}
	}
	return nil, errors.New("didn't find keyboard device")
}

// Device returns an EventWriter for injecting input events into the input event device at path.
func Device(ctx context.Context, path string) (*EventWriter, error) {
	f, err := os.OpenFile(path, os.O_WRONLY, 0600)
	if err != nil {
		return nil, err
	}
	return &EventWriter{f, time.Now}, nil
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

// Type injects key events suitable for generating the string s.
// Only characters that can be typed using a QWERTY keyboard are supported,
// and the current keyboard layout must be QWERTY. The left Shift key is automatically
// pressed and released for uppercase letters or other characters that can be typed
// using Shift.
func (ew *EventWriter) Type(s string) error {
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

	// To simplify error-checking, save the first error we see and perform no-op writes afterward.
	var firstErr error
	sendKey := func(et EventType, ec EventCode, val int32) {
		if firstErr == nil {
			firstErr = ew.Event(et, ec, val)
		}
		if firstErr == nil {
			firstErr = ew.Sync()
		}
	}

	shifted := false
	for i, k := range keys {
		if k.shifted && !shifted {
			sendKey(EV_KEY, KEY_LEFTSHIFT, 1)
			shifted = true
		}

		sendKey(EV_KEY, k.code, 1)
		sendKey(EV_KEY, k.code, 0)

		if shifted && (i+1 == len(keys) || !keys[i+1].shifted) {
			sendKey(EV_KEY, KEY_LEFTSHIFT, 0)
			shifted = false
		}
	}

	return firstErr
}
