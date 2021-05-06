// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/platform"
	"chromiumos/tast/testing"
)

const (
	perfettoConfig = `
	buffers: {
		size_kb: 63488
		fill_policy: DISCARD
	}
	buffers: {
		size_kb: 2048
		fill_policy: DISCARD
	}
	data_sources: {
		config {
			name: "linux.process_stats"
			process_stats_config {
				proc_stats_poll_ms: 1000
			}
		}
	}
	duration_ms: 10000`

	perfettoTraceResultPath = "perfetto_trace.pb"

	traceProcessorURL  = "https://storage.googleapis.com/perfetto/trace_processor_shell-linux-a3ce2cbf4cbe4f86cc10b02957db727cecfafae8"
	traceProcessorPath = "trace_processor"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PerfettoSystemTracingService,
		Desc:         "Verifies functions of Perfetto traced and traced_probes",
		Contacts:     []string{"chenghaoyang@chromium.org", "chinglinyu@chromium.org"},
		ServiceDeps:  []string{"tast.cros.platform.PerfettoSystemTracingService"},
		SoftwareDeps: []string{"chrome"},
	})
}

func PerfettoSystemTracingService(fullCtx context.Context, s *testing.State) {
	d := s.DUT()

	ctx, cancel := ctxutil.Shorten(fullCtx, 10*time.Second)
	defer cancel()

	cl, err := rpc.Dial(ctx, d, s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	pc := platform.NewPerfettoSystemTracingServiceClient(cl.Conn)
	response, err := pc.GeneratePerfettoTrace(ctx, &platform.GeneratePerfettoTraceRequest{Config: perfettoConfig})
	if err != nil {
		s.Fatal("Failed to call gRPC GeneratePerfettoTrace: ", err)
	}
	// Store pb into file for debug
	outputPath := filepath.Join(s.OutDir(), perfettoTraceResultPath)
	f, err := os.OpenFile(outputPath, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		s.Fatal("Failed to open the result file: ", err)
	}
	if _, err := f.Write(response.Result); err != nil {
		f.Close()
		s.Fatal("Failed to write the result to file: ", err)
	}
	if err := f.Close(); err != nil {
		s.Fatal("Failed to close the result file: ", err)
	}

	traceProcessorPath := filepath.Join(s.OutDir(), traceProcessorPath)
	cmd := testexec.CommandContext(ctx, "curl", "-L", "-#", "-o", traceProcessorPath, traceProcessorURL)
	if err := cmd.Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to download trace_processor: ", err)
	}
	if err := os.Chmod(traceProcessorPath, 0755); err != nil {
		s.Fatal("Failed to change trace_processor's run permission: ", err)
	}
	cmd2 := testexec.CommandContext(ctx, traceProcessorPath, outputPath, "--metrics-output=json", "--run-metrics", "android_mem")
	out, err := cmd2.Output()
	if err != nil {
		s.Fatal(err, string(out[:]))
	}
	if err = ioutil.WriteFile(filepath.Join(s.OutDir(), "trace_metrics.json"), out, 0644); err != nil {
		s.Fatal("Failed to save raw data: ", err)
	}

	result, _ := json.Marshal(string(out[:]))
	s.Log("result: ", string(result))
}
