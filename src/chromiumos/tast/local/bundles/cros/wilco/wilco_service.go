// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wilco

import (
	"github.com/golang/protobuf/proto"
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

// ChromeService implements tast.cros.example.ChromeService.
type WilcoService struct {
	s                *testing.ServiceState
	running          bool
	runConfiguration *wpb.RunConfiguration
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
		WilcoDtcRunning: vmPID != 0,
	}, nil
}

func (c *WilcoService) Start(ctx context.Context, req *wpb.RunConfiguration) (*empty.Empty, error) {
	if c.running {
		return nil, errors.New("wilco VM and Support Daemon already running")
	}

	c.runConfiguration = proto.Clone(req).(*wpb.RunConfiguration)

	if req.StartWilcoDtc {
		if err := wilco.StartVM(ctx, &wilco.VMConfig{
			StartProcesses: req.StartWilcoDtcBinaries,
			TestDBusConfig: false,
		}); err != nil {
			return nil, errors.Wrap(err, "unable to start the Wilco DTC VM")
		}

		if req.StartWilcoDtcBinaries {
			if err := wilco.WaitForDDVDBus(ctx); err != nil {
				return nil, errors.Wrap(err, "DDV dbus service is not available")
			}
		}
	}

	if req.StartWilcoDtcSupportd {
		if err := wilco.StartSupportd(ctx); err != nil {
			return nil, errors.Wrap(err, "unable to start the Wilco DTC Support Daemon")
		}
	}

	c.running = true

	return &empty.Empty{}, nil
}

func (c *WilcoService) Stop(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	if !c.running {
		return nil, errors.New("wilco VM and Support Daemon not running")
	}

	if c.runConfiguration.StartWilcoDtc {
		if err := wilco.StopVM(ctx); err != nil {
			return nil, errors.Wrap(err, "unable to stop the Wilco DTC VM")
		}
	}

	if c.runConfiguration.StartWilcoDtcSupportd {
		if err := wilco.StopSupportd(ctx); err != nil {
			return nil, errors.Wrap(err, "unable to stop the Wilco DTC Support Daemon")
		}
	}

	c.running = false

	return &empty.Empty{}, nil
}

func (c *WilcoService) GetConfigurationData(ctx context.Context, req *empty.Empty) (*wpb.GetConfigurationDataResponse, error) {
	if !c.running {
		return nil, errors.New("wilco VM and Support Daemon not running")
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
