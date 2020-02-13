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

func (c *WilcoService) GetStatus(ctx context.Context, req *empty.Empty) (*wpb.GetStatusResponse, error) {
	supportdPID, err := wilco.SupportdPID(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get status of the Wilco DTC Support Daemon")
	}

	vmPID, err := wilco.VMPID(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get status of the Wilco DTC VM")
	}

	if supportdPID < 0 {
		return nil, errors.New("PID of Wilco DTC Support Daemon is negative")
	}

	if vmPID < 0 {
		return nil, errors.New("PID of Wilco DTC VM is negative")
	}

	return &wpb.GetStatusResponse{
		WilcoDtcSupportdPid: uint64(supportdPID),
		WilcoDtcPid:         uint64(vmPID),
	}, nil
}

func (c *WilcoService) GetConfigurationData(ctx context.Context, req *empty.Empty) (*wpb.GetConfigurationDataResponse, error) {
	if status, err := c.GetStatus(ctx, &empty.Empty{}); err != nil {
		return nil, errors.Wrap(err, "failed to get status")
	} else if status.WilcoDtcSupportdPid == 0 {
		return nil, errors.Wrap(err, "Wilco DTC Support Daemon not running")
	} else if status.WilcoDtcPid == 0 {
		return nil, errors.Wrap(err, "Wilco DTC VM not running")
	}

	request := dtcpb.GetConfigurationDataRequest{}
	response := dtcpb.GetConfigurationDataResponse{}

	if err := wilco.DPSLSendMessage(ctx, "GetConfigurationData", &request, &response); err != nil {
		return nil, errors.Wrap(err, "unable to get configuration data")
	}

	return &wpb.GetConfigurationDataResponse{
		JsonConfigurationData: response.JsonConfigurationData,
	}, nil
}
