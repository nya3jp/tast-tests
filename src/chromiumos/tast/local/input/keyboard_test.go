// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package input

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"chromiumos/tast/errors"
	"chromiumos/tast/testutil"
)

// testBuffer implements io.WriteCloser.
type testBuffer struct {
	buf       bytes.Buffer
	numWrites int
	err       error // error to return on writes
}

func (b *testBuffer) Write(p []byte) (int, error) {
	b.numWrites++
	if b.err != nil {
		return 0, b.err
	}
	return b.buf.Write(p)
}

func (b *testBuffer) Close() error { return nil }

// eventString returns a string representation of the passed-in event details
// as "[<time> <event-type> <event-code> <value>]".
func eventString(tv syscall.Timeval, et, ec uint16, val int32) string {
	sec := (time.Duration(tv.Nano()) * time.Nanosecond).Seconds()
	return fmt.Sprintf("[%0.3f 0x%x 0x%x %d]", sec, et, ec, val)
}

// readEvent reads a single event32 or event64 from r and returns the resulting eventString representation.
func readEvent(r io.Reader) (string, error) {
	switch intSize := unsafe.Sizeof(int(1)); intSize {
	case 4:
		ev := event32{}
		if err := binary.Read(r, binary.LittleEndian, &ev); err != nil {
			return "", err
		}
		return eventString(syscall.Timeval{Sec: int64(ev.Sec), Usec: int64(ev.Usec)}, ev.Type, ev.Code, ev.Val), nil
	case 8:
		ev := event64{}
		if err := binary.Read(r, binary.LittleEndian, &ev); err != nil {
			return "", err
		}
		return eventString(ev.Tv, ev.Type, ev.Code, ev.Val), nil
	default:
		return "", errors.Errorf("unexpected int size of %d byte(s)", intSize)
	}
}

// readAllEvents calls readEvent on r until EOF is reached and returns the resulting event strings.
// A partial set of events may be returned on error.
func readAllEvents(r io.Reader) ([]string, error) {
	var events []string
	for {
		s, err := readEvent(r)
		if err != nil {
			if err != io.EOF {
				return events, err
			}
			break
		}
		events = append(events, s)
	}
	return events, nil
}

func TestEventWriterSuccess(t *testing.T) {
	b := testBuffer{}
	now := time.Unix(1, 0)
	kw := KeyboardEventWriter{rw: &RawEventWriter{&b, func() time.Time { return now }}, fast: true}

	if err := kw.rw.Event(EV_KEY, KEY_A, 1); err != nil {
		t.Error("Writing key down failed: ", err)
	}
	if err := kw.rw.Sync(); err != nil {
		t.Error("Writing first sync failed: ", err)
	}
	if err := kw.rw.Event(EV_KEY, KEY_A, 0); err != nil {
		t.Error("Writing key up failed: ", err)
	}
	if err := kw.rw.Sync(); err != nil {
		t.Error("Writing first sync failed: ", err)
	}
	if err := kw.Close(); err != nil {
		t.Error("Close failed: ", err)
	}

	written, err := readAllEvents(bytes.NewReader(b.buf.Bytes()))
	if err != nil {
		t.Error("Failed to read events: ", err)
	}

	tv := syscall.NsecToTimeval(now.UnixNano())
	expected := []string{
		eventString(tv, uint16(EV_KEY), uint16(KEY_A), 1),
		eventString(tv, uint16(EV_SYN), uint16(SYN_REPORT), 0),
		eventString(tv, uint16(EV_KEY), uint16(KEY_A), 0),
		eventString(tv, uint16(EV_SYN), uint16(SYN_REPORT), 0),
	}
	if !reflect.DeepEqual(written, expected) {
		t.Errorf("Wrote %v; want %v", written, expected)
	}
}

func TestEventWriterWriteError(t *testing.T) {
	// Create a buffer that always returns an error on write.
	b := testBuffer{}
	b.err = errors.New("intentional error")
	kw := KeyboardEventWriter{rw: &RawEventWriter{&b, time.Now}, fast: true}
	defer kw.Close()

	if err := kw.rw.Event(EV_KEY, KEY_A, 1); err == nil {
		t.Error("Event didn't report expected error")
	}
}

