// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ti50

import (
	"context"
	"regexp"
	"testing"

	"github.com/golang/mock/gomock"

	"chromiumos/tast/common/firmware/serial/mocks"
	"chromiumos/tast/errors"
)

func createDut(ctrl *gomock.Controller, bufLen int) (*Andreiboard, *mocks.MockPort) {
	p := mocks.NewMockPort(ctrl)

	dut := Andreiboard{
		targetBufferUnread: make([]byte, bufLen),
		port:               p,
	}

	dut.port = p
	return &dut, p
}

func TestAndreiboard(t *testing.T) {
	ctx := context.Background()
	expectRead := func(port *mocks.MockPort, inLen int, fill []byte) *gomock.Call {
		err := error(nil)
		if fill == nil {
			err = errors.New("EOF")
		}
		return port.EXPECT().Read(ctx, gomock.Any()).Return(len(fill), err).Do(func(ctx context.Context, b []byte) {
			if len(b) != inLen {
				t.Fatalf("Read buffer len, want %d, got %d", inLen, len(b))
			}
			copy(b, fill)
		})
	}

	checkMatch := func(wErr error, wMatch ...string) func([][]byte, error) {
		return func(gMatch [][]byte, gErr error) {
			t.Logf("Checking match %v, err %v", wMatch, wErr)
			if gErr != nil && wErr != nil {
				if gErr.Error() != wErr.Error() {
					t.Fatalf("Unexpected error, want %s, got %s", wErr, gErr)
				}
			} else if gErr != wErr {
				t.Fatalf("Unexpected error, want %v, got %v", wErr, gErr)
			}
			if wErr != nil {
				if gMatch != nil {
					t.Fatalf("Unexpected match, want nil, got %v", gMatch)
				} else {
					return
				}
			}

			for i, s := range wMatch {
				if i >= len(gMatch) {
					break
				}
				if string(gMatch[i]) != s {
					t.Fatalf("Capture group %d, want %s, got %s", i, s, string(gMatch[i]))
				}
			}

			if len(wMatch) != len(gMatch) {
				t.Fatalf("Capture group length, want %d, got %d", len(wMatch), len(gMatch))
			}
			t.Log("Check passed")
		}
	}

	ctrl := gomock.NewController(t)

	var dut *Andreiboard
	var port *mocks.MockPort

	t.Log("Read error should result in same error")
	dut, port = createDut(ctrl, 5)
	expectRead(port, 5, nil)
	checkMatch(errors.New("port read error: EOF"), "")(dut.ReadSerialSubmatch(ctx, regexp.MustCompile("abc")))

	t.Log("Matched string should be returned")
	dut, port = createDut(ctrl, 5)
	expectRead(port, 5, []byte("abc"))
	checkMatch(nil, "abc")(dut.ReadSerialSubmatch(ctx, regexp.MustCompile("abc")))

	t.Log("Matched string should be cleared from unread buffer")
	dut, port = createDut(ctrl, 5)
	gomock.InOrder(
		expectRead(port, 5, []byte("abc")),
		expectRead(port, 5, []byte("de")),
		expectRead(port, 3, []byte("")),
	)
	checkMatch(nil, "ab")(dut.ReadSerialSubmatch(ctx, regexp.MustCompile("ab")))
	checkMatch(nil, "c")(dut.ReadSerialSubmatch(ctx, regexp.MustCompile("c")))
	checkMatch(errors.New("failed to find match"), "")(dut.ReadSerialSubmatch(ctx, regexp.MustCompile("c")))

	t.Log("Unread buffer should be preserved if a match fails")
	dut, port = createDut(ctrl, 5)
	gomock.InOrder(
		expectRead(port, 5, []byte("ab")),
		expectRead(port, 3, []byte("")),
	)
	checkMatch(errors.New("failed to find match"), "")(dut.ReadSerialSubmatch(ctx, regexp.MustCompile("c")))
	checkMatch(nil, "ab")(dut.ReadSerialSubmatch(ctx, regexp.MustCompile("ab")))

	t.Log("Buffer full condition should result in error")
	dut, port = createDut(ctrl, 5)
	gomock.InOrder(
		expectRead(port, 5, []byte("abcde")),
	)
	checkMatch(errors.New("buffer is full"), "")(dut.ReadSerialSubmatch(ctx, regexp.MustCompile("f")))

	t.Log("Should be able to match after beginning of string")
	dut, port = createDut(ctrl, 5)
	gomock.InOrder(
		expectRead(port, 5, []byte("abcde")),
	)
	checkMatch(nil, "cde")(dut.ReadSerialSubmatch(ctx, regexp.MustCompile("cde")))

	t.Log("Capture groups should work")
	dut, port = createDut(ctrl, 5)
	gomock.InOrder(
		expectRead(port, 5, []byte("abcde")),
	)
	checkMatch(nil, "abcde", "ab", "cde")(dut.ReadSerialSubmatch(ctx, regexp.MustCompile("(..)(...)")))

	t.Log("Multiline match should work")
	dut, port = createDut(ctrl, 5)
	gomock.InOrder(
		expectRead(port, 5, []byte("ab\ncd")),
	)
	checkMatch(nil, "ab\ncd", "b\ncd")(dut.ReadSerialSubmatch(ctx, regexp.MustCompile("(?s)a(.*)")))

	t.Log("Write should work")
	dut, port = createDut(ctrl, 5)
	port.EXPECT().Write(ctx, []byte("abc")).Return(3, nil)
	err := dut.WriteSerial(ctx, []byte("abc"))
	if err != nil {
		t.Fatal("Write error: ", err)
	}
}
