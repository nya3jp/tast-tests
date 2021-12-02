// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ti50

import (
	"context"
	"io"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware/ti50/dutcontrol"
	"chromiumos/tast/testing"
)

// ErrReadTimeout indicates that a read operation exceeded the readTimeout configured on the port.
// Subsequent reads may still succeed depending on the state of the port.
var ErrReadTimeout = errors.New("read timeout")

// DUTControlPort is a dutcontrol console port.
type DUTControlPort struct {
	stream      dutcontrol.DutControl_ConsoleClient
	data        <-chan *dutcontrol.ConsoleSerialData
	written     <-chan *dutcontrol.ConsoleSerialWriteResult
	readTimeout time.Duration
	unreadBuf   []byte
}

// Read bytes into buffer and return number of bytes read.
// Bytes already written to the port shall be moved into buf, up to its size.
func (p *DUTControlPort) Read(ctx context.Context, buf []byte) (n int, err error) {
	if len(p.unreadBuf) >= len(buf) {
		n := copy(buf, p.unreadBuf)
		p.unreadBuf = p.unreadBuf[n:]
		return n, nil
	}
	timer := time.NewTimer(p.readTimeout)
	select {
	case d, more := <-p.data:
		if !more {
			return 0, io.EOF
		}
		p.unreadBuf = append(p.unreadBuf, d.Data...)
		n := copy(buf, p.unreadBuf)
		p.unreadBuf = p.unreadBuf[n:]
		if d.Err != "" {
			return n, errors.New(d.Err)
		}
		return n, nil
	case <-timer.C:
		return 0, ErrReadTimeout
	case <-ctx.Done():
		return 0, ctx.Err()
	}
}

// Write bytes from buffer and return number of bytes written.
// It returns a non-nil error when n != len(b), nil otherwise.
func (p *DUTControlPort) Write(ctx context.Context, buf []byte) (int, error) {
	err := p.stream.Send(&dutcontrol.ConsoleRequest{
		Operation: &dutcontrol.ConsoleRequest_SerialWrite{
			SerialWrite: &dutcontrol.ConsoleSerialWrite{
				Data: buf,
			},
		},
	})

	if err != nil {
		return 0, err
	}

	select {
	case d, more := <-p.written:
		if !more {
			return 0, io.EOF
		}
		if d.Err != "" {
			err = errors.New(d.Err)
		}
		return int(d.N), err
	case <-ctx.Done():
		return 0, ctx.Err()
	}
}

// Flush un-read/written bytes.
func (p *DUTControlPort) Flush(ctx context.Context) error {
	p.unreadBuf = nil

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-p.data:
		case <-p.written:
		default:
			return nil
		}
	}
}

// Close closes the port.
func (p *DUTControlPort) Close(ctx context.Context) error {
	testing.ContextLog(ctx, "Closing DUTControlPort")
	return p.stream.CloseSend()
}
