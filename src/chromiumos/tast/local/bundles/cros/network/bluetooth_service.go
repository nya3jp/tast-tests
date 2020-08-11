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

// SetBluetoothPowered sets the Bluetooth adapter power status. This setting does not persist across reboots.
func (s *BluetoothService) SetBluetoothPowered(ctx context.Context, req *network.SetBluetoothPoweredRequest) (*empty.Empty, error) {
	adapters, err := bluetooth.Adapters(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "unable to get Bluetooth adapters")
	}
	if len(adapters) != 1 {
		return nil, errors.Errorf("got %d adapters, expected 1 adapter", len(adapters))
	}
	if err := adapters[0].SetPowered(ctx, req.Powered); err != nil {
		return nil, errors.Wrap(err, "couldn't set Bluetooth powered state")
	}
	return &empty.Empty{}, nil
}

// GetBluetoothPowered checks whether the Bluetooth adapter is enabled.
func (s *BluetoothService) GetBluetoothPowered(ctx context.Context, _ *empty.Empty) (*network.GetBluetoothPoweredResponse, error) {
	adapters, err := bluetooth.Adapters(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "unable to get Bluetooth adapters")
	}

	if len(adapters) != 1 {
		return &network.GetBluetoothPoweredResponse{Powered: false}, nil
	}
	res, err := adapters[0].Powered(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "could not get Bluetooth power state")
	}
	return &network.GetBluetoothPoweredResponse{Powered: res}, nil
}

// ValidateBluetoothFunctional checks to see whether the Bluetooth device is usable.
func (s *BluetoothService) ValidateBluetoothFunctional(ctx context.Context, _ *empty.Empty) (*empty.Empty, error) {
	adapters, err := bluetooth.Adapters(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "unable to get Bluetooth adapters")
	}

	if len(adapters) != 1 {
		return nil, errors.Errorf("got %d adapters, expected 1 adapter", len(adapters))
	}
	// If the Bluetooth device is not usable, the discovery will error out.
	err = adapters[0].StartDiscovery(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to start Bluetooth adapter discovery")
	}
	// We don't actually care about the discovery contents, just whether or not
	// the discovery failed or not. We can stop the scan immediately.
	err = adapters[0].StopDiscovery(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to stop Bluetooth adapter discovery")
	}
	return &empty.Empty{}, nil
}
