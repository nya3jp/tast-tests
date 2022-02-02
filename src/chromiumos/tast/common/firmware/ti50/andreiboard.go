// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ti50

import (
	"context"
	"os"
	"path/filepath"
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

// Open opens the console port.
func (a *Andreiboard) Open(ctx context.Context) error {
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

// IsOpen returns true iff the port is open.
func (a *Andreiboard) IsOpen() bool {
	return a.port != nil
}

// Close closes the console port.
func (a *Andreiboard) Close(ctx context.Context) error {
	if a.port != nil {
		err := a.port.Close(ctx)
		if err != nil {
			return err
		}
		a.port = nil
	}
	return nil
}

func appendToLogFile(ctx context.Context, buf []byte) error {
	dir, ok := testing.ContextOutDir(ctx)
	if !ok {
		return errors.New("failed to get directory for saving files")
	}
	f, err := os.OpenFile(filepath.Join(dir, "andreiboard.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	if _, err := f.Write(buf); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return nil
}

// ReadSerialSubmatch reads from the serial port until regex is matched.
func (a *Andreiboard) ReadSerialSubmatch(ctx context.Context, re *regexp.Regexp) (output [][]byte, err error) {
	if err := a.Open(ctx); err != nil {
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
			a.targetBufferUnreadLen = copy(a.targetBufferUnread, buf)
			return nil, errors.Errorf("buffer is full (wanted %s)", re)
		}
		current, err := a.port.Read(ctx, buf[total:])
		if current > 0 {
			if err := appendToLogFile(ctx, buf[total:total+current]); err != nil {
				testing.ContextLog(ctx, "Log file error: ", err)
			}
		}
		total += current
		if err != nil {
			a.targetBufferUnreadLen = copy(a.targetBufferUnread, buf[:total])
			return nil, errors.Wrapf(err, "port read error (wanted %s)", re)
		}
		if current == 0 {
			break
		}
	}

	a.targetBufferUnreadLen = copy(a.targetBufferUnread, buf[:total])
	return nil, errors.New("failed to find match")
}

// WriteSerial writes to the serial port.
func (a *Andreiboard) WriteSerial(ctx context.Context, b []byte) error {
	if err := a.Open(ctx); err != nil {
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
