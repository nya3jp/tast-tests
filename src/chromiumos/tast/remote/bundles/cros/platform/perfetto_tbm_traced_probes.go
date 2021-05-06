// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"os"
	"time"

	tppb "chromiumos/perfetto/trace_processor"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/remote/bundles/cros/platform/perfetto"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/platform"
	"chromiumos/tast/testing"
)

const (
	traceConfigFile = "perfetto/perfetto_tbm_traced_probes.pbtxt"

	traceMetricCPU = "android_cpu"
	traceMetricMEM = "android_mem"

	targetProcessName = "/usr/bin/traced_probes"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:        PerfettoTBMTracedProbes,
		Desc:        "Verifies functions of Perfetto traced and traced_probes",
		Contacts:    []string{"chenghaoyang@chromium.org", "chinglinyu@chromium.org"},
		Data:        []string{traceConfigFile, perfetto.TraceProcessor},
		ServiceDeps: []string{"tast.cros.platform.PerfettoTraceBasedMetricsService"},
	})
}

// processCPUMetric sends the cpu metric result to crosbolt.
func processCPUMetric(cpuMetric *tppb.AndroidCpuMetric, s *testing.State) {
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

// processMemMetric sends the memory metric result to crosbolt.
func processMemMetric(memMetric *tppb.AndroidMemoryMetric, s *testing.State) {
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

	traceProcessorPath := s.DataPath(perfetto.TraceProcessor)
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
	outputPath := perfetto.RunPerfetto(ctx, s, &pc, traceConfigFile, 10*1024*1024)
	metrics := perfetto.RunMetric(ctx, s, outputPath, []string{traceMetricCPU, traceMetricMEM, "android_powrails"})

	// We may also send the result to crosbolt for the regression check.
	processCPUMetric(metrics.GetAndroidCpu(), s)
	processMemMetric(metrics.GetAndroidMem(), s)
}
