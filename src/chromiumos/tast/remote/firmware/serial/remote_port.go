// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package serial

import (
	"context"

	"github.com/golang/protobuf/ptypes/wrappers"

	pb "chromiumos/tast/services/cros/firmware"
)

// RemotePort allows communication with a serial port on a remote dut.
type RemotePort struct {
	client pb.SerialPortServiceClient
	id     uint32
}

// Read bytes into buffer and returns the number of bytes read.
func (p *RemotePort) Read(ctx context.Context, b []byte) (n int, err error) {
	bufLen := len(b)
	bufVal, err := p.client.Read(ctx, &pb.SerialReadRequest{Id: p.id, MaxLen: uint32(bufLen)})
	if err != nil {
		return 0, err
	}
	return copy(b, bufVal.GetValue()), nil
}

// Write bytes in buffer and returns the number of bytes written successfully.
func (p *RemotePort) Write(ctx context.Context, b []byte) (n int, err error) {
	bytesVal := pb.SerialWriteRequest{Id: p.id, Buffer: b}
	written, err := p.client.Write(ctx, &bytesVal)
	if err != nil {
		return 0, err
	}
	return int(written.GetValue()), nil
}

// Flush un-read/written data on the port.
func (p *RemotePort) Flush(ctx context.Context) error {
	_, err := p.client.Flush(ctx, &wrappers.UInt32Value{Value: p.id})
	return err
}

// Close the connection.  The port should not be used afterwards.
func (p *RemotePort) Close(ctx context.Context) error {
	if p.client == nil {
		return nil
	}
	if _, err := p.client.Close(ctx, &wrappers.UInt32Value{Value: p.id}); err != nil {
		return err
	}
	p.client = nil
	p.id = 0
	return nil
}
