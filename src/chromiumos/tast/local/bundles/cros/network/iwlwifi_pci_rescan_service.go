// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/network/iwlwifirescan"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/services/cros/network"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			network.RegisterIwlwifiPCIRescanServer(srv, &IwlwifiPCIRescanService{})
		},
	})
}

// IwlwifiPCIRescanService implements tast.cros.network.IwlwifiPCIRescan gRPC service.
type IwlwifiPCIRescanService struct{}

// RemoveIfaceAndWaitForRecovery triggers iwlwifi-rescan rule by removing the WiFi device.
// iwlwifi-rescan will rescan PCI bus and bring the WiFi device back.
func (s *IwlwifiPCIRescanService) RemoveIfaceAndWaitForRecovery(ctx context.Context, _ *empty.Empty) (*empty.Empty, error) {
	if err := iwlwifirescan.RemoveIfaceAndWaitForRecovery(ctx); err != nil {
		return nil, err
	}
	return &empty.Empty{}, nil
}

// HealthCheck checks if the DUT has a WiFi device. If not, we may need to reboot the DUT.
func (s *IwlwifiPCIRescanService) HealthCheck(ctx context.Context, _ *empty.Empty) (*empty.Empty, error) {
	manager, err := shill.NewManager(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create shill manager")
	}
	_, err = shill.WifiInterface(ctx, manager, 5*time.Second)
	if err != nil {
		return nil, errors.Wrap(err, "could not get a WiFi interface")
	}
	return &empty.Empty{}, nil
}
