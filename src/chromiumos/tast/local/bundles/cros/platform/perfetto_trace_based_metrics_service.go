// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"bytes"
	"context"

	"google.golang.org/grpc"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/services/cros/platform"
	"chromiumos/tast/testing"
)

const (
	tracedJobExec       = "traced"
	tracedProbesJobExec = "traced_probes"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			platform.RegisterPerfettoTraceBasedMetricsServiceServer(srv, &PerfettoTraceBasedMetricsService{s})
		},
	})
}

// collectTraceDataFromConfig collect a system-wide trace using the
// perfetto command line tool.
func collectTraceDataFromConfig(ctx context.Context, config string) ([]byte, error) {
	// This runs a perfetto trace session with the options:
	//   -c - --txt: configure the trace session as defined in the stdin
	//   -o -      : send the trace data (binary proto) to stdout
	cmd := testexec.CommandContext(ctx, "perfetto", "-c", "-", "--txt", "-o", "-")
	cmd.Stdin = bytes.NewBufferString(config)
	out, err := cmd.Output()
	if err != nil {
		return nil, errors.Wrap(err, "failed to run the tracing session")
	}

	return out, nil
}

// PerfettoTraceBasedMetricsService implements
// tast.cros.platform.PerfettoTraceBasedMetricsService
type PerfettoTraceBasedMetricsService struct {
	s *testing.ServiceState
}

// GeneratePerfettoTrace uses perfetto to generate trace and send
// back to the host.
func (*PerfettoTraceBasedMetricsService) GeneratePerfettoTrace(req *platform.GeneratePerfettoTraceRequest, sender platform.PerfettoTraceBasedMetricsService_GeneratePerfettoTraceServer) error {
	ctx := sender.Context()

	result, err := collectTraceDataFromConfig(ctx, req.Config)
	if err != nil {
		return errors.Wrap(err, "failed to collect trace data")
	}

	const packetSize = 1024 * 1024
	for start := 0; start < len(result); start += packetSize {
		end := start + packetSize
		if end > len(result) {
			end = len(result)
		}
		if err := sender.Send(&platform.GeneratePerfettoTraceResponse{Result: result[start:end]}); err != nil {
			return err
		}
	}
	return nil
}
