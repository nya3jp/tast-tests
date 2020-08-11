// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package coex

import (
	"context"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/local/bundles/cros/coex/phytoggle"
	"chromiumos/tast/services/cros/coex"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			coex.RegisterPhyToggleServer(srv, &PhyToggleService{})
		},
	})
}

// PhyToggleService implements tast.cros.coex.PhyToggle gRPC service.
type PhyToggleService struct{}

func (s *PhyToggleService) AssertPhysUp(ctx context.Context, _ *empty.Empty) (*empty.Empty, error) {
	if err := phytoggle.AssertPhysUp(ctx); err != nil {
		return nil, err
	}
	return &empty.Empty{}, nil
}

func (s *PhyToggleService) BringPhysUp(ctx context.Context, request *coex.Credentials) (*empty.Empty, error) {
	if err := phytoggle.BringPhysUp(ctx, request.Req); err != nil {
		return nil, err
	}
	return &empty.Empty{}, nil
}

func (s *PhyToggleService) AssertBluetoothUp(ctx context.Context, _ *empty.Empty) (*empty.Empty, error) {
	if res, err := phytoggle.BluetoothStatus(ctx); err != nil || !res {
		return nil, err
	}
	return &empty.Empty{}, nil
}

func (s *PhyToggleService) AssertBluetoothDown(ctx context.Context, _ *empty.Empty) (*empty.Empty, error) {
	if res, err := phytoggle.BluetoothStatus(ctx); err != nil || res {
		return nil, err
	}
	return &empty.Empty{}, nil
}

func (s *PhyToggleService) AssertWifiUp(ctx context.Context, _ *empty.Empty) (*empty.Empty, error) {
	if res, err := phytoggle.WifiStatus(ctx); !res {
		return nil, err
	}
	return &empty.Empty{}, nil
}

func (s *PhyToggleService) AssertWifiDown(ctx context.Context, _ *empty.Empty) (*empty.Empty, error) {
	if res, err := phytoggle.WifiStatus(ctx); res {
		return nil, err
	}
	return &empty.Empty{}, nil
}

func (s *PhyToggleService) DisableBluetooth(ctx context.Context, request *coex.Credentials) (*empty.Empty, error) {
	if err := phytoggle.ChangeBluetooth(ctx, "on", request.Req); err != nil {
		return nil, err
	}
	return &empty.Empty{}, nil
}

func (s *PhyToggleService) DisableWifi(ctx context.Context, request *coex.Credentials) (*empty.Empty, error) {
	if err := phytoggle.ChangeWifi(ctx, "on", request.Req); err != nil {
		return nil, err
	}
	return &empty.Empty{}, nil
}
