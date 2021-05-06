// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"bytes"
	"context"
	"encoding/json"
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
	perfettoConfig          = "buffers: {\nsize_kb: 63488\nfill_policy: DISCARD\n}\nbuffers: {\nsize_kb: 2048\nfill_policy: DISCARD\n}\ndata_sources: {\nconfig {\nname: \"linux.process_stats\"\nprocess_stats_config {\nproc_stats_poll_ms: 1000\n}\n}\n}\nduration_ms: 10000"
	perfettoTraceResultPath = "perfetto_trace.pb"

	traceProcessorUrl  = "https://get.perfetto.dev/trace_processor"
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
		s.Fatal("failed to open the result file", err)
	}
	if _, err := f.Write(response.Result); err != nil {
		f.Close()
		s.Fatal("failed to write the result to file", err)
	}
	if err := f.Close(); err != nil {
		s.Fatal("failed to close the result file", err)
	}

	traceProcessorPath := filepath.Join(s.OutDir(), traceProcessorPath)
	cmd := testexec.CommandContext(ctx, "curl", "-L", "-o", traceProcessorPath, traceProcessorUrl)
	if err := cmd.Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("failed to download trace_processor", err)
	}
	if err := os.Chmod(traceProcessorPath, 0755); err != nil {
		s.Fatal("failed to change trace_processor's run permission", err)
	}
	cmd2 := testexec.CommandContext(ctx, traceProcessorPath, outputPath, "--metrics-output=json", "--run-metrics", "android_mem")
	out, err := cmd2.CombinedOutput()
	if err != nil {
		s.Fatal(err, string(out[:]))
	}
	index := bytes.IndexByte(out, byte('\n'))
	// Ignore the first two lines
	if index != -1 {
		out = out[index+1:]
		index = bytes.IndexByte(out, byte('\n'))
		if index != -1 {
			out = out[index+1:]
		}
	}

	result, _ := json.Marshal(string(out[:]))
	s.Log("result: ", string(result))
}
