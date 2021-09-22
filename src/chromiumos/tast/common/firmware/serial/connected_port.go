// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package serial

import (
	"context"

	"github.com/tarm/serial"

	"chromiumos/tast/errors"
)

// ConnectedPort implements Port, delegates to tarm/serial.
type ConnectedPort struct {
	port *serial.Port
}

// Read reads bytes into provided buffer, returns number of bytes read.
// Bytes already written to the port shall be moved into buf, up to its size.
// In blocking mode (port opened without ReadTimeout), it blocks until at least
// one byte is read.  In non-blocking mode, it may return an error with zero
// bytes read if ReadTimeout is exceeded, however this is not guaranteed on all
// platforms and so a context deadline exceeded error may be returned even
// though the deadline is greater than the ReadTimeout.
func (p *ConnectedPort) Read(ctx context.Context, buf []byte) (n int, err error) {
	done := make(chan struct{}, 1)
	go func() {
		if p.port == nil {
			err = errors.New("port already closed")
		} else {
			n, err = p.port.Read(buf)
		}
		done <- struct{}{}
	}()
	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	case <-done:
		return n, err
	}
}

// Write writes bytes from provided buffer, returns number of bytes written.
// It returns a non-nil error when n != len(b), nil otherwise.
func (p *ConnectedPort) Write(ctx context.Context, buf []byte) (n int, err error) {
	done := make(chan struct{}, 1)
	go func() {
		if p.port == nil {
			err = errors.New("port already closed")
		} else {
			n, err = p.port.Write(buf)
		}
		done <- struct{}{}
	}()
	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	case <-done:
		return n, err
	}
}

// Flush flushes un-read/written content from the port.
func (p *ConnectedPort) Flush(ctx context.Context) error {
	var err error
	done := make(chan struct{}, 1)
	go func() {
		if p.port == nil {
			err = errors.New("port already closed")
		} else {
			err = p.port.Flush()
		}
		done <- struct{}{}
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-done:
		return err
	}
}

// Close closes the port.
func (p *ConnectedPort) Close(ctx context.Context) error {
	var err error
	done := make(chan struct{}, 1)
	go func() {
		if p.port == nil {
			err = nil
			done <- struct{}{}
			return
		}
		err = p.port.Close()
		if err == nil {
			p.port = nil
		}
		done <- struct{}{}
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-done:
		return err
	}
}
