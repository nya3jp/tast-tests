// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package input

import (
	"bytes"
	"fmt"
	"reflect"
	"syscall"
	"testing"
	"time"
)

func TestEventWriterTouch(t *testing.T) {
	const (
		x       = 13
		y       = 17
		p       = 62
		tMajor  = 4
		tMinor  = 3
		touchID = 12345
	)

	b := testBuffer{}
	now := time.Unix(5, 0)
	mw := TouchscreenEventWriter{
		rw:            &RawEventWriter{&b, func() time.Time { return now }},
		nextTouchID:   touchID,
		width:         1000,
		height:        1000,
		maxTouchSlot:  9,
		maxTrackingID: 65536,
		maxPressure:   128,
	}

	tw, err := mw.NewSingleTouchWriter()
	if err != nil {
		t.Fatalf("TouchEvent returned error: %v", err)
	}
	defer tw.Close()

	tw.touches[0].absPressure = p
	tw.touches[0].touchMajor = tMajor
	tw.touches[0].touchMinor = tMinor
	tw.touches[0].touchID = touchID

	tw.Move(x, y)
	tw.End()

	written, err := readAllEvents(bytes.NewReader(b.buf.Bytes()))
	if err != nil {
		t.Error("Failed to read events: ", err)
	}

	tv := syscall.NsecToTimeval(now.UnixNano())
	syn := eventString(tv, uint16(EV_SYN), uint16(SYN_REPORT), 0)
	expected := []string{
		eventString(tv, uint16(EV_ABS), uint16(ABS_MT_SLOT), 0),
		eventString(tv, uint16(EV_ABS), uint16(ABS_MT_TRACKING_ID), touchID),
		eventString(tv, uint16(EV_ABS), uint16(ABS_MT_POSITION_X), x),
		eventString(tv, uint16(EV_ABS), uint16(ABS_MT_POSITION_Y), y),
		eventString(tv, uint16(EV_ABS), uint16(ABS_MT_PRESSURE), p),
		eventString(tv, uint16(EV_ABS), uint16(ABS_MT_TOUCH_MAJOR), tMajor),
		eventString(tv, uint16(EV_ABS), uint16(ABS_MT_TOUCH_MINOR), tMinor),
		eventString(tv, uint16(EV_KEY), uint16(BTN_TOUCH), 1),
		eventString(tv, uint16(EV_ABS), uint16(ABS_X), x),
		eventString(tv, uint16(EV_ABS), uint16(ABS_Y), y),
		eventString(tv, uint16(EV_ABS), uint16(ABS_PRESSURE), p),
		syn,
		eventString(tv, uint16(EV_ABS), uint16(ABS_MT_SLOT), 0),
		eventString(tv, uint16(EV_ABS), uint16(ABS_MT_TRACKING_ID), -1),
		eventString(tv, uint16(EV_ABS), uint16(ABS_PRESSURE), 0),
		eventString(tv, uint16(EV_KEY), uint16(BTN_TOUCH), 0),
		syn,
	}
	if !reflect.DeepEqual(written, expected) {
		t.Errorf("Wrote %v; want %v", written, expected)
	}
}

func TestRotate(t *testing.T) {
	const (
		inX     = 13
		inY     = 17
		p       = 62
		tMajor  = 4
		tMinor  = 3
		touchID = 12345
		width   = 2000
		height  = 1000
	)

	for _, c := range []struct {
		rotation  int
		expX      int32
		expY      int32
		transpose bool
	}{
		{0, inX, inY, false},
		{90, width - inY, inX, true},
		{180, width - inX, height - inY, false},
		{270, inY, height - inX, true},
		{-90, inY, height - inX, true},
		{360, inX, inY, false},
	} {
		t.Run(fmt.Sprintf("%d", c.rotation), func(t *testing.T) {
			b := testBuffer{}
			now := time.Unix(5, 0)
			mw := TouchscreenEventWriter{
				rw:            &RawEventWriter{&b, func() time.Time { return now }},
				nextTouchID:   touchID,
				width:         width,
				height:        height,
				maxTouchSlot:  9,
				maxTrackingID: 65536,
				maxPressure:   128,
			}

			tw, err := mw.NewSingleTouchWriter()
			if err != nil {
				t.Fatal("Failed to create a touch event writer: ", err)
			}
			tw.touches[0].absPressure = p
			tw.touches[0].touchMajor = tMajor
			tw.touches[0].touchMinor = tMinor

			if err := mw.Rotate(c.rotation); err != nil {
				t.Fatalf("Failed to set rotation to %d: %v", c.rotation, err)
			}
			if c.transpose {
				if mw.Width() != height || mw.Height() != width {
					t.Errorf("Width and Height should return the rotated size: got %d,%d vs want %d,%d", mw.Width(), mw.Height(), height, width)
				}
			} else {
				if mw.Width() != width || mw.Height() != height {
					t.Errorf("Width and Height should remain same: got %d,%d want %d,%d", mw.Width(), mw.Height(), width, height)
				}
			}

			tw.Move(inX, inY)
			written, err := readAllEvents(bytes.NewReader(b.buf.Bytes()))
			if err != nil {
				t.Fatal("Failed to read events: ", err)
			}

			tv := syscall.NsecToTimeval(now.UnixNano())
			syn := eventString(tv, uint16(EV_SYN), uint16(SYN_REPORT), 0)
			expected := []string{
				eventString(tv, uint16(EV_ABS), uint16(ABS_MT_SLOT), 0),
				eventString(tv, uint16(EV_ABS), uint16(ABS_MT_TRACKING_ID), touchID),
				eventString(tv, uint16(EV_ABS), uint16(ABS_MT_POSITION_X), c.expX),
				eventString(tv, uint16(EV_ABS), uint16(ABS_MT_POSITION_Y), c.expY),
				eventString(tv, uint16(EV_ABS), uint16(ABS_MT_PRESSURE), p),
				eventString(tv, uint16(EV_ABS), uint16(ABS_MT_TOUCH_MAJOR), tMajor),
				eventString(tv, uint16(EV_ABS), uint16(ABS_MT_TOUCH_MINOR), tMinor),
				eventString(tv, uint16(EV_KEY), uint16(BTN_TOUCH), 1),
				eventString(tv, uint16(EV_ABS), uint16(ABS_X), c.expX),
				eventString(tv, uint16(EV_ABS), uint16(ABS_Y), c.expY),
				eventString(tv, uint16(EV_ABS), uint16(ABS_PRESSURE), p),
				syn,
			}
			if !reflect.DeepEqual(written, expected) {
				t.Errorf("Wrote %v; want %v", written, expected)
			}
		})
	}
}

func TestRotateErr(t *testing.T) {
	b := testBuffer{}
	now := time.Unix(5, 0)
	mw := TouchscreenEventWriter{
		rw:            &RawEventWriter{&b, func() time.Time { return now }},
		width:         2000,
		height:        1000,
		maxTouchSlot:  9,
		maxTrackingID: 65536,
		maxPressure:   128,
	}

	beforeRotation := mw.rotation
	if err := mw.Rotate(127); err == nil {
		t.Error("Failed to succeed, expected an error")
	}
	if beforeRotation != mw.rotation {
		t.Errorf("Unexpected update of rotation for failed Rotate: %d vs %d", beforeRotation, mw.rotation)
	}
}
