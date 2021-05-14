// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/common/testexec"
	upstartcommon "chromiumos/tast/common/upstart"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/upstart"
	arcpb "chromiumos/tast/services/cros/arc"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			arcpb.RegisterADBOverUSBServiceServer(srv, &ADBOverUSBService{s: s})
		},
	})
}

// ADBOverUSBService implements tast.cros.arc.ADBOverUSBService
type ADBOverUSBService struct {
	s *testing.ServiceState
}

// SetUDCEnabled enables or disables UDC on DUT. If initial setting is as the same as requested, it will be no-ops.
func (*ADBOverUSBService) SetUDCEnabled(ctx context.Context, request *arcpb.EnableUDCRequest) (*arcpb.EnableUDCResponse, error) {
	output, err := testexec.CommandContext(ctx, "crossystem", "dev_enable_udc").Output(testexec.DumpLogOnError)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read dev_enable_udc")
	}

	UDCEnabled := strings.TrimSpace(string(output))

	var requestUDCStringValue string
	if request.Enable {
		requestUDCStringValue = "1"
	} else {
		requestUDCStringValue = "0"
	}

	if UDCEnabled == requestUDCStringValue {
		response := arcpb.EnableUDCResponse{
			UDCValueUpdated: false,
		}
		return &response, nil
	}

	command := "dev_enable_udc=" + requestUDCStringValue
	if err := testexec.CommandContext(ctx, "crossystem", command).Run(testexec.DumpLogOnError); err != nil {
		return nil, errors.Wrapf(err, "failed to run crossystem %v", command)
	}
	response := arcpb.EnableUDCResponse{
		UDCValueUpdated: true,
	}
	return &response, nil
}

// CheckADBDJobStatus checks arc(vm)-adbd job status
func (*ADBOverUSBService) CheckADBDJobStatus(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	cr, err := chrome.New(ctx, chrome.ARCEnabled())
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to Chrome")
	}
	defer func() {
		if err := cr.Close(ctx); err != nil {
			testing.ContextLog(ctx, "Failed to close Chrome while (re)booting ARC")
		}
	}()

	outDir, ok := testing.ContextOutDir(ctx)
	if !ok {
		return nil, errors.Wrap(err, "failed to get output dir")
	}

	a, err := arc.New(ctx, outDir)
	if err != nil {
		return nil, errors.Wrap(err, "failed to start ARC")
	}
	defer func() {
		if a != nil {
			a.Close(ctx)
		}
	}()

	testing.ContextLog(ctx, "checking status of arc(vm)-adbd job")
	vmEnabled, err := arc.VMEnabled()
	if err != nil {
		return nil, errors.Wrap(err, "failed to check if VM is enabled")
	}

	adbdJobName := "arc-adbd"
	if vmEnabled {
		adbdJobName = "arcvm-adbd"
	}

	if !(upstart.JobExists(ctx, adbdJobName)) {
		return nil, errors.Wrapf(err, "Missing: %v job does not exist", adbdJobName)
	}

	if err := upstart.WaitForJobStatus(ctx, adbdJobName, upstartcommon.StartGoal, upstartcommon.RunningState, upstart.RejectWrongGoal, 1*time.Minute); err != nil {
		return nil, errors.Wrapf(err, "Failed: job %v is not up and running", adbdJobName)
	}
	return &empty.Empty{}, nil
}
