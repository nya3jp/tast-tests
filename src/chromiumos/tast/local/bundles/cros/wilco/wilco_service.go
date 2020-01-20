// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wilco

import (
	"github.com/golang/protobuf/ptypes/empty"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/wilco"
	wpb "chromiumos/tast/services/cros/wilco"
	"chromiumos/tast/testing"
	dtcpb "chromiumos/wilco_dtc"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			wpb.RegisterWilcoServiceServer(srv, &WilcoService{s: s})
		},
	})
}

// WilcoService implements tast.cros.wilco.WilcoService.
type WilcoService struct { // NOLINT
	s *testing.ServiceState
}

func (c *WilcoService) GetStatus(ctx context.Context, req *empty.Empty) (*wpb.RunningStatus, error) {
	supportdPID, err := wilco.SupportdPID(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get status of the Wilco DTC Support Daemon")
	}

	vmPID, err := wilco.VMPID(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get status of the Wilco DTC VM")
	}

	return &wpb.RunningStatus{
		WilcoDtcSupportdRunning: supportdPID != 0,
		WilcoDtcRunning:         vmPID != 0,
	}, nil
}

func (c *WilcoService) GetConfigurationData(ctx context.Context, req *empty.Empty) (*wpb.GetConfigurationDataResponse, error) {
	if status, err := c.GetStatus(ctx, &empty.Empty{}); err != nil {
		return nil, errors.Wrap(err, "failed to get status")
	} else if !status.WilcoDtcSupportdRunning {
		return nil, errors.Wrap(err, "Wilco DTC Support Daemon not running")
	} else if !status.WilcoDtcRunning {
		return nil, errors.Wrap(err, "Wilco DTC VM not running")
	}

	request := dtcpb.GetConfigurationDataRequest{}
	response := dtcpb.GetConfigurationDataResponse{}

	if err := wilco.DPSLSendMessage(ctx, "GetConfigurationData", &request, &response); err != nil {
		return nil, errors.Wrap(err, "unable to get configuration data")
	}

	var ret wpb.GetConfigurationDataResponse
	ret.JsonConfigurationData = response.JsonConfigurationData
	return &ret, nil
}
