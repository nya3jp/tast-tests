// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ti50

import (
	"context"
	"regexp"

	"chromiumos/tast/common/firmware/serial"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// Andreiboard contains common implementations boards.
type Andreiboard struct {
	targetBufferUnread    []byte
	targetBufferUnreadLen int
	portOpener            serial.PortOpener
	port                  serial.Port
	spiflash              string
}

// NewAndreiboard creates a new port, bufMax should be set to the max
// number of bytes read that are not yet matched by ReadSerialSubmatch.
func NewAndreiboard(bufMax int, portOpener serial.PortOpener, spiFlash string) *Andreiboard {
	return &Andreiboard{targetBufferUnread: make([]byte, bufMax), portOpener: portOpener, spiflash: spiFlash}
}

// GetSpiFlash gets the path to previously set spiflash.
func (a *Andreiboard) GetSpiFlash() string {
	return a.spiflash
}

// openPort opens the port upon first use.
func (a *Andreiboard) openPort(ctx context.Context) error {
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

// Close the port.
func (a *Andreiboard) Close(ctx context.Context) error {
	if a.port != nil {
		err := a.port.Close(ctx)
		if err != nil {
			a.port = nil
		}
		return err
	}
	return nil
}

// ReadSerialSubmatch reads from the serial port until regex is matched.
func (a *Andreiboard) ReadSerialSubmatch(ctx context.Context, re *regexp.Regexp) (output [][]byte, err error) {
	if err := a.openPort(ctx); err != nil {
		return nil, errors.Wrap(err, "port open error")
	}

	buf := make([]byte, len(a.targetBufferUnread))
	total := copy(buf, a.targetBufferUnread[:a.targetBufferUnreadLen])
	for {
		indices := re.FindSubmatchIndex(buf[:total])
		if indices != nil {
			a.targetBufferUnreadLen = copy(a.targetBufferUnread, buf[indices[1]:total])
			return re.FindSubmatch(buf[:total]), nil
		}
		if total == len(a.targetBufferUnread) {
			testing.ContextLog(ctx, "Buffer full, contents:")
			testing.ContextLog(ctx, string(buf))
			a.targetBufferUnreadLen = copy(a.targetBufferUnread, buf)
			return nil, errors.New("buffer is full")
		}
		current, err := a.port.Read(ctx, buf[total:])
		total += current
		if err != nil {
			testing.ContextLog(ctx, "Read error, buffer contents:")
			testing.ContextLog(ctx, string(buf[:total]))
			a.targetBufferUnreadLen = copy(a.targetBufferUnread, buf[:total])
			return nil, errors.Wrap(err, "port read error")
		}
		if current == 0 {
			break
		}
	}

	testing.ContextLog(ctx, "Count not find match, buffer contents:")
	testing.ContextLog(ctx, string(buf[:total]))
	a.targetBufferUnreadLen = copy(a.targetBufferUnread, buf[:total])
	return nil, errors.New("failed to find match")
}

// WriteSerial writes to the serial port.
func (a *Andreiboard) WriteSerial(ctx context.Context, b []byte) error {
	if err := a.openPort(ctx); err != nil {
		return err
	}
	n, err := a.port.Write(ctx, b)

	if err != nil {
		return err
	}

	if n != len(b) {
		return errors.Errorf("not all bytes written, got %d, want %d", n, len(b))
	}
	return nil
}

// FlushSerial flushes un-read/written chars.
func (a *Andreiboard) FlushSerial(ctx context.Context) error {
	a.targetBufferUnreadLen = 0
	if a.port != nil {
		err := a.port.Flush(ctx)
		if err != nil {
			return err
		}
	}
	return nil
}
