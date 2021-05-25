// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"github.com/golang/protobuf/ptypes/wrappers"
	"google.golang.org/grpc"

	"chromiumos/tast/common/firmware/serial"
	"chromiumos/tast/errors"
	pb "chromiumos/tast/services/cros/firmware"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			pb.RegisterSerialPortServiceServer(srv, &SerialPortService{s: s})
		},
	})
}

// SerialPortService implements tast.cros.firmware.SerialPortService
type SerialPortService struct {
	s    *testing.ServiceState
	port serial.Port
}

// Open handles the Open rpc call.
func (s *SerialPortService) Open(ctx context.Context, in *pb.SerialPortConfig) (*empty.Empty, error) {
	testing.ContextLog(ctx, "Opening service port")
	e := empty.Empty{}
	if s.port != nil {
		return nil, errors.New("Port already opened")
	}
	opener := serial.NewConnectedPortOpener(in.GetName(), int(in.GetBaud()), time.Duration(in.GetReadTimeout()))

	p, err := opener.OpenPort(ctx)
	if err != nil {
		return &e, err
	}
	s.port = p
	return &e, nil
}

// Read handles the Read rpc call.
func (s *SerialPortService) Read(ctx context.Context, in *wrappers.UInt32Value) (*wrappers.BytesValue, error) {
	if s.port == nil {
		return nil, errors.New("Port not opened")
	}

	buf := make([]byte, in.GetValue())
	readLen, err := s.port.Read(ctx, buf)
	v := wrappers.BytesValue{Value: buf[:readLen]}
	return &v, err
}

// Write handles the Write rpc call.
func (s *SerialPortService) Write(ctx context.Context, in *wrappers.BytesValue) (*wrappers.Int64Value, error) {
	if s.port == nil {
		return nil, errors.New("Port not opened")
	}
	n, err := s.port.Write(ctx, in.GetValue())
	v := wrappers.Int64Value{Value: int64(n)}
	return &v, err
}

// Flush handles the Flush rpc call.
func (s *SerialPortService) Flush(ctx context.Context, in *empty.Empty) (*empty.Empty, error) {
	if s.port == nil {
		return nil, errors.New("Port not opened")
	}
	return &empty.Empty{}, s.port.Flush(ctx)
}

// Close handles the Close rpc call.
func (s *SerialPortService) Close(ctx context.Context, in *empty.Empty) (*empty.Empty, error) {
	if s.port == nil {
		return nil, errors.New("Port not opened")
	}
	err := s.port.Close(ctx)
	if err == nil {
		s.port = nil
	}
	return &empty.Empty{}, err
}
