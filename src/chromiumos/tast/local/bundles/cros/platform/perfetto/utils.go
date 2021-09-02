// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package perfetto

import (
	"context"
	"io/ioutil"
	"os"
	"strconv"
	"strings"

	"android.googlesource.com/platform/external/perfetto/protos/perfetto/metrics"
	"github.com/golang/protobuf/proto"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/upstart"
)

func createTempFileForTrace() (*os.File, error) {
	return ioutil.TempFile("", "perfetto-trace-*.pb")
}

// CheckTracingServices checks the status of job traced and traced_probes.
// Returns the traced and traced_probes process IDs on success or an error if
// either job is not in the running state, or either jobs has crashed (and
// remains in the zombie process status.
func CheckTracingServices(ctx context.Context) (tracedPID, tracedProbesPID int, err error) {
	// Ensure traced is not in the zombie process state.
	if err = upstart.CheckJob(ctx, TracedJobName); err != nil {
		return 0, 0, err
	}
	// Get the PID of traced.
	if _, _, tracedPID, err = upstart.JobStatus(ctx, TracedJobName); err != nil {
		return 0, 0, err
	}

	if err = upstart.CheckJob(ctx, TracedProbesJobName); err != nil {
		return 0, 0, err
	}
	// Get the PID of traced_probes.
	if _, _, tracedProbesPID, err = upstart.JobStatus(ctx, TracedProbesJobName); err != nil {
		return 0, 0, err
	}

	return
}

// CollectTraceDataWithConfig collects a system-wide trace using the
// perfetto command line tool.
// On success, returns the temporary file of the trace data. It's the
// caller's responsibility for removing it if it's no longer needed.
func CollectTraceDataWithConfig(ctx context.Context, configFile string) (*os.File, error) {
	tempFile, err := createTempFileForTrace()
	if err != nil {
		return nil, errors.Wrap(err, "failed to create temp file")
	}

	// This runs a perfetto trace session with the options:
	//   -c traceConfigPath --txt: configure the trace session as defined in the text proto |traceConfigPath|
	//   -o traceOutputPath      : save the trace data (binary proto) to |traceOutputPath|
	cmd := testexec.CommandContext(ctx, "perfetto", "-c", configFile, "--txt", "-o", tempFile.Name())
	if err := cmd.Run(testexec.DumpLogOnError); err != nil {
		return nil, errors.Wrap(err, "failed to run the tracing session")
	}

	return tempFile, nil
}

// StartTraceDataWithConfig starts a system-wide trace using the
// perfetto command line tool in the background, and return the PID in
// string, which the caller should use to call StopTraceDataWithPID.
// On success, returns the temporary file of the trace data. It's the
// caller's responsibility for removing it if it's no longer needed.
func StartTraceDataWithConfig(ctx context.Context, configFile string) (int, *os.File, error) {
	tempFile, err := createTempFileForTrace()
	if err != nil {
		return -1, nil, errors.Wrap(err, "failed to create temp file")
	}

	// This runs a perfetto trace session with the options:
	//   --background            : Exits immediately and continues in the background.
	//                             Prints the PID of the bg process.
	//   -c traceConfigPath --txt: configure the trace session as defined in the text proto |traceConfigPath|
	//   -o traceOutputPath      : save the trace data (binary proto) to |tempFile|
	cmd := testexec.CommandContext(ctx, "perfetto", "--background", "-c", configFile, "--txt", "-o", tempFile.Name())
	out, err := cmd.Output()
	if err != nil {
		return -1, nil, errors.Wrap(err, "failed to start the tracing session")
	}

	outInString := strings.TrimSuffix(string(out), "\n")
	PID, err := strconv.Atoi(outInString)
	if err != nil {
		return -1, nil, errors.Wrap(err, "failed to parse PID")
	}

	return PID, tempFile, nil
}

// StopTraceDataWithPID stops the system-wide trace with the PID,
// which should be returned by StartTraceDataWithConfig.
func StopTraceDataWithPID(ctx context.Context, PID int) {
	cmd := testexec.CommandContext(ctx, "kill", "-TERM", strconv.Itoa(PID))
	cmd.Run()
}

// RunMetrics collects the result with trace_processor_shell.
func RunMetrics(ctx context.Context, outputPath string, metrics []string) (*perfetto_protos.TraceMetrics, error) {
	metric := strings.Join(metrics, ",")
	cmd := testexec.CommandContext(ctx, "trace_processor_shell", outputPath, "--run-metrics", metric)
	out, err := cmd.Output(testexec.DumpLogOnError)
	if err != nil {
		return nil, errors.Wrap(err, "failed to run metrics with trace_processor_shell")
	}

	tbm := &perfetto_protos.TraceMetrics{}
	if err := proto.UnmarshalText(string(out), tbm); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal metrics result")
	}

	return tbm, nil
}
