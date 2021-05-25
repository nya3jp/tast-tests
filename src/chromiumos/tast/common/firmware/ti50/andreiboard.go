// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ti50

import (
	"context"
	"fmt"
	"regexp"

	"chromiumos/tast/common/firmware/serial"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

type AndreiBoard struct {
	targetBufferUnread    []byte
	targetBufferUnreadLen int
	portOpener            serial.PortOpener
	port                  serial.Port
}

func NewAndreiBoard(bufMax int, portOpener serial.PortOpener) *AndreiBoard {
	return &AndreiBoard{targetBufferUnread: make([]byte, bufMax), portOpener: portOpener}
}

// Open UD target port.
func (a *AndreiBoard) openPort(ctx context.Context) error {
	if a.port != nil {
		return nil
	}
	p, err := a.portOpener.OpenPort(ctx)
	if err != nil {
		return err
	}
	a.port = p
	return nil
}

// Close and cleanup.
func (a *AndreiBoard) Close(ctx context.Context) error {
	if a.port != nil {
		return a.port.Close(ctx)
	}
	return nil
}

// Read output from console port until regex is matched.
func (a *AndreiBoard) ReadSerialSubmatch(ctx context.Context, re *regexp.Regexp) (output [][]byte, err error) {
	if err := a.openPort(ctx); err != nil {
		return nil, err
	}

	buf := make([]byte, len(a.targetBufferUnread))
	total := copy(buf, a.targetBufferUnread[:a.targetBufferUnreadLen])
	i := 0
	defer func() {
		if ctx == nil {
			return
		}
	    testing.ContextLogf(ctx, "Looped %d times.", i)
	}()
	for {
		indices := re.FindSubmatchIndex(buf[:total])
		if indices != nil {
			a.targetBufferUnreadLen = copy(a.targetBufferUnread, buf[indices[1]:total])
			return re.FindSubmatch(buf[:total]), nil
		}
		if total == len(a.targetBufferUnread) {
			return nil, errors.New("Buffer is full")
		}
		current, err := a.port.Read(ctx, buf[total:])
		total += current
		if err != nil {
			return nil, err
		}
		if current == 0 {
			break
		}
		i++
	}

	a.targetBufferUnreadLen = copy(a.targetBufferUnread, buf[:total])
	return nil, errors.New("Could not find match")
}

// Write to console port.
func (a *AndreiBoard) WriteSerial(ctx context.Context, b []byte) error {
	if err := a.openPort(ctx); err != nil {
		return err
	}
	n, err := a.port.Write(ctx, b)

	if err != nil {
		return err
	}

	if n != len(b) {
		return errors.New(fmt.Sprintf("Not all bytes written, got %d, wait %d", n, len(b)))
	}
	return nil
}

// Flush un-read/written chars from console port.
func (a *AndreiBoard) FlushSerial(ctx context.Context) error {
	a.targetBufferUnreadLen = 0
	if a.port != nil {
		err := a.port.Flush(ctx)
		if err != nil {
			return err
		}
	}
	return nil
}
