// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bluetooth"
	"chromiumos/tast/services/cros/network"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			network.RegisterBluetoothServiceServer(srv, &BluetoothService{s: s})
		},
	})
}

// BluetoothService implements tast.cros.network.BluetoothService gRPC service.
type BluetoothService struct {
	s *testing.ServiceState
}

//SetBluetoothStatus sets the bluetooth adapter power status. This setting does not persist across reboots.
func (s *BluetoothService) SetBluetoothStatus(ctx context.Context, req *network.SetBluetoothStatusRequest) (*empty.Empty, error) {
	adapters, err := bluetooth.Adapters(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "unable to get Bluetooth adapters")
	}
	if len(adapters) != 1 {
		return nil, errors.Errorf("too many adapters, got %d adapters", len(adapters))
	}
	if err := adapters[0].SetPowered(ctx, req.State); err != nil {
		return nil, errors.Wrap(err, "couldn't set bluetooth powered state")
	}
	return &empty.Empty{}, nil
}

// GetBluetoothStatus checks whether the bluetooth adapter is enabled
func (s *BluetoothService) GetBluetoothStatus(ctx context.Context, _ *empty.Empty) (*network.GetBluetoothStatusResponse, error) {
	adapters, err := bluetooth.Adapters(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "unable to get Bluetooth adapters")
	}

	if len(adapters) != 1 {
		return &network.GetBluetoothStatusResponse{Status: false}, nil
	}
	res, err := adapters[0].Powered(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "could not get bluetooth power state")
	}
	return &network.GetBluetoothStatusResponse{Status: res}, nil
}
