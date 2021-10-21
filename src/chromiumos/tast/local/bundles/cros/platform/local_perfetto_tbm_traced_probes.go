// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"time"

	"android.googlesource.com/platform/external/perfetto/protos/perfetto/metrics"

	"chromiumos/tast/local/bundles/cros/platform/perfetto"
	"chromiumos/tast/testing"
)

const (
	traceMetricCPU = "android_cpu"
	traceMetricMEM = "android_mem"

	targetProcessName = "/usr/bin/traced_probes"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     LocalPerfettoTBMTracedProbes,
		Desc:     "Verifies functions of Perfetto traced, traced_probes and trace_processor_shell",
		Contacts: []string{"chenghaoyang@chromium.org", "chinglinyu@chromium.org"},
		Data:     []string{perfetto.TBMTracedProbesConfigFile, perfetto.GetTraceProcessorByArch()},
		Attr:     []string{"group:mainline", "informational"},
	})
}

// processCPUMetric extracts information of the target process in the
// cpu metric.
func processCPUMetric(cpuMetric *perfetto_protos.AndroidCpuMetric, s *testing.State) {
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
func processMemMetric(memMetric *perfetto_protos.AndroidMemoryMetric, s *testing.State) {
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

// LocalPerfettoTBMTracedProbes tests perfetto trace collection on
// traced_probes and process the trace result with trace_processor_shell.
func LocalPerfettoTBMTracedProbes(ctx context.Context, s *testing.State) {
	// Start a trace session using the perfetto command line tool.
	traceConfigPath := s.DataPath(perfetto.TBMTracedProbesConfigFile)
	sess, err := perfetto.StartTracing(ctx, traceConfigPath)
	// The temporary file of trace data is no longer needed when returned.
	defer sess.RemoveTraceResultFile()

	if err != nil {
		s.Fatal("Failed to start tracing: ", err)
	}

	// Developers can run other tests to trigger more trace data.
	const pauseDuration = time.Second * 10
	if err := testing.Sleep(ctx, pauseDuration); err != nil {
		s.Fatal("Failed to sleep while waiting for overview to trigger: ", err)
	}

	if err := sess.Stop(); err != nil {
		s.Fatal("Failed to stop the tracing session: ", err)
	}

	metrics, err := sess.RunMetrics(ctx, s.DataPath(perfetto.GetTraceProcessorByArch()), []string{traceMetricCPU, traceMetricMEM})
	if err != nil {
		s.Fatal("Failed to RunMetrics: ", err)
	}

	processCPUMetric(metrics.GetAndroidCpu(), s)
	processMemMetric(metrics.GetAndroidMem(), s)
}
