// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

const (
	tracedJob       = "traced"
	tracedProbesJob = "traced_probes"
	waitDuration    = 10 * time.Second

	// Trace config file in text proto format.
	traceConfigFile = "perfetto/system_trace_cfg.pbtxt"
	// Trace data output in binary proto format.
	traceOutputFile = "perfetto_trace.pb"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     PerfettoSystemTracing,
		Desc:     "Verifies functions of Perfetto traced and traced_probes",
		Contacts: []string{"chinglinyu@chromium.org", "chromeos-performance-eng@google.com"},
		Data:     []string{traceConfigFile},
		Attr:     []string{"group:mainline", "informational"}, // TODO(chinglinyu) remove informational once perfetto is landed
	})
}

// ensureJobsStopped stops traced and makes sure traced_probes is also stopped.
func ensureJobsStopped(ctx context.Context) error {
	// Use a short context for waiting job status.
	wctx, wcancel := context.WithTimeout(ctx, 10*time.Second)
	defer wcancel()

	// Stop traced.
	if err := upstart.StopJob(wctx, tracedJob); err != nil {
		return errors.Wrap(err, "failed to stop the traced job")
	}

	// Check that traced_probes is also stopped with traced.
	if err := upstart.WaitForJobStatus(wctx, tracedProbesJob, upstart.StopGoal, upstart.WaitingState, upstart.RejectWrongGoal, waitDuration); err != nil {
		return errors.Wrap(err, "the traced_probes job isn't stopped")
	}

	return nil
}

// collectTraceData collect a system-wide trace using the perfetto command line
// too.
func collectTraceData(ctx context.Context, s *testing.State) error {
	// Trace config specifies a 5 sec duration. Use 20 sec to avoid premature timeout on slow devices.
	wctx, wcancel := context.WithTimeout(ctx, 20*time.Second)
	defer wcancel()

	// Start a trace session using the perfetto command line tool.
	traceOutputPath := filepath.Join(s.OutDir(), traceOutputFile)
	traceConfigPath := s.DataPath(traceConfigFile)
	// This runs a perfetto trace session with the options:
	//   -c traceConfigPath --txt: configure the trace session as defined in the text proto |traceConfigPath|
	//   -o traceOutputPath      : save the trace data (binary proto) to |traceOutputPath|
	cmd := testexec.CommandContext(wctx, "/usr/bin/perfetto", "-c", traceConfigPath, "--txt", "-o", traceOutputPath)
	if err := cmd.Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to run the tracing session")
	}

	// Validate the trace data.
	stat, err := os.Stat(traceOutputPath)
	if err != nil {
		return errors.Wrapf(err, "unexpected error stating %s", traceOutputPath)
	}
	s.Logf("Collected %d bytes of trace data", stat.Size())
	// TODO(chinglinyu): really validate the trace data content.
	return nil
}

// waitForRunning waits until |job| is up and running.
func waitForRunning(ctx context.Context, job string) error {
	// Wait for the job to
	return testing.Poll(ctx, func(context.Context) error {
		return upstart.CheckJob(ctx, job)
	}, &testing.PollOptions{
		Timeout:  5 * time.Second,
		Interval: 500 * time.Millisecond,
	})
}

// getRunningJobStatus checks the status of job traced and traced_probes.
// Returns the traced and traced_probes process IDs on success or an error if
// either job is not in the running state, or either jobs has crashed (and
// remains in the zombie process status.
func getRunningJobStatus(ctx context.Context) (int, int, error) {
	// Use a short context for waiting job status.
	wctx, wcancel := context.WithTimeout(ctx, 15*time.Second)
	defer wcancel()

	// Ensure traced is not in the zombie process state.
	if err := upstart.CheckJob(wctx, tracedJob); err != nil {
		return 0, 0, err
	}
	// Get the PID of traced.
	_, _, tracedPid, err := upstart.JobStatus(wctx, tracedJob)
	if err != nil {
		return 0, 0, err
	}

	// Wait for traced_probes, which starts on traced started.
	if err := waitForRunning(wctx, tracedProbesJob); err != nil {
		return 0, 0, err
	}

	// Get the PID of traced_probes.
	_, _, tracedProbesPid, err := upstart.JobStatus(wctx, tracedProbesJob)
	if err != nil {
		return 0, 0, err
	}

	return tracedPid, tracedProbesPid, nil
}

// PerfettoSystemTracing tests perfetto system-wide trace collection.
func PerfettoSystemTracing(ctx context.Context, s *testing.State) {
	wctx, wcancel := context.WithTimeout(ctx, 10*time.Second)
	defer wcancel()
	// Make sure traced is running (and start it if not).
	if err := upstart.EnsureJobRunning(wctx, tracedJob); err != nil {
		s.Fatalf("Job %s isn't running", tracedJob)
	}
	defer func() {
		if err := ensureJobsStopped(ctx); err != nil {
			s.Fatal("Error in stopping the jobs: ", err)
		}
	}()

	// Remember the PID of both jobs to verify that the jobs didn't have seccomp crash during trace collection.
	tracedPid, tracedProbesPid, err := getRunningJobStatus(ctx)
	if err != nil {
		s.Fatal("Error in getting job status: ", err)
	}

	if err := collectTraceData(ctx, s); err != nil {
		s.Fatal("Failed to collect trace data: ", err)
	}

	tracedPid2, tracedProbesPid2, err := getRunningJobStatus(ctx)
	if err != nil {
		s.Fatal("Error in getting job status after trace collection: ", err)
	}

	// Check thhat PID stays the same as a heuristic that the jobs didn't crash during the test.
	if tracedPid != tracedPid2 {
		s.Errorf("Unexpected respawn of job %s (PID changed from %d to %d)", tracedJob, tracedPid, tracedPid2)
	}
	if tracedProbesPid != tracedProbesPid2 {
		s.Errorf("Unexpected respawn of job %s (PID changed from %d to %d)", tracedProbesJob, tracedProbesPid, tracedProbesPid2)
	}
}
