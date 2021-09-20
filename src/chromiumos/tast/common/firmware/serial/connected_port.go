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