func TestEventWriterOpenError(t *testing.T) {
	td := testutil.TempDir(t)
	defer os.RemoveAll(td)

	// When attempting to open a nonexistent device, an error should be reported.
	if rw, err := Device(context.Background(), filepath.Join(td, "bogus")); err == nil {
		t.Error("Device didn't report expected error for nonexistent device")
		rw.Close()
	}
}

func TestEventWriterType(t *testing.T) {
	b := testBuffer{}
	now := time.Unix(5, 0)
	kw := KeyboardEventWriter{rw: &RawEventWriter{&b, func() time.Time { return now }}, fast: true}

	const str = "AHa!"
	if err := kw.Type(context.Background(), str); err != nil {
		t.Fatalf("Type(%q) returned error: %v", str, err)
	}

	written, err := readAllEvents(bytes.NewReader(b.buf.Bytes()))
	if err != nil {
		t.Error("Failed to read events: ", err)
	}

	tv := syscall.NsecToTimeval(now.UnixNano())
	syn := eventString(tv, uint16(EV_SYN), uint16(SYN_REPORT), 0)
	expected := []string{
		eventString(tv, uint16(EV_KEY), uint16(KEY_LEFTSHIFT), 1), syn,
		eventString(tv, uint16(EV_KEY), uint16(KEY_A), 1), syn,
		eventString(tv, uint16(EV_KEY), uint16(KEY_A), 0), syn,
		eventString(tv, uint16(EV_KEY), uint16(KEY_H), 1), syn,
		eventString(tv, uint16(EV_KEY), uint16(KEY_H), 0), syn,
		eventString(tv, uint16(EV_KEY), uint16(KEY_LEFTSHIFT), 0), syn,
		eventString(tv, uint16(EV_KEY), uint16(KEY_A), 1), syn,
		eventString(tv, uint16(EV_KEY), uint16(KEY_A), 0), syn,
		eventString(tv, uint16(EV_KEY), uint16(KEY_LEFTSHIFT), 1), syn,
		eventString(tv, uint16(EV_KEY), uint16(KEY_1), 1), syn,
		eventString(tv, uint16(EV_KEY), uint16(KEY_1), 0), syn,
		eventString(tv, uint16(EV_KEY), uint16(KEY_LEFTSHIFT), 0), syn,
	}
	if !reflect.DeepEqual(written, expected) {
		t.Errorf("Wrote %v; want %v", written, expected)
	}
}

func TestEventWriterAccel(t *testing.T) {
	b := testBuffer{}
	now := time.Unix(5, 0)
	kw := KeyboardEventWriter{rw: &RawEventWriter{&b, func() time.Time { return now }}, fast: true}

	const accel = "Ctrl+Alt+T"
	if err := kw.Accel(context.Background(), accel); err != nil {
		t.Fatalf("Accel(%q) returned error: %v", accel, err)
	}

	written, err := readAllEvents(bytes.NewReader(b.buf.Bytes()))
	if err != nil {
		t.Error("Failed to read events: ", err)
	}

	tv := syscall.NsecToTimeval(now.UnixNano())
	syn := eventString(tv, uint16(EV_SYN), uint16(SYN_REPORT), 0)
	expected := []string{
		eventString(tv, uint16(EV_KEY), uint16(KEY_LEFTCTRL), 1), syn,
		eventString(tv, uint16(EV_KEY), uint16(KEY_LEFTALT), 1), syn,
		eventString(tv, uint16(EV_KEY), uint16(KEY_T), 1), syn,
		eventString(tv, uint16(EV_KEY), uint16(KEY_T), 0), syn,
		eventString(tv, uint16(EV_KEY), uint16(KEY_LEFTALT), 0), syn,
		eventString(tv, uint16(EV_KEY), uint16(KEY_LEFTCTRL), 0), syn,
	}
	if !reflect.DeepEqual(written, expected) {
		t.Errorf("Wrote %v; want %v", written, expected)
	}
}
