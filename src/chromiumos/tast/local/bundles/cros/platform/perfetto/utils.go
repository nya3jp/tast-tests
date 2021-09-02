// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package perfetto

import (
	"context"
	"io/ioutil"
	"os"
	"runtime"
	"strings"

	"android.googlesource.com/platform/external/perfetto/protos/perfetto/metrics"
	"github.com/golang/protobuf/proto"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/upstart"
)

type TracingSession struct {
	cmd             *testexec.Cmd
	TraceResultFile *os.File
}

func createTempFileForTrace() (*os.File, error) {
	return ioutil.TempFile("", "perfetto-trace-*.pb")
}

// Stop stops the system-wide trace, which should be created by StartTracing.
func (sess *TracingSession) Stop() error {
	return sess.cmd.Wait()
}

// RunMetrics collects the result with trace_processor_shell.
func (sess *TracingSession) RunMetrics(ctx context.Context, traceProcessorPath string, metrics []string) (*perfetto_protos.TraceMetrics, error) {
	metric := strings.Join(metrics, ",")
	cmd := testexec.CommandContext(ctx, traceProcessorPath, sess.TraceResultFile.Name(), "--run-metrics", metric)
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

// RemoveTraceResultFile removes the temp file of trace result.
func (sess *TracingSession) RemoveTraceResultFile() {
	os.Remove(sess.TraceResultFile.Name())
}

// GetTraceProcessorByArch returns the TraceProcessor name could be used on the
// DUT's architecture.
// Developers should also add the TraceProcessor name in their tests' Data.
func GetTraceProcessorByArch() string {
	switch runtime.GOARCH {
	case "arm":
		return TraceProcessorArm
	case "arm64":
		return TraceProcessorArm64
	default:
		return TraceProcessorAmd64
	}
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

// StartTracing starts a system-wide trace using the perfetto command
// line tool in the background, and return the PID in string, which
// the caller should use to call StopTraceDataWithPID.
// On success, returns the temporary file of the trace data. It's the
// caller's responsibility for removing it if it's no longer needed.
func StartTracing(ctx context.Context, configFile string) (*TracingSession, error) {
	tempFile, err := createTempFileForTrace()
	if err != nil {
		return nil, errors.Wrap(err, "failed to create temp file")
	}

	// This runs a perfetto trace session with the options:
	//   -c traceConfigPath --txt: configure the trace session as defined in the text proto |traceConfigPath|
	//   -o traceOutputPath      : save the trace data (binary proto) to |traceOutputPath|
	cmd := testexec.CommandContext(ctx, "perfetto", "-c", configFile, "--txt", "-o", tempFile.Name())
	if err := cmd.Start(); err != nil {
		return nil, errors.Wrap(err, "failed to start the tracing session")
	}

	return &TracingSession{cmd: cmd, TraceResultFile: tempFile}, nil
}

// CollectTracing collects a system-wide trace using the perfetto
// command line tool.
// On success, returns the temporary file of the trace data. It's the
// caller's responsibility for removing it if it's no longer needed.
func CollectTracing(ctx context.Context, configFile string) (*TracingSession, error) {
	sess, err := StartTracing(ctx, configFile)
	if err != nil {
		return nil, err
	}

	if err := sess.Stop(); err != nil {
		return nil, errors.Wrap(err, "failed to stop the tracing session")
	}

	return sess, nil
}

// RunMetrics collects the result with trace_processor_shell.
func RunMetrics(ctx context.Context, traceProcessorPath string, outputPath string, metrics []string) (*perfetto_protos.TraceMetrics, error) {
	metric := strings.Join(metrics, ",")
	cmd := testexec.CommandContext(ctx, traceProcessorPath, outputPath, "--run-metrics", metric)
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
