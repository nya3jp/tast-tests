// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"io"

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

// collectTraceDataFromConfig collect a system-wide trace using the perfetto
// command line tool.
func collectTraceDataFromConfig(ctx context.Context, config string) ([]byte, error) {
	wctx, wcancel := context.WithCancel(ctx)
	defer wcancel()

	// This runs a perfetto trace session with the options:
	//   -c - --txt: configure the trace session as defined in the stdin
	//   -o -      : send the trace data (binary proto) to stdout
	cmd := testexec.CommandContext(wctx, "/usr/bin/perfetto", "-c", "-", "--txt", "-o", "-")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, errors.Wrap(err, "failed to create stdin pipe")
	}

	if _, err := io.WriteString(stdin, config); err != nil {
		return nil, errors.Wrap(err, "failed to write config to stdin")
	}
	stdin.Close()

	out, err := cmd.Output()
	if err != nil {
		return nil, errors.Wrap(err, "failed to run the tracing session")
	}
	testing.ContextLog(ctx, string(out[:]))

	return out, nil
}

// PerfettoTraceBasedMetricsService implements tast.cros.platform.PerfettoTraceBasedMetricsService
type PerfettoTraceBasedMetricsService struct {
	s *testing.ServiceState
}

// GeneratePerfettoTrace uses perfetto to generate trace and send back to the
// host.
func (*PerfettoTraceBasedMetricsService) GeneratePerfettoTrace(ctx context.Context, req *platform.GeneratePerfettoTraceRequest) (*platform.GeneratePerfettoTraceResponse, error) {
	result, err := collectTraceDataFromConfig(ctx, req.Config)
	if err != nil {
		return nil, errors.Wrap(err, "failed to collect trace data")
	}

	return &platform.GeneratePerfettoTraceResponse{
		Result: result,
	}, nil
}
