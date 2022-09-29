// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package tracing

import (
	"bytes"
	"context"
	"encoding/csv"
	"io/ioutil"
	"log"
	"os"
	"runtime"
	"strings"

	"android.googlesource.com/platform/external/perfetto/protos/perfetto/metrics/github.com/google/perfetto/perfetto_proto"
	"github.com/golang/protobuf/proto"
	"golang.org/x/sys/unix"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/upstart"
)

// Session stores the cmd and the result file of the trace.
// Remember to call Session.RemoveTraceResultFile to clean up the
// temporary file.
type Session struct {
	cmd             *testexec.Cmd
	TraceResultFile *os.File
}

func createTempFileForTrace() (*os.File, error) {
	return ioutil.TempFile("", "perfetto-trace-*.pb")
}

// Stop stops the system-wide trace, which should be created by StartSession.
// Note that the session shouldn't be stopped too early (like in 500
// milliseconds), so that perfetto_cmd has time to register the signal handler
// to handle SIGTERM properly.
func (sess *Session) Stop() error {
	if err := sess.cmd.Signal(unix.SIGTERM); err != nil {
		return errors.Wrap(err, "failed to terminate the tracing session")
	}

	return sess.Wait()
}

// Wait waits until the tracing session is done, which should be created
// by StartSession.
func (sess *Session) Wait() error {
	return sess.cmd.Wait()
}

// RunMetrics collects the result with trace_processor_shell.
func (sess *Session) RunMetrics(ctx context.Context, traceProcessorPath string, metrics []string) (*perfetto_proto.TraceMetrics, error) {
	metric := strings.Join(metrics, ",")
	cmd := testexec.CommandContext(ctx, traceProcessorPath, sess.TraceResultFile.Name(), "--run-metrics", metric)
	out, err := cmd.Output(testexec.DumpLogOnError)
	if err != nil {
		return nil, errors.Wrap(err, "failed to run metrics with trace_processor_shell")
	}

	tbm := &perfetto_proto.TraceMetrics{}
	if err := proto.UnmarshalText(string(out), tbm); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal metrics result")
	}

	return tbm, nil
}

// RunQueryString processes the trace data with a SQL query string and returns the query csv result as [][]string.
func (sess *Session) RunQueryString(ctx context.Context, traceProcessorPath, query string) ([][]string, error) {
	// trace_processor_shell accepts the SQL query as a file. Create a temp query file.
	queryFile, err := ioutil.TempFile("", "trace_processor_query_*.sql")
	if err != nil {
		return nil, errors.Wrap(err, "failed to create the temp SQL query file")
	}
	defer func() {
		if err := os.Remove(queryFile.Name()); err != nil {
			log.Printf("failed to remove the temporary trace result file: %v", err)
		}
	}()

	if _, err := queryFile.WriteString(query); err != nil {
		return nil, errors.Wrap(err, "failed to create the temp SQL query file")
	}

	return sess.RunQuery(ctx, traceProcessorPath, queryFile.Name())
}

// RunQuery processes the trace data with a SQL query and returns the query csv result as [][]string.
func (sess *Session) RunQuery(ctx context.Context, traceProcessorPath, queryPath string) ([][]string, error) {
	cmd := testexec.CommandContext(ctx, traceProcessorPath, sess.TraceResultFile.Name(), "-q", queryPath)
	out, err := cmd.Output(testexec.DumpLogOnError)
	if err != nil {
		return nil, errors.Wrap(err, "failed to run metrics with trace_processor_shell")
	}

	// trace_procesor_shell query output is in csv.
	csv := csv.NewReader(bytes.NewReader(out))
	// Return the query csv result as [][]string.
	return csv.ReadAll()
}

// RemoveTraceResultFile removes the temp file of trace result.
func (sess *Session) RemoveTraceResultFile() {
	if err := os.Remove(sess.TraceResultFile.Name()); err != nil {
		log.Printf("failed to remove the temporary trace result file: %v", err)
	}
}

// TraceProcessor returns the TraceProcessor name could be used on the DUT's
// architecture.
// Developers should also add the TraceProcessor name in their tests' Data.
func TraceProcessor() string {
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

	return tracedPID, tracedProbesPID, nil
}

// StartSession starts a system-wide trace using the perfetto command
// line tool in the background, and return the PID in string, which
// the caller should use to call StopTraceDataWithPID.
// On success, returns the temporary file of the trace data. It's the
// caller's responsibility for removing it if it's no longer needed.
func StartSession(ctx context.Context, configFile string) (*Session, error) {
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

	return &Session{cmd: cmd, TraceResultFile: tempFile}, nil
}

// StartSessionAndWaitUntilDone collects a system-wide trace using the
// perfetto command line tool.
// On success, returns the temporary file of the trace data. It's the
// caller's responsibility for removing it if it's no longer needed.
func StartSessionAndWaitUntilDone(ctx context.Context, configFile string) (*Session, error) {
	sess, err := StartSession(ctx, configFile)
	if err != nil {
		return nil, err
	}

	if err := sess.Wait(); err != nil {
		return nil, errors.Wrap(err, "failed to stop the tracing session")
	}

	return sess, nil
}
