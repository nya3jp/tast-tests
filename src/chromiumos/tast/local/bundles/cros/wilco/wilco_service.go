// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wilco

import (
	"context"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes/empty"
	"golang.org/x/sys/unix"
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

func (c *WilcoService) SendMessageToUi(ctx context.Context, req *wpb.SendMessageToUiRequest) (*wpb.SendMessageToUiResponse, error) { // NOLINT
	if status, err := c.GetStatus(ctx, &empty.Empty{}); err != nil {
		return nil, errors.Wrap(err, "failed to get status")
	} else if status.WilcoDtcSupportdPid == 0 {
		return nil, errors.Wrap(err, "Wilco DTC Support Daemon not running")
	} else if status.WilcoDtcPid == 0 {
		return nil, errors.Wrap(err, "Wilco DTC VM not running")
	}

	request := dtcpb.SendMessageToUiRequest{
		JsonMessage: req.JsonMessage,
	}
	response := dtcpb.SendMessageToUiResponse{}

	if err := wilco.DPSLSendMessage(ctx, "SendMessageToUi", &request, &response); err != nil {
		return nil, errors.Wrap(err, "unable to send message to UI")
	}

	return &wpb.SendMessageToUiResponse{
		ResponseJsonMessage: response.ResponseJsonMessage,
	}, nil
}

func (c *WilcoService) TestPerformWebRequest(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	{
		request := dtcpb.PerformWebRequestParameter{
			HttpMethod: dtcpb.PerformWebRequestParameter_HTTP_METHOD_GET,
			Url:        "https://chromium.org",
		}
		response := dtcpb.PerformWebRequestResponse{}

		if err := wilco.DPSLSendMessage(ctx, "PerformWebRequest", &request, &response); err != nil {
			return nil, errors.Wrap(err, "failed to call PerformWebRequest")
		}

		if response.Status == dtcpb.PerformWebRequestResponse_STATUS_INTERNAL_ERROR {
			return nil, errors.New("received status STATUS_INTERNAL_ERROR")
		}

		// TODO(vsavu): Remove logging after test works.
		if response.Status != dtcpb.PerformWebRequestResponse_STATUS_OK {
			// Don't fail website isn't reachable from the DUT.
			testing.ContextLogf(ctx, "Request for chromium.org failed with status %s", response.Status)
		}
	}

	{
		request := dtcpb.PerformWebRequestParameter{
			HttpMethod: dtcpb.PerformWebRequestParameter_HTTP_METHOD_GET,
			Url:        "https://localhost/test",
		}
		response := dtcpb.PerformWebRequestResponse{}

		if err := wilco.DPSLSendMessage(ctx, "PerformWebRequest", &request, &response); err != nil {
			return nil, errors.Wrap(err, "failed to call PerformWebRequest")
		}

		// Requests to localhost are blocked.
		if response.Status != dtcpb.PerformWebRequestResponse_STATUS_NETWORK_ERROR {
			return nil, errors.Errorf("invalid status for localhost request; got %s; expect STATUS_NETWORK_ERROR", response.Status)
		}
	}

	// TODO(crbug.com/1064236): add request to test server

	return &empty.Empty{}, nil
}

func (c *WilcoService) ExecuteRoutine(ctx context.Context, req *wpb.ExecuteRoutineRequest) (*wpb.ExecuteRoutineResponse, error) {
	rrRequest := &dtcpb.RunRoutineRequest{}
	if err := proto.Unmarshal(req.Request, rrRequest); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshall request")
	}

	rrResponse := dtcpb.RunRoutineResponse{}

	if err := routines.CallRunRoutine(ctx, rrRequest, &rrResponse); err != nil {
		return nil, errors.Wrap(err, "unable to run routine")
	}

	uuid := rrResponse.Uuid
	response := dtcpb.GetRoutineUpdateResponse{}

	if err := routines.WaitUntilRoutineChangesState(ctx, uuid, dtcpb.DiagnosticRoutineStatus_ROUTINE_STATUS_RUNNING, 30*time.Second); err != nil {
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

func (c *WilcoService) TestRoutineCancellation(ctx context.Context, req *wpb.ExecuteRoutineRequest) (*empty.Empty, error) {
	rrRequest := &dtcpb.RunRoutineRequest{}
	if err := proto.Unmarshal(req.Request, rrRequest); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshall request")
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
	err := routines.WaitUntilRoutineChangesState(ctx, uuid, dtcpb.DiagnosticRoutineStatus_ROUTINE_STATUS_CANCELLING, 30*time.Second)
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

func (c *WilcoService) TestGetAvailableRoutines(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	request := dtcpb.GetAvailableRoutinesRequest{}
	response := dtcpb.GetAvailableRoutinesResponse{}

	if err := wilco.DPSLSendMessage(ctx, "GetAvailableRoutines", &request, &response); err != nil {
		return nil, errors.Wrap(err, "unable to get routines")
	}

	contains := func(all []dtcpb.DiagnosticRoutine, expected dtcpb.DiagnosticRoutine) bool {
		for _, e := range all {
			if expected == e {
				return true
			}
		}
		return false
	}

	for _, val := range []dtcpb.DiagnosticRoutine{
		dtcpb.DiagnosticRoutine_ROUTINE_BATTERY,
		dtcpb.DiagnosticRoutine_ROUTINE_BATTERY_SYSFS,
		dtcpb.DiagnosticRoutine_ROUTINE_URANDOM,
		dtcpb.DiagnosticRoutine_ROUTINE_SMARTCTL_CHECK,
		dtcpb.DiagnosticRoutine_ROUTINE_CPU_CACHE,
		dtcpb.DiagnosticRoutine_ROUTINE_CPU_STRESS,
		dtcpb.DiagnosticRoutine_ROUTINE_FLOATING_POINT_ACCURACY,
		dtcpb.DiagnosticRoutine_ROUTINE_NVME_WEAR_LEVEL,
		dtcpb.DiagnosticRoutine_ROUTINE_NVME_SHORT_SELF_TEST,
		dtcpb.DiagnosticRoutine_ROUTINE_NVME_LONG_SELF_TEST,
	} {
		if !contains(response.Routines, val) {
			return nil, errors.Errorf("routine %s missing", val)
		}
	}
	return &empty.Empty{}, nil
}

func (c *WilcoService) TestGetStatefulPartitionAvailableCapacity(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	const allowedErrorMargin = int32(100) // 100 MiB

	absDiff := func(a, b int32) int32 {
		if a > b {
			return a - b
		}
		return b - a
	}

	request := dtcpb.GetStatefulPartitionAvailableCapacityRequest{}
	response := dtcpb.GetStatefulPartitionAvailableCapacityResponse{}

	if err := wilco.DPSLSendMessage(ctx, "GetStatefulPartitionAvailableCapacity", &request, &response); err != nil {
		return nil, errors.Wrap(err, "unable to get stateful partition available capacity")
	}

	if response.Status != dtcpb.GetStatefulPartitionAvailableCapacityResponse_STATUS_OK {
		return nil, errors.Errorf("unexpected status %d", response.Status)
	}

	var stat unix.Statfs_t
	if err := unix.Statfs("/mnt/stateful_partition", &stat); err != nil {
		return nil, errors.Wrap(err, "failed to get disk stats for the stateful partition")
	}

	realAvailableMb := int32(stat.Bavail * uint64(stat.Bsize) / uint64(1024) / uint64(1024))

	if response.AvailableCapacityMb%int32(100) > 0 {
		return nil, errors.Errorf("invalid available capacity (not rounded to 100 MiB): %v", response.AvailableCapacityMb)
	}

	if absDiff(response.AvailableCapacityMb, realAvailableMb) > allowedErrorMargin {
		return nil, errors.Errorf("invalid available capacity: got %v; want %v +- %v", response.AvailableCapacityMb, realAvailableMb, allowedErrorMargin)
	}

	return &empty.Empty{}, nil
}

func (c *WilcoService) StartDPSLListener(ctx context.Context, req *wpb.StartDPSLListenerRequest) (*empty.Empty, error) {
	if c.receiver != nil {
		return nil, errors.New("DPSL listener already running")
	}

	ctx, cancel := context.WithCancel(context.Background()) // NOLINT

	var response *dtcpb.HandleMessageFromUiResponse

	if len(req.HandleMessageFromUiResponse) > 0 {
		response = &dtcpb.HandleMessageFromUiResponse{
			ResponseJsonMessage: req.HandleMessageFromUiResponse,
		}
	}

	rec, err := wilco.NewDPSLMessageReceiver(ctx, wilco.WithHandleMessageFromUiResponse(response))
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

func (c *WilcoService) WaitForHandleMessageFromUi(ctx context.Context, req *empty.Empty) (*wpb.WaitForHandleMessageFromUiResponse, error) { // NOLINT
	if c.receiver == nil {
		return nil, errors.New("DPSL listener not running")
	}

	msg := dtcpb.HandleMessageFromUiRequest{}

	if err := c.receiver.WaitForMessage(ctx, &msg); err != nil {
		return nil, errors.Wrap(err, "unable to receive HandleMessageFromUi event")
	}

	return &wpb.WaitForHandleMessageFromUiResponse{
		JsonMessage: msg.JsonMessage,
	}, nil
}
