// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package input

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"syscall"
	"testing"
	"time"
	"unsafe"

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
		return "", fmt.Errorf("unexpected int size of %d byte(s)", intSize)
	}
}

func TestEventWriterSuccess(t *testing.T) {
	b := testBuffer{}
	ew := newWriter(&b, nil)
	now := time.Unix(1, 0)
	ew.Event(now, EV_KEY, KEY_A, 1)
	ew.Sync(now)
	ew.Event(now, EV_KEY, KEY_A, 0)
	ew.Sync(now)
	if err := ew.Close(); err != nil {
		t.Error("EventWriter reported error: ", err)
	}

	r := bytes.NewReader(b.buf.Bytes())
	var written []string
	for {
		s, err := readEvent(r)
		if err != nil {
			if err != io.EOF {
				t.Error("Failed to read event: ", err)
			}
			break
		}
		written = append(written, s)
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
	ew := newWriter(&b, nil)
	now := time.Unix(1, 0)

	// After the first write fails, Err should report an error.
	ew.Event(now, EV_KEY, KEY_A, 1)
	if err := ew.Err(); err == nil {
		t.Error("Err didn't report expected error")
	}

	// No more writes should be performed.
	ew.Sync(now)
	ew.Event(now, EV_KEY, KEY_A, 0)
	if err := ew.Close(); err == nil {
		t.Error("Close didn't report expected error")
	}
	if b.numWrites != 1 {
		t.Errorf("EventWriter performed %d writes; expected 1", b.numWrites)
	}
}

func TestEventWriterOpenError(t *testing.T) {
	td := testutil.TempDir(t)
	defer os.RemoveAll(td)

	// After attempting to open a nonexistent device, the EventWriter should report an error.
	ew := Device(context.Background(), filepath.Join(td, "bogus"))
	if err := ew.Err(); err == nil {
		t.Error("Err didn't report expected error")
	}
	ew.Event(time.Now(), EV_KEY, KEY_A, 1)
	if err := ew.Close(); err == nil {
		t.Error("Close didn't report expected error")
	}
}
