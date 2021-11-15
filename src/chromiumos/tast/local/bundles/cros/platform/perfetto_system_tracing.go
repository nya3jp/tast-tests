// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/tracing"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     PerfettoSystemTracing,
		Desc:     "Verifies functions of Perfetto traced and traced_probes",
		Contacts: []string{"chinglinyu@chromium.org", "chromeos-performance-eng@google.com"},
		Data:     []string{tracing.TraceConfigFile},
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
	traceConfigPath := s.DataPath(tracing.TraceConfigFile)

	sess, err := tracing.StartSessionAndWaitUntilDone(wctx, traceConfigPath)
	// The temporary file of trace data is no longer needed when returned.
	defer sess.RemoveTraceResultFile()

	if err != nil {
		return err
	}

	// Validate the trace data.
	stat, err := sess.TraceResultFile.Stat()
	if err != nil {
		return errors.Wrapf(err, "unexpected error stating %s", sess.TraceResultFile.Name())
	}
	s.Logf("Collected %d bytes of trace data", stat.Size())

	return nil
}

// PerfettoSystemTracing tests perfetto system-wide trace collection.
func PerfettoSystemTracing(ctx context.Context, s *testing.State) {
	// The tracing service daemons are started by default. Check their status.
	// Remember the PID of both jobs to verify that the jobs didn't have seccomp crash during trace collection.
	tracedPID, tracedProbesPID, err := tracing.CheckTracingServices(ctx)
	if err != nil {
		s.Fatal("Tracing services not running: ", err)
	}

	if err := collectTraceData(ctx, s); err != nil {
		s.Fatal("Failed to collect trace data: ", err)
	}

	tracedPID2, tracedProbesPID2, err := tracing.CheckTracingServices(ctx)
	if err != nil {
		s.Fatal("Tracing services not running after trace collection: ", err)
	}

	// Check that PID stays the same as a heuristic that the jobs didn't crash during the test.
	if tracedPID != tracedPID2 {
		s.Errorf("Unexpected respawn of job %s (PID changed from %d to %d)", tracing.TracedJobName, tracedPID, tracedPID2)
	}
	if tracedProbesPID != tracedProbesPID2 {
		s.Errorf("Unexpected respawn of job %s (PID changed from %d to %d)", tracing.TracedProbesJobName, tracedProbesPID, tracedProbesPID2)
	}
}
