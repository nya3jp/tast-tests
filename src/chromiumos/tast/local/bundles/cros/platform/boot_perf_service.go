// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"math"
	"strings"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/local/bundles/cros/platform/bootperf"
	"chromiumos/tast/services/cros/platform"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			platform.RegisterBootPerfServiceServer(srv, &BootPerfService{s})
		},
	})
}

// BootPerfService implements tast.cros.platform.BootPerfService
type BootPerfService struct {
	s *testing.ServiceState
}

// EnableBootchart enables bootchart by adding "cros_bootchart" to kernel
// arguments.
func (*BootPerfService) EnableBootchart(ctx context.Context, _ *empty.Empty) (*empty.Empty, error) {
	if err := bootperf.EnableBootchart(ctx); err != nil {
		return nil, err
	}

	return &empty.Empty{}, nil
}

// DisableBootchart Disables bootchart by removing "cros_bootchart" from kernel
// arguments.
func (*BootPerfService) DisableBootchart(ctx context.Context, _ *empty.Empty) (*empty.Empty, error) {
	if err := bootperf.DisableBootchart(ctx); err != nil {
		return nil, err
	}

	return &empty.Empty{}, nil
}

// GetBootPerfMetrics gathers recorded timing and disk usage statistics during
// boot time. The test calculates some or all of the following metrics:
//   * seconds_kernel_to_startup
//   * seconds_kernel_to_startup_done
//   * seconds_kernel_to_chrome_exec
//   * seconds_kernel_to_chrome_main
//   * seconds_kernel_to_signin_start
//   * seconds_kernel_to_signin_wait
//   * seconds_kernel_to_signin_users
//   * seconds_kernel_to_login
//   * seconds_kernel_to_network
//   * seconds_startup_to_chrome_exec
//   * seconds_chrome_exec_to_login
//   * rdbytes_kernel_to_startup
//   * rdbytes_kernel_to_startup_done
//   * rdbytes_kernel_to_chrome_exec
//   * rdbytes_kernel_to_chrome_main
//   * rdbytes_kernel_to_login
//   * rdbytes_startup_to_chrome_exec
//   * rdbytes_chrome_exec_to_login
//   * seconds_power_on_to_kernel
//   * seconds_power_on_to_login
//   * seconds_shutdown_time
//   * seconds_reboot_time
//   * seconds_reboot_error
func (*BootPerfService) GetBootPerfMetrics(ctx context.Context, _ *empty.Empty) (*platform.GetBootPerfMetricsResponse, error) {
	out := &platform.GetBootPerfMetricsResponse{
		Metrics: make(map[string]float64),
	}

	// perform a testing.Poll() to wait for boot perf artifacts to show up.
	testing.ContextLog(ctx, "Wait until boot complete")
	err := bootperf.WaitUntilBootComplete(ctx)
	if err != nil {
		return nil, err
	}

	testing.ContextLog(ctx, "Gather boot time metrics")
	err = bootperf.GatherTimeMetrics(ctx, out)
	if err != nil {
		return nil, err
	}

	testing.ContextLog(ctx, "Gather boot disk read metrics")
	bootperf.GatherDiskMetrics(out)

	testing.ContextLog(ctx, "Gather firmware boot metric")
	err = bootperf.GatherFirmwareBootTime(out)
	if err != nil {
		return nil, err
	}

	testing.ContextLog(ctx, "Gather reboot metrics")
	err = bootperf.GatherRebootMetrics(out)
	if err != nil {
		return nil, err
	}

	testing.ContextLog(ctx, "Calculate diff")
	bootperf.CalculateDiff(out)

	// Round the seconds_* values for nicer presentation.
	for key, value := range out.Metrics {
		if strings.HasPrefix(key, "seconds_") {
			out.Metrics[key] = math.Round(value*1000) / 1000
		}
	}

	return out, nil
}

// GetBootPerfRawData gathers raw data used in calculating boot perf metrics for
// debugging.
func (*BootPerfService) GetBootPerfRawData(ctx context.Context, _ *empty.Empty) (*platform.GetBootPerfRawDataResponse, error) {
	// Passed cached bootstat raw data to the client.
	raw := make(map[string][]byte)

	if err := bootperf.GatherMetricRawDataFiles(raw); err != nil {
		return nil, err
	}
	if err := bootperf.GatherConsoleRamoops(raw); err != nil {
		return nil, err
	}

	return &platform.GetBootPerfRawDataResponse{
		RawData: raw,
	}, nil
}
