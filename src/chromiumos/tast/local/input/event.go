// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package input supports injecting input events via kernel devices.
package input

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"syscall"
	"time"
	"unsafe"

	"chromiumos/tast/testing"
)

//go:generate go run gen/gen_constants.go ../../../../../../../third_party/kernel/v4.14/include/uapi/linux/input-event-codes.h generated_constants.go
//go:generate go fmt generated_constants.go

// TODO(derat): Define some constants for event values, e.g. 0 for key up, 1 for key down, 2 for key repeat.

// EventWriter supports injecting input events into a device.
type EventWriter struct {
	w   io.WriteCloser // device
	err error          // first error encountered
}

// newWriter returns an EventWriter that will write to w.
// If err is non-nil, the writer will contain it and not attempt to perform any writes.
func newWriter(w io.WriteCloser, err error) *EventWriter {
	return &EventWriter{w: w, err: err}
}

// Keyboard returns an EventWriter to inject events into an arbitrary keyboard device.
func Keyboard(ctx context.Context) *EventWriter {
	infos, err := readDevices(procDevices)
	if err != nil {
		return newWriter(nil, fmt.Errorf("failed to read %v: %v", procDevices, err))
	}
	for _, info := range infos {
		if info.isKeyboard() {
			testing.ContextLogf(ctx, "Opening keyboard device %+v", info)
			return Device(ctx, info.path)
		}
	}
	return newWriter(nil, errors.New("didn't find keyboard device"))
}

// Device returns an EventWriter for injecting input events into the input event device at path.
// On error, an EventWriter will be returned, but Err will return an error if called and any writes will be no-ops.
func Device(ctx context.Context, path string) *EventWriter {
	f, err := os.OpenFile(path, os.O_WRONLY, 0600)
	return newWriter(f, err)
}

// Close closes the device.
// If an error was encountered while closing or during any previous operation, it will be returned.
func (ew *EventWriter) Close() error {
	ew.saveErr(ew.w.Close())
	return ew.err
}

// Err returns the first error that was encountered or nil if no error has been encountered.
func (ew *EventWriter) Err() error {
	return ew.err
}

// saveErr saves err to ew.err if ew.err is nil.
func (ew *EventWriter) saveErr(err error) {
	if ew.err != nil {
		ew.err = err
	}
}

// Event injects an event containing the supplied values into the device.
// This is a no-op if ew.Err is non-nil, i.e. an earlier error was encountered.
func (ew *EventWriter) Event(t time.Time, et EventType, ec EventCode, val int32) {
	if ew.err != nil {
		return
	}

	tv := syscall.NsecToTimeval(t.UnixNano())

	// input_event contains a timeval struct, which uses "long" for its members.
	// binary.Write wants explicitly-sized data, so we need to pass a different
	// struct depending on the system's int size.
	var ev interface{}
	intSize := unsafe.Sizeof(int(1))
	switch intSize {
	case 4:
		ev = &event32{int32(tv.Sec), int32(tv.Usec), uint16(et), uint16(ec), val}
	case 8:
		ev = &event64{tv, uint16(et), uint16(ec), val}
	default:
		ew.saveErr(fmt.Errorf("unexpected int size of %d byte(s)", intSize))
		return
	}

	// Little-endian is appropriate regardless of the system's underlying endianness.
	ew.saveErr(binary.Write(ew.w, binary.LittleEndian, ev))
}

// event32 corresponds to a 32-bit input_event struct.
type event32 struct {
	sec, usec int32
	et, ec    uint16
	val       int32
}

// event64 corresponds to a 64-bit input_event struct.
type event64 struct {
	tv     syscall.Timeval
	et, ec uint16
	val    int32
}

// Sync writes a synchronization event delineating a packet of input data occurring at a single point in time.
// It's shorthand for Event(t, EV_SYN, SYN_REPORT, 0).
func (ew *EventWriter) Sync(t time.Time) {
	ew.Event(t, EV_SYN, SYN_REPORT, 0)
}
