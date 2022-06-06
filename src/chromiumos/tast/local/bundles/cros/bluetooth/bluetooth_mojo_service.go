// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bluetooth

import (
	"context"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	pb "chromiumos/tast/services/cros/bluetooth"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			pb.RegisterBluetoothMojoServiceServer(srv, &BluetoothMojoService{s: s})
		},
	})
}

// BluetoothService implements tast.cros.bluetooth.BluetoothMojoService.
type BluetoothMojoService struct {
	s *testing.ServiceState
}

func (c *BluetoothMojoService) SetBluetoothState(ctx context.Context, req *pb.SetBluetoothStateRequest) (*empty.Empty, error) {

	return &empty.Empty{}, nil
}
