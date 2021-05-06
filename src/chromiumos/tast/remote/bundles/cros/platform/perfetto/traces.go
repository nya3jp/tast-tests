// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package perfetto

import (
	"context"
	"io/ioutil"
	"strings"

	"github.com/golang/protobuf/proto"
	grpc "google.golang.org/grpc"

	tppb "chromiumos/perfetto/trace_processor"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/services/cros/platform"
	"chromiumos/tast/testing"
)

const (
	// TraceProcessor is retrieved from the scripe in https://get.perfetto.dev/trace_processor, with "os" being "linux" and "arch" being "x86_64". We download trace_processor_shell from gs bucket "perfetto". Update the external data file correspondingly when we need to uprev trace_processor_shell.
	TraceProcessor = "trace_processor_shell-linux-a3ce2cbf4cbe4f86cc10b02957db727cecfafae8"
)

// RunPerfetto uses gRPC to run perfetto cmdline with |traceConfigFile| in the DUT.
func RunPerfetto(ctx context.Context, s *testing.State, pc *platform.PerfettoTraceBasedMetricsServiceClient, traceConfigFile string, maxMsgSizeInBytes int) string {
	traceConfigPath := s.DataPath(traceConfigFile)
	config, err := ioutil.ReadFile(traceConfigPath)
	if err != nil {
		s.Fatal("Failed to read config file: ", err)
	}

	response, err := (*pc).GeneratePerfettoTrace(ctx, &platform.GeneratePerfettoTraceRequest{Config: string(config)}, grpc.MaxCallRecvMsgSize(maxMsgSizeInBytes))
	if err != nil {
		s.Fatal("Failed to call gRPC GeneratePerfettoTrace: ", err)
	}

	cmd := testexec.CommandContext(ctx, "mktemp", "/tmp/perfetto-trace-XXXXXX.pb")
	outputPath, err := cmd.Output()
	if err != nil {
		s.Fatal("Failed to create temp file: ", err)
	}

	if err := ioutil.WriteFile(string(outputPath), response.Result, 0644); err != nil {
		s.Fatal("Failed to write the result to file: ", err)
	}

	return string(outputPath)
}

// RunMetric collects the result with trace_processor_shell.
func RunMetric(ctx context.Context, s *testing.State, outputPath string, metrics []string) *tppb.TraceMetrics {
	traceProcessorPath := s.DataPath(TraceProcessor)
	metric := strings.Join(metrics, ",")
	cmd := testexec.CommandContext(ctx, traceProcessorPath, outputPath, "--run-metrics", metric)
	out, err := cmd.Output()
	if err != nil {
		s.Fatal(err, string(out[:]))
	}

	tbm := &tppb.TraceMetrics{}
	if err := proto.UnmarshalText(string(out), tbm); err != nil {
		s.Fatal("Failed to unmarshal cpu result: ", err)
	}

	return tbm
}
