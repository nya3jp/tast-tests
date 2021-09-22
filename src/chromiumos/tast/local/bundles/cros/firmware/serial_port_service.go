// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"

	"github.com/golang/protobuf/ptypes"
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
	s        *testing.ServiceState
	ports    map[uint32]serial.Port
	nextPort uint32
}

// Open handles the Open rpc call.
func (s *SerialPortService) Open(ctx context.Context, in *pb.SerialPortConfig) (*pb.PortId, error) {
	testing.ContextLog(ctx, "Opening service port")

	readTimeout, err := ptypes.Duration(in.GetReadTimeout())
	if err != nil {
		return nil, errors.Wrap(err, "converting ReadTimeout")
	}
	p, err := serial.NewConnectedPortOpener(in.GetName(), int(in.GetBaud()), readTimeout).OpenPort(ctx)
	if err != nil {
		return nil, err
	}

	if s.ports == nil {
		s.ports = make(map[uint32]serial.Port)
		s.nextPort = 1
	}
	id := s.nextPort
	s.nextPort++
	s.ports[id] = p
	return &pb.PortId{Value: id}, nil
}

func (s *SerialPortService) getPort(id uint32) (serial.Port, error) {
	if s.ports == nil {
		return nil, errors.New("no ports have been opened")
	}

	p, ok := s.ports[id]
	if !ok {
		return nil, errors.Errorf("port %d not found", id)
	}

	return p, nil
}

// Read handles the Read rpc call.
func (s *SerialPortService) Read(ctx context.Context, in *pb.SerialReadRequest) (*wrappers.BytesValue, error) {
	p, err := s.getPort(in.GetId().GetValue())
	if err != nil {
		return nil, err
	}
	buf := make([]byte, in.GetMaxLen())
	readLen, err := p.Read(ctx, buf)
	if err != nil {
		return nil, err
	}
	return &wrappers.BytesValue{Value: buf[:readLen]}, err
}

// Write handles the Write rpc call.
func (s *SerialPortService) Write(ctx context.Context, in *pb.SerialWriteRequest) (*wrappers.Int64Value, error) {
	p, err := s.getPort(in.GetId().GetValue())
	if err != nil {
		return nil, err
	}
	n, err := p.Write(ctx, in.GetBuffer())
	if err != nil {
		return nil, err
	}
	return &wrappers.Int64Value{Value: int64(n)}, err
}

// Flush handles the Flush rpc call.
func (s *SerialPortService) Flush(ctx context.Context, in *pb.PortId) (*empty.Empty, error) {
	p, err := s.getPort(in.GetValue())
	if err != nil {
		return nil, err
	}
	return &empty.Empty{}, p.Flush(ctx)
}

// Close handles the Close rpc call.
func (s *SerialPortService) Close(ctx context.Context, in *pb.PortId) (*empty.Empty, error) {
	id := in.GetValue()
	p, err := s.getPort(id)
	if err != nil {
		return nil, err
	}
	if err = p.Close(ctx); err != nil {
		return nil, err
	}
	delete(s.ports, id)
	return &empty.Empty{}, nil
}
