// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/golang/protobuf/proto"

	tppb "chromiumos/perfetto/trace_processor"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/platform"
	"chromiumos/tast/testing"
)

const (
	traceConfigFileCPU = "perfetto/perfetto_tbm_traced_probes_cpu.pbtxt"

	traceConfigFileMEM = "perfetto/perfetto_tbm_traced_probes_mem.pbtxt"

	// This URL is retrieved from the scripe in https://get.perfetto.dev/trace_processor, with "os" being "linux" and "arch" being "x86_64". Update the URL correspondingly when we need to uprev trace_processor_shell.
	traceProcessorURL = "https://storage.googleapis.com/perfetto/trace_processor_shell-linux-a3ce2cbf4cbe4f86cc10b02957db727cecfafae8"
	traceProcessor    = "trace_processor"

	traceMetricCPU = "android_cpu"
	traceMetricMEM = "android_mem"

	targetProcessName = "/usr/bin/traced_probes"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:        PerfettoTBMTracedProbes,
		Desc:        "Verifies functions of Perfetto traced and traced_probes",
		Contacts:    []string{"chenghaoyang@chromium.org", "chinglinyu@chromium.org"},
		Data:        []string{traceConfigFileCPU, traceConfigFileMEM},
		ServiceDeps: []string{"tast.cros.platform.PerfettoTraceBasedMetricsService"},
	})
}

// RunPerfettoAndRunMetric uses gRPC to run perfetto cmdline with |traceConfigFile| in the DUT and collect the result with trace_processor_shell.
func RunPerfettoAndRunMetric(ctx context.Context, pc platform.PerfettoTraceBasedMetricsServiceClient, traceProcessorPath string, traceConfigFile string, metric string, s *testing.State) *tppb.TraceMetrics {
	traceConfigPath := s.DataPath(traceConfigFile)
	config, err := ioutil.ReadFile(traceConfigPath)
	if err != nil {
		s.Fatal("Failed to read config file: ", err)
	}

	response, err := pc.GeneratePerfettoTrace(ctx, &platform.GeneratePerfettoTraceRequest{Config: string(config)})
	if err != nil {
		s.Fatal("Failed to call gRPC GeneratePerfettoTrace: ", err)
	}
	// Store pb into file for debug
	outputPath := filepath.Join(s.OutDir(), metric+".pb")
	if err := ioutil.WriteFile(outputPath, response.Result, 0644); err != nil {
		s.Fatal("Failed to write the result to file: ", err)
	}

	cmd := testexec.CommandContext(ctx, traceProcessorPath, outputPath, "--run-metrics", metric)
	out, err := cmd.Output()
	if err != nil {
		s.Fatal(err, string(out[:]))
	}
	if err = ioutil.WriteFile(filepath.Join(s.OutDir(), metric), out, 0644); err != nil {
		s.Fatal("Failed to save raw data: ", err)
	}

	metrics := &tppb.TraceMetrics{}
	if err := proto.UnmarshalText(string(out), metrics); err != nil {
		s.Fatal("Failed to unmarshal cpu result: ", err)
	}

	return metrics
}

// ProcessCPUMetric sends the cpu metric result to crosbolt.
func ProcessCPUMetric(cpuMetric *tppb.AndroidCpuMetric, s *testing.State) {
	foundTarget := false
	for _, processInfo := range cpuMetric.GetProcessInfo() {
		if processInfo.GetName() == targetProcessName {
			foundTarget = true

			metric := processInfo.GetMetrics()
			s.Log("megacycles: ", metric.GetMcycles())
			s.Log("runtime in nanosecond: ", metric.GetRuntimeNs())
			s.Log("min_freq in kHz: ", metric.GetMinFreqKhz())
			s.Log("max_freq in kHz: ", metric.GetMaxFreqKhz())
			s.Log("avg_freq in kHz: ", metric.GetAvgFreqKhz())

			break
		}
	}

	if foundTarget == false {
		s.Error("Failed to find the target process: ", targetProcessName)
	}
}

// ProcessMemMetric sends the memory metric result to crosbolt.
func ProcessMemMetric(memMetric *tppb.AndroidMemoryMetric, s *testing.State) {
	foundTarget := false
	for _, processMetric := range memMetric.GetProcessMetrics() {
		if processMetric.GetProcessName() == targetProcessName {
			foundTarget = true

			counters := processMetric.GetTotalCounters()
			s.Log("anon_avg in rss: ", counters.GetAnonRss().GetAvg())
			s.Log("file_avg in rss: ", counters.GetFileRss().GetAvg())
			s.Log("swap_avg in rss: ", counters.GetSwap().GetAvg())

			break
		}
	}

	if foundTarget == false {
		s.Error("Failed to find the target process: ", targetProcessName)
	}
}

// PerfettoTBMTracedProbes is the function that collects perfetto trace results from the client.
func PerfettoTBMTracedProbes(fullCtx context.Context, s *testing.State) {
	ctx, cancel := ctxutil.Shorten(fullCtx, 10*time.Second)
	defer cancel()

	// Get trace_processor_shell
	traceProcessorPath := filepath.Join(s.OutDir(), traceProcessor)
	cmd := testexec.CommandContext(ctx, "curl", "-L", "-#", "-o", traceProcessorPath, traceProcessorURL)
	if err := cmd.Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to download trace_processor: ", err)
	}
	if err := os.Chmod(traceProcessorPath, 0755); err != nil {
		s.Fatal("Failed to change trace_processor's run permission: ", err)
	}

	// Prepare local client.
	d := s.DUT()

	cl, err := rpc.Dial(ctx, d, s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	pc := platform.NewPerfettoTraceBasedMetricsServiceClient(cl.Conn)
	metrics := RunPerfettoAndRunMetric(ctx, pc, traceProcessorPath, traceConfigFileCPU, traceMetricCPU, s)

	// We may also send the result to crosbolt for the regression check.
	ProcessCPUMetric(metrics.GetAndroidCpu(), s)

	metrics = RunPerfettoAndRunMetric(ctx, pc, traceProcessorPath, traceConfigFileMEM, traceMetricMEM, s)

	ProcessMemMetric(metrics.GetAndroidMem(), s)
}
