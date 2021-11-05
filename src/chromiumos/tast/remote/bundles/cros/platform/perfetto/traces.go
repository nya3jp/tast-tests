// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package perfetto

import (
	"context"
	"io"
	"io/ioutil"
	"os"
	"strings"

	"android.googlesource.com/platform/external/perfetto/protos/perfetto/metrics/github.com/google/perfetto/perfetto_proto"
	"github.com/golang/protobuf/proto"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/services/cros/platform"
)

const (
	// TraceProcessor is retrieved from the script in
	// https://get.perfetto.dev/trace_processor, with "os" being "linux"
	// and "arch" being "x86_64".
	// We download trace_processor_shell from gs bucket "perfetto".
	// Update the external data file correspondingly when we need to
	// uprev trace_processor_shell.
	TraceProcessor = "trace_processor_shell-linux-a3ce2cbf4cbe4f86cc10b02957db727cecfafae8"
)

// RunPerfetto uses gRPC to run perfetto cmdline with
// |traceConfigFile| in the DUT.
func RunPerfetto(ctx context.Context, pc platform.PerfettoTraceBasedMetricsServiceClient, traceConfigPath string) (ret string, retErr error) {
	config, err := ioutil.ReadFile(traceConfigPath)
	if err != nil {
		return "", errors.Wrap(err, "failed to read config file")
	}

	stream, err := pc.GeneratePerfettoTrace(ctx, &platform.GeneratePerfettoTraceRequest{Config: string(config)})
	if err != nil {
		return "", errors.Wrap(err, "failed to call gRPC GeneratePerfettoTrace")
	}

	tempFile, err := ioutil.TempFile("/tmp", "perfetto-trace-*.pb")
	if err != nil {
		return "", errors.Wrap(err, "failed to create temp file")
	}

	defer func() {
		if err := tempFile.Close(); err != nil && retErr == nil {
			ret = ""
			retErr = err
		}
		if retErr != nil {
			os.Remove(tempFile.Name())
		}
	}()

	for {
		res, err := stream.Recv()
		if err == io.EOF {
			return tempFile.Name(), nil
		}
		if err != nil {
			return "", errors.Wrap(err, "failed to receive from the stream")
		}
		if _, err := tempFile.Write(res.Result); err != nil {
			return "", errors.Wrap(err, "failed to write to temp file")
		}
	}
}

// RunMetrics collects the result with trace_processor_shell.
func RunMetrics(ctx context.Context, traceProcessorPath, outputPath string, metrics []string) (*perfetto_proto.TraceMetrics, error) {
	metric := strings.Join(metrics, ",")
	cmd := testexec.CommandContext(ctx, traceProcessorPath, outputPath, "--run-metrics", metric)
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
