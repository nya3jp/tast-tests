// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package serial

import (
	"context"

	"github.com/golang/protobuf/ptypes/empty"
	"github.com/golang/protobuf/ptypes/wrappers"

	pb "chromiumos/tast/services/cros/firmware"
)

// RemotePort allows communication with a serial port on a remote dut.
type RemotePort struct {
	client pb.SerialPortServiceClient
}

// Read bytes into buffer and returns the number of bytes read.
func (p *RemotePort) Read(ctx context.Context, b []byte) (n int, err error) {
	bufLen := len(b)
	bufVal, err := p.client.Read(ctx, &wrappers.UInt32Value{Value: uint32(bufLen)})
	if err != nil {
		return 0, err
	}
	return copy(b, bufVal.GetValue()), nil
}

// Write bytes in buffer and returns the number of bytes written successfully.
func (p *RemotePort) Write(ctx context.Context, b []byte) (n int, err error) {
	bytesVal := wrappers.BytesValue{Value: b}
	written, err := p.client.Write(ctx, &bytesVal)
	if err != nil {
		return 0, err
	}
	return int(written.GetValue()), nil
}

// Flush un-read/written data on the port.
func (p *RemotePort) Flush(ctx context.Context) error {
	_, err := p.client.Flush(ctx, &empty.Empty{})
	return err
}

// Close the connection.  The port should not be used afterwards.
func (p *RemotePort) Close(ctx context.Context) error {
	if _, err := p.client.Close(ctx, &empty.Empty{}); err != nil {
		return err
	}
	p.client = nil
	return nil
}
