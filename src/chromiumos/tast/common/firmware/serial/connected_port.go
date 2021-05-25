// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package serial

import (
	"context"

	tarm "github.com/tarm/serial"
)

// ConnectedPort shares the interface with RemotePort, delegates to tarm.serial.
type ConnectedPort struct {
	port *tarm.Port
}

// Read reads bytes into provided buffer, returns number of bytes read.
func (p *ConnectedPort) Read(ctx context.Context, buf []byte) (n int, err error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	return p.port.Read(buf)
}

// Write writes bytes from provided buffer, returns number of bytes written.
func (p *ConnectedPort) Write(ctx context.Context, buf []byte) (n int, err error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	return p.port.Write(buf)
}

// Flush flushes un-read/written content from the port.
func (p *ConnectedPort) Flush(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return p.port.Flush()
}

// Close closes the port.
func (p *ConnectedPort) Close(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return p.port.Close()
}
