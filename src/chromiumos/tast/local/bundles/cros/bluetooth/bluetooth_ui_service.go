// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bluetooth

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bluetooth"
	"chromiumos/tast/local/bluetooth/bluez"
	"chromiumos/tast/local/bluetooth/floss"
	"chromiumos/tast/local/common"
	pb "chromiumos/tast/services/cros/bluetooth"
	"chromiumos/tast/testing"
)

func init() {
	var bluetoothUIService Service
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			bluetoothUIService = Service{sharedObject: common.SharedObjectsForServiceSingleton}
			pb.RegisterBluetoothUIServiceServer(srv, &bluetoothUIService)
		},
	})
}

// Service implements tast.cros.bluetooth.BluetoothUIService.
type Service struct {
	sharedObject *common.SharedObjectsForService
	impl         bluetooth.Bluetooth
}

func (s *Service) Initialize(ctx context.Context, req *pb.InitializeRequest) (*emptypb.Empty, error) {
	if s.impl != nil {
		return nil, errors.New("Initialize cannot be called more than once")
	}
	if req.UseFloss {
		s.impl = &floss.Floss{}
	} else {
		s.impl = &bluez.BlueZ{}
	}
	return &emptypb.Empty{}, nil
}

func (s *Service) Enable(ctx context.Context, empty *emptypb.Empty) (*emptypb.Empty, error) {
	if s.impl == nil {
		return nil, errors.New("Initialize must be called before any other method")
	}
	if err := s.impl.Enable(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to enable Bluetooth")
	}
	return &emptypb.Empty{}, nil
}

func (s *Service) PollForAdapterState(ctx context.Context, req *pb.PollForAdapterStateRequest) (*emptypb.Empty, error) {
	if s.impl == nil {
		return nil, errors.New("Initialize must be called before any other method")
	}
	if err := s.impl.PollForAdapterState(ctx, req.ExpectedState); err != nil {
		return nil, errors.Wrap(err, "failed to enable Bluetooth")
	}
	return &emptypb.Empty{}, nil
}

func (s *Service) Devices(ctx context.Context, empty *emptypb.Empty) (*pb.DevicesResponse, error) {
	if s.impl == nil {
		return nil, errors.New("Initialize must be called before any other method")
	}
	deviceInfos, err := s.impl.Devices(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to collect device information")
	}
	res := &pb.DevicesResponse{}
	res.DeviceInfos = make([]*pb.DeviceInfo, len(deviceInfos))
	for i, deviceInfo := range deviceInfos {
		res.DeviceInfos[i] = &pb.DeviceInfo{
			Address: deviceInfo.Address,
			Name:    deviceInfo.Name,
		}
	}
	return res, nil
}

func (s *Service) StartDiscovery(ctx context.Context, empty *emptypb.Empty) (*emptypb.Empty, error) {
	if s.impl == nil {
		return nil, errors.New("Initialize must be called before any other method")
	}
	if err := s.impl.StartDiscovery(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to start discovery")
	}
	return &emptypb.Empty{}, nil
}

func (s *Service) StopDiscovery(ctx context.Context, empty *emptypb.Empty) (*emptypb.Empty, error) {
	if s.impl == nil {
		return nil, errors.New("Initialize must be called before any other method")
	}
	if err := s.impl.Reset(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to stop discovery")
	}
	return &emptypb.Empty{}, nil
}

func (s *Service) Reset(ctx context.Context, empty *emptypb.Empty) (*emptypb.Empty, error) {
	if s.impl == nil {
		return nil, errors.New("Initialize must be called before any other method")
	}
	if err := s.impl.Reset(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to reset the Bluetooth state")
	}
	return &emptypb.Empty{}, nil
}
