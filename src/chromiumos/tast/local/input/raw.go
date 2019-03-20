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
)

// RawEventWriter supports injecting raw input events into a device.
type RawEventWriter struct {
	w       io.WriteCloser // device
	nowFunc func() time.Time
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
