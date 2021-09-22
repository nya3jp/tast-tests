// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package serial

import (
	"context"

	pb "chromiumos/tast/services/cros/firmware"
)

// RemotePort allows communication with a serial port on a remote dut.
type RemotePort struct {
	client pb.SerialPortServiceClient
	id     uint32
}

// Read bytes into buffer and returns the number of bytes read.
// Bytes already written to the port shall be moved into buf, up to its size.
// In blocking mode (port opened without ReadTimeout), it blocks until at least
// one byte is read.  In non-blocking mode, it may return an error with zero
// bytes read if ReadTimeout is exceeded, however this is not guaranteed on all
// platforms and so a context deadline exceeded error may be returned even
// though the deadline is greater than the ReadTimeout.
func (p *RemotePort) Read(ctx context.Context, b []byte) (n int, err error) {
	buf, err := p.client.Read(ctx, &pb.SerialReadRequest{Id: &pb.PortId{Value: p.id}, MaxLen: uint32(len(b))})
	if err != nil {
		return 0, err
	}
	return copy(b, buf.GetValue()), nil
}

// Write bytes in buffer and returns the number of bytes written successfully.
// It returns a non-nil error when n != len(b), nil otherwise.
func (p *RemotePort) Write(ctx context.Context, b []byte) (n int, err error) {
	written, err := p.client.Write(ctx, &pb.SerialWriteRequest{Id: &pb.PortId{Value: p.id}, Buffer: b})
	if err != nil {
		return 0, err
	}
	return int(written.GetValue()), nil
}

// Flush un-read/written data on the port.
func (p *RemotePort) Flush(ctx context.Context) error {
	_, err := p.client.Flush(ctx, &pb.PortId{Value: p.id})
	return err
}

// Close the connection. The port should not be used afterwards.
func (p *RemotePort) Close(ctx context.Context) error {
	if p.client == nil {
		return nil
	}
	if _, err := p.client.Close(ctx, &pb.PortId{Value: p.id}); err != nil {
		return err
	}
	p.client = nil
	p.id = 0
	return nil
}
