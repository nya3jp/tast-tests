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
	"chromiumos/tast/local/bundles/cros/platform/perfetto"
	"chromiumos/tast/testing"
)

const (
	// Trace data output in binary proto format.
	traceOutputFile = "perfetto_trace.pb"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     PerfettoSystemTracing,
		Desc:     "Verifies functions of Perfetto traced and traced_probes",
		Contacts: []string{"chinglinyu@chromium.org", "chromeos-performance-eng@google.com"},
		Data:     []string{perfetto.TraceConfigFile},
		Attr:     []string{"group:mainline"},
	})
}

// collectTraceData collect a system-wide trace using the perfetto command line
// too.
func collectTraceData(ctx context.Context, s *testing.State) error {
	// Trace config specifies a 5 sec duration. Use 20 sec to avoid premature timeout on slow devices.
	wctx, wcancel := context.WithTimeout(ctx, 20*time.Second)
	defer wcancel()

	// Start a trace session using the perfetto command line tool.
	traceOutputPath := filepath.Join(s.OutDir(), traceOutputFile)
	traceConfigPath := s.DataPath(perfetto.TraceConfigFile)
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

// PerfettoSystemTracing tests perfetto system-wide trace collection.
func PerfettoSystemTracing(ctx context.Context, s *testing.State) {
	// The tracing service daemons are started by default. Check their status.
	// Remember the PID of both jobs to verify that the jobs didn't have seccomp crash during trace collection.
	tracedPID, tracedProbesPID, err := perfetto.CheckTracingServices(ctx)
	if err != nil {
		s.Fatal("Tracing services not running: ", err)
	}

	if err := collectTraceData(ctx, s); err != nil {
		s.Fatal("Failed to collect trace data: ", err)
	}

	tracedPID2, tracedProbesPID2, err := perfetto.CheckTracingServices(ctx)
	if err != nil {
		s.Fatal("Tracing services not running after trace collection: ", err)
	}

	// Check that PID stays the same as a heuristic that the jobs didn't crash during the test.
	if tracedPID != tracedPID2 {
		s.Errorf("Unexpected respawn of job %s (PID changed from %d to %d)", perfetto.TracedJobName, tracedPID, tracedPID2)
	}
	if tracedProbesPID != tracedProbesPID2 {
		s.Errorf("Unexpected respawn of job %s (PID changed from %d to %d)", perfetto.TracedProbesJobName, tracedProbesPID, tracedProbesPID2)
	}
}
