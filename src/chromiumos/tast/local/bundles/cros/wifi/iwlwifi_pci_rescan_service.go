// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/local/bundles/cros/wifi/iwlwifirescan"
	"chromiumos/tast/services/cros/wifi"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			wifi.RegisterIwlwifiPCIRescanServer(srv, &IwlwifiPCIRescanService{})
		},
	})
}

// IwlwifiPCIRescanService implements tast.cros.wifi.IwlwifiPCIRescan gRPC service.
type IwlwifiPCIRescanService struct{}

// RemoveIfaceAndWaitForRecovery triggers iwlwifi-rescan rule by removing the WiFi device.
// iwlwifi-rescan will rescan PCI bus and bring the WiFi device back.
func (s *IwlwifiPCIRescanService) RemoveIfaceAndWaitForRecovery(ctx context.Context, _ *empty.Empty) (*empty.Empty, error) {
	if err := iwlwifirescan.RemoveIfaceAndWaitForRecovery(ctx); err != nil {
		return nil, err
	}
	return &empty.Empty{}, nil
}
