// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/shill"
	cellular_pb "chromiumos/tast/services/cros/cellular"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			cellular_pb.RegisterRemoteCellularServiceServer(srv, &RemoteCellularService{state: s})
		},
	})
}

// RemoteCellularService implements tast.cros.cellular.RemoteCellularService.
type RemoteCellularService struct {
	state *testing.ServiceState
}

// QueryInterface returns information about the cellular device interface.
// Note: This method assumes that:
// 1. There is a unique Cellular Device.
// 2. The "interface" field of the Cellular Device corresponds to the data connection.
func (s *RemoteCellularService) QueryInterface(ctx context.Context, _ *empty.Empty) (*cellular_pb.QueryInterfaceResponse, error) {
	manager, err := shill.NewManager(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create shill manager")
	}

	device, err := manager.DeviceByType(ctx, shillconst.TypeCellular)
	if err != nil || device == nil {
		return nil, errors.Wrap(err, "failed to get cellular device")
	}

	props, err := device.GetProperties(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get device properties")
	}

	iface, err := props.GetString(shillconst.DevicePropertyInterface)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get device interface from properties")
	}

	return &cellular_pb.QueryInterfaceResponse{
		Name: iface,
	}, nil
}
