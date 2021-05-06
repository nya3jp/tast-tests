// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"bytes"
	"context"
	"io"
	"time"

	"google.golang.org/grpc"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/services/cros/platform"
	"chromiumos/tast/testing"
)

// TODO(chenghaoyang): create a library for constants and helper functions
const (
	tracedJobExec       = "traced"
	tracedProbesJobExec = "traced_probes"
)

// funcWaitForRunning waits until |job| is up and running.
func funcWaitForRunning(ctx context.Context, job string) error {
	// Wait for the job to
	return testing.Poll(ctx, func(context.Context) error {
		return upstart.CheckJob(ctx, job)
	}, &testing.PollOptions{
		Timeout:  5 * time.Second,
		Interval: 500 * time.Millisecond,
	})
}

// funcGetRunningJobStatus checks the status of job traced and traced_probes.
// Returns the traced and traced_probes process IDs on success or an error if
// either job is not in the running state, or either jobs has crashed (and
// remains in the zombie process status.
func funcGetRunningJobStatus(ctx context.Context) (int, int, error) {
	// Use a short context for waiting job status.
	wctx, wcancel := context.WithTimeout(ctx, 15*time.Second)
	defer wcancel()

	// Ensure traced is not in the zombie process state.
	if err := upstart.CheckJob(wctx, tracedJobExec); err != nil {
		return 0, 0, err
	}
	// Get the PID of traced.
	_, _, tracedPid, err := upstart.JobStatus(wctx, tracedJobExec)
	if err != nil {
		return 0, 0, err
	}

	// Wait for traced_probes, which starts on traced started.
	if err := funcWaitForRunning(wctx, tracedProbesJobExec); err != nil {
		return 0, 0, err
	}

	// Get the PID of traced_probes.
	_, _, tracedProbesPid, err := upstart.JobStatus(wctx, tracedProbesJobExec)
	if err != nil {
		return 0, 0, err
	}

	return tracedPid, tracedProbesPid, nil
}

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			platform.RegisterPerfettoSystemTracingServiceServer(srv, &PerfettoSystemTracingService{s})
		},
	})
}

// collectTraceDataFromConfig collect a system-wide trace using the perfetto
// command line too.
func collectTraceDataFromConfig(ctx context.Context, config string) ([]byte, error) {
	// Trace config specifies a 5 sec duration. Use 20 sec to avoid premature timeout on slow devices.
	wctx, wcancel := context.WithTimeout(ctx, 20*time.Second)
	defer wcancel()

	// This runs a perfetto trace session with the options:
	//   -c - --txt: configure the trace session as defined in the stdin
	//   -o -      : send the trace data (binary proto) to stdout
	cmd := testexec.CommandContext(wctx, "/usr/bin/perfetto", "-c", "-", "--txt", "-o", "-")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, errors.Wrap(err, "failed to create stdin pipe")
	}

	go func() {
		defer stdin.Close()
		io.WriteString(stdin, config)
	}()

	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, errors.Wrap(err, "failed to run the tracing session")
	}
	index := bytes.IndexByte(out, byte('\n'))
	if index != -1 {
		out = out[index+1:]
	}

	return out, nil
}

// PerfettoSystemTracingService implements tast.cros.platform.PerfettoSystemTracingService
type PerfettoSystemTracingService struct {
	s *testing.ServiceState
}

// PerfettoSystemTracingService.GeneratePerfettoTrace uses perfetto to
// generate trace and send back to the host.
func (*PerfettoSystemTracingService) GeneratePerfettoTrace(ctx context.Context, req *platform.GeneratePerfettoTraceRequest) (*platform.GeneratePerfettoTraceResponse, error) {
	// return &empty.Empty{}, nil
	wctx, wcancel := context.WithTimeout(ctx, 10*time.Second)
	defer wcancel()
	// Make sure traced is running (and start it if not).
	if err := upstart.EnsureJobRunning(wctx, tracedJobExec); err != nil {
		return nil, errors.Wrapf(err, "Job %s isn't running", tracedJobExec)
	}

	_, _, err := funcGetRunningJobStatus(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "error in getting job status")
	}

	result, err := collectTraceDataFromConfig(ctx, req.Config)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to collect trace data")
	}

	return &platform.GeneratePerfettoTraceResponse{
		Result: result,
	}, nil
}
