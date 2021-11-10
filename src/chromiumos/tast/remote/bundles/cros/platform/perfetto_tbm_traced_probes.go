// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"

	"android.googlesource.com/platform/external/perfetto/protos/perfetto/metrics/github.com/google/perfetto/perfetto_proto"

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

// processCPUMetric extracts information of the target process in the
// cpu metric.
func processCPUMetric(cpuMetric *perfetto_proto.AndroidCpuMetric, s *testing.State) {
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

// processMemMetric extracts information of the target process in the
// mem metric.
func processMemMetric(memMetric *perfetto_proto.AndroidMemoryMetric, s *testing.State) {
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

// PerfettoTBMTracedProbes is the function that collects perfetto
// trace results with trace-based metrics from the client.
func PerfettoTBMTracedProbes(ctx context.Context, s *testing.State) {
	// Prepare local client.
	d := s.DUT()

	cl, err := rpc.Dial(ctx, d, s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	pc := platform.NewPerfettoTraceBasedMetricsServiceClient(cl.Conn)
	outputPath, err := perfetto.RunPerfetto(ctx, pc, s.DataPath(traceConfigFile))
	if err != nil {
		s.Fatal("Failed to RunPerfetto: ", err)
	}

	metrics, err := perfetto.RunMetrics(ctx, s.DataPath(perfetto.TraceProcessor), outputPath, []string{traceMetricCPU, traceMetricMEM})
	if err != nil {
		s.Fatal("Failed to RunMetrics: ", err)
	}

	// We may also send the result to crosbolt for the regression check.
	processCPUMetric(metrics.GetAndroidCpu(), s)
	processMemMetric(metrics.GetAndroidMem(), s)
}
