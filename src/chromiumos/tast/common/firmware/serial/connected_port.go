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
	if p.port == nil {
		return 0, errors.New("port already closed")
	}
	done := make(chan struct{})
	buf1 := make([]byte, len(buf))
	go func() {
		// Use buf1 to ensure buf is not modified after return in case of timeout.
		n, err = p.port.Read(buf1)
		done <- struct{}{}
	}()
	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	case <-done:
		if err == nil {
			copy(buf, buf1[:n])
		}
		return n, err
	}
}

// Write writes bytes from provided buffer, returns number of bytes written.
func (p *ConnectedPort) Write(ctx context.Context, buf []byte) (n int, err error) {
	if p.port == nil {
		return 0, errors.New("port already closed")
	}
	done := make(chan struct{})
	// Use buf1 to ensure deterministic write in case of timeout.
	buf1 := make([]byte, len(buf))
	copy(buf1, buf)
	go func() {
		n, err = p.port.Write(buf1)
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
	if p.port == nil {
		return errors.New("port already closed")
	}
	errC := make(chan error)
	go func() {
		errC <- p.port.Flush()
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errC:
		return err
	}
}

// Close closes the port.
func (p *ConnectedPort) Close(ctx context.Context) error {
	if p.port == nil {
		return nil
	}
	errC := make(chan error)
	go func() {
		errC <- p.port.Close()
	}()
	var err error
	select {
	case <-ctx.Done():
		err = ctx.Err()
	case err = <-errC:
	}
	if err != nil {
		return err
	}
	p.port = nil
	return nil
}
