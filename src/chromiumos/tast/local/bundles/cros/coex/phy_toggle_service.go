// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package coex

import (
	"context"
	// "time"

	"chromiumos/tast/local/bundles/cros/coex/phy_toggle"
	"chromiumos/tast/services/cros/coex"
	"chromiumos/tast/testing"
	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"
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

func (s *PhyToggleService) BringIfUp(ctx context.Context, request *coex.Credentials) (*empty.Empty, error) {
	if err := phy_toggle.BringIfUp(ctx, request.Req); err != nil {
		return nil, err
	}
	return &empty.Empty{}, nil
}
func (s *PhyToggleService) AssertIfUp(ctx context.Context, _ *empty.Empty) (*empty.Empty, error) {
	if err := phy_toggle.AssertIfUp(ctx); err != nil {
		return nil, err
	}
	return &empty.Empty{}, nil
}
func (s *PhyToggleService) ChangeBluetooth(ctx context.Context, request *coex.Credentials) (*empty.Empty, error) {
	if err := phy_toggle.ChangeBluetooth(ctx, "on", request.Req); err != nil {
		return nil, err
	}
	return &empty.Empty{}, nil
}

// // RemoveIfaceAndWaitForRecovery triggers iwlwifi-rescan rule by removing the WiFi device.
// // iwlwifi-rescan will rescan PCI bus and bring the WiFi device back.
// func (s *IwlwifiPCIRescanService) RemoveIfaceAndWaitForRecovery(ctx context.Context, _ *empty.Empty) (*empty.Empty, error) {
//     if err := iwlwifirescan.RemoveIfaceAndWaitForRecovery(ctx); err != nil {
//         return nil, err
//     }
//     return &empty.Empty{}, nil
// }

// // HealthCheck checks if the DUT has a WiFi device. If not, we may need to reboot the DUT.
// func (s *IwlwifiPCIRescanService) HealthCheck(ctx context.Context, _ *empty.Empty) (*empty.Empty, error) {
//     manager, err := shill.NewManager(ctx)
//     if err != nil {
//         return nil, errors.Wrap(err, "failed to create shill manager")
//     }
//     _, err = shill.WifiInterface(ctx, manager, 5*time.Second)
//     if err != nil {
//         return nil, errors.Wrap(err, "could not get a WiFi interface")
//     }
//     return &empty.Empty{}, nil
// }
