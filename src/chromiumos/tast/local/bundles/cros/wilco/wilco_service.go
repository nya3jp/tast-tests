// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wilco

import (
	"context"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/wilco/routines"
	"chromiumos/tast/local/wilco"
	wpb "chromiumos/tast/services/cros/wilco"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
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
	s                 *testing.ServiceState
	receiver          *wilco.DPSLMessageReceiver
	receiverCtxCancel func()
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

func (c *WilcoService) RestartVM(ctx context.Context, req *wpb.RestartVMRequest) (*empty.Empty, error) {
	if err := wilco.StopVM(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to stop VM")
	}

	config := wilco.DefaultVMConfig()
	config.StartProcesses = req.StartProcesses
	config.TestDBusConfig = req.TestDbusConfig

	if err := wilco.StartVM(ctx, config); err != nil {
		return nil, errors.Wrap(err, "failed to start VM")
	}

	return &empty.Empty{}, nil
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

func (c *WilcoService) ExecuteRoutine(ctx context.Context, req *wpb.ExecuteRoutineRequest) (*wpb.ExecuteRoutineResponse, error) {
	rrRequest := dtcpb.RunRoutineRequest{}
	if err := proto.Unmarshal(req.Request, &rrRequest); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshall request")
	}

	rrResponse := dtcpb.RunRoutineResponse{}

	if err := routines.CallRunRoutine(ctx, rrRequest, &rrResponse); err != nil {
		return nil, errors.Wrap(err, "unable to run routine")
	}

	uuid := rrResponse.Uuid
	response := dtcpb.GetRoutineUpdateResponse{}

	if err := routines.WaitUntilRoutineChangesState(ctx, uuid, dtcpb.DiagnosticRoutineStatus_ROUTINE_STATUS_RUNNING, 2*time.Second); err != nil {
		return nil, errors.Wrap(err, "routine not finished")
	}

	if err := routines.GetRoutineStatus(ctx, uuid, true, &response); err != nil {
		return nil, errors.Wrap(err, "unable to get routine status")
	}

	var status wpb.DiagnosticRoutineStatus
	switch response.Status {
	case dtcpb.DiagnosticRoutineStatus_ROUTINE_STATUS_FAILED:
		status = wpb.DiagnosticRoutineStatus_ROUTINE_STATUS_FAILED
	case dtcpb.DiagnosticRoutineStatus_ROUTINE_STATUS_PASSED:
		status = wpb.DiagnosticRoutineStatus_ROUTINE_STATUS_PASSED
	default:
		testing.ContextLogf(ctx, "Unexpected routine status %s", response.Status)
		status = wpb.DiagnosticRoutineStatus_ROUTINE_STATUS_ERROR
	}

	if err := routines.RemoveRoutine(ctx, uuid); err != nil {
		return nil, errors.Wrap(err, "unable to remove routine")
	}

	return &wpb.ExecuteRoutineResponse{
		Status: status,
	}, nil
}

func (c *WilcoService) TestRoutineCancellation(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	rrRequest := dtcpb.RunRoutineRequest{
		Routine: dtcpb.DiagnosticRoutine_ROUTINE_URANDOM,
		Parameters: &dtcpb.RunRoutineRequest_UrandomParams{
			UrandomParams: &dtcpb.UrandomRoutineParameters{
				LengthSeconds: 5,
			},
		},
	}
	rrResponse := dtcpb.RunRoutineResponse{}
	if err := routines.CallRunRoutine(ctx, rrRequest, &rrResponse); err != nil {
		return nil, errors.Wrap(err, "unable to call routine")
	}

	uuid := rrResponse.Uuid
	response := dtcpb.GetRoutineUpdateResponse{}

	if err := routines.CancelRoutine(ctx, uuid); err != nil {
		return nil, errors.Wrap(err, "unable to cancel routine")
	}

	// Because cancellation is slow, we time how long it takes to change from
	// STATUS_CANCELLING.
	ctx, st := timing.Start(ctx, "cancel")
	err := routines.WaitUntilRoutineChangesState(ctx, uuid, dtcpb.DiagnosticRoutineStatus_ROUTINE_STATUS_CANCELLING, 4*time.Second)
	st.End()
	if err != nil {
		return nil, errors.Wrap(err, "routine not finished")
	}

	if err := routines.GetRoutineStatus(ctx, uuid, true, &response); err != nil {
		return nil, errors.Wrap(err, "unable to get routine status")
	}

	if response.Status != dtcpb.DiagnosticRoutineStatus_ROUTINE_STATUS_CANCELLED {
		return nil, errors.Errorf("invalid status; got %s, want ROUTINE_STATUS_CANCELLED", response.Status)
	}

	if err := routines.RemoveRoutine(ctx, uuid); err != nil {
		return nil, errors.Wrap(err, "unable to remove routine")
	}

	return &empty.Empty{}, nil
}

func (c *WilcoService) StartDPSLListener(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	if c.receiver != nil {
		return nil, errors.New("DPSL listener already running")
	}

	ctx, cancel := context.WithCancel(context.Background()) // NOLINT
	rec, err := wilco.NewDPSLMessageReceiver(ctx)
	if err != nil {
		cancel()
		return nil, errors.Wrap(err, "failed to create dpsl message listener")
	}

	c.receiver = rec
	c.receiverCtxCancel = cancel

	return &empty.Empty{}, nil
}

func (c *WilcoService) StopDPSLListener(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	if c.receiver == nil {
		return nil, errors.New("DPSL listener not running")
	}

	c.receiver.Stop(ctx)
	c.receiver = nil
	c.receiverCtxCancel()

	return &empty.Empty{}, nil
}

func (c *WilcoService) WaitForHandleConfigurationDataChanged(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	if c.receiver == nil {
		return nil, errors.New("DPSL listener not running")
	}

	msg := dtcpb.HandleConfigurationDataChangedRequest{}

	if err := c.receiver.WaitForMessage(ctx, &msg); err != nil {
		return nil, errors.Wrap(err, "unable to receive HandleConfigurationDataChanged event")
	}

	return &empty.Empty{}, nil
}
