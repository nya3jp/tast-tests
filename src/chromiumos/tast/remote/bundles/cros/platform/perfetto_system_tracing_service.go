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

	"chromiumos/tast/common/perf"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/platform"
	"chromiumos/tast/testing"
	tppb "chromiumos/trace_processor"
)

const (
	perfettoConfigCPU = `
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
			target_buffer: 1
			process_stats_config {
				scan_all_processes_on_start: true
			}
		}
	}
	data_sources: {
		config {
			name: "linux.sys_stats"
			sys_stats_config {
				stat_period_ms: 1000
				stat_counters: STAT_CPU_TIMES
				stat_counters: STAT_FORK_COUNT
			}
		}
	}
	data_sources: {
		config {
			name: "linux.ftrace"
			ftrace_config {
				ftrace_events: "sched/sched_switch"
				ftrace_events: "power/suspend_resume"
				ftrace_events: "sched/sched_wakeup"
				ftrace_events: "sched/sched_wakeup_new"
				ftrace_events: "sched/sched_waking"
				ftrace_events: "power/cpu_frequency"
				ftrace_events: "power/cpu_idle"
				ftrace_events: "raw_syscalls/sys_enter"
				ftrace_events: "raw_syscalls/sys_exit"
				ftrace_events: "sched/sched_process_exit"
				ftrace_events: "sched/sched_process_free"
				ftrace_events: "task/task_newtask"
				ftrace_events: "task/task_rename"
			}
		}
	}
	duration_ms: 5000
	`

	perfettoConfigMEM = `
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
	duration_ms: 5000`

	traceProcessorURL = "https://storage.googleapis.com/perfetto/trace_processor_shell-linux-a3ce2cbf4cbe4f86cc10b02957db727cecfafae8"
	traceProcessor    = "trace_processor"

	traceMetricCPU = "android_cpu"
	traceMetricMEM = "android_mem"

	targetProcessName = "/usr/bin/traced_probes"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PerfettoSystemTracingService,
		Desc:         "Verifies functions of Perfetto traced and traced_probes",
		Contacts:     []string{"chenghaoyang@chromium.org", "chinglinyu@chromium.org"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		ServiceDeps:  []string{"tast.cros.platform.PerfettoSystemTracingService"},
		SoftwareDeps: []string{"chrome"},
	})
}

// RunPerfettoAndRunMetric uses gRPC to run perfetto cmdline with |config| in the DUT and collect the result with trace_processor_shell.
func RunPerfettoAndRunMetric(ctx context.Context, pc platform.PerfettoSystemTracingServiceClient, traceProcessorPath string, config string, metric string, s *testing.State) *tppb.TraceMetrics {
	response, err := pc.GeneratePerfettoTrace(ctx, &platform.GeneratePerfettoTraceRequest{Config: config})
	if err != nil {
		s.Fatal("Failed to call gRPC GeneratePerfettoTrace: ", err)
	}
	// Store pb into file for debug
	outputPath := filepath.Join(s.OutDir(), metric+".pb")
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
func ProcessCPUMetric(cpuMetric *tppb.AndroidCpuMetric, pv *perf.Values, s *testing.State) {
	foundTarget := false
	for _, processInfo := range cpuMetric.GetProcessInfo() {
		if processInfo.GetName() == targetProcessName {
			foundTarget = true

			metric := processInfo.GetMetrics()

			pv.Append(perf.Metric{
				Name:      traceMetricCPU,
				Variant:   "cycles",
				Unit:      "megacycle",
				Direction: perf.SmallerIsBetter,
				Multiple:  true,
			}, float64(metric.GetMcycles()))

			pv.Append(perf.Metric{
				Name:      traceMetricCPU,
				Variant:   "runtime",
				Unit:      "nanosecond",
				Direction: perf.SmallerIsBetter,
				Multiple:  true,
			}, float64(metric.GetRuntimeNs()))

			pv.Append(perf.Metric{
				Name:      traceMetricCPU,
				Variant:   "min_freq",
				Unit:      "kHz",
				Direction: perf.SmallerIsBetter,
				Multiple:  true,
			}, float64(metric.GetMinFreqKhz()))

			pv.Append(perf.Metric{
				Name:      traceMetricCPU,
				Variant:   "max_freq",
				Unit:      "kHz",
				Direction: perf.SmallerIsBetter,
				Multiple:  true,
			}, float64(metric.GetMaxFreqKhz()))

			pv.Append(perf.Metric{
				Name:      traceMetricCPU,
				Variant:   "avg_freq",
				Unit:      "kHz",
				Direction: perf.SmallerIsBetter,
				Multiple:  true,
			}, float64(metric.GetAvgFreqKhz()))

			break
		}
	}

	if foundTarget == false {
		s.Error("Failed to find the target process: ", targetProcessName)
	}
}

// ProcessMemMetric sends the memory metric result to crosbolt.
func ProcessMemMetric(memMetric *tppb.AndroidMemoryMetric, pv *perf.Values, s *testing.State) {
	foundTarget := false
	for _, processMetric := range memMetric.GetProcessMetrics() {
		if processMetric.GetProcessName() == targetProcessName {
			foundTarget = true

			counters := processMetric.GetTotalCounters()

			pv.Append(perf.Metric{
				Name:      traceMetricMEM,
				Variant:   "anon_avg",
				Unit:      "rss",
				Direction: perf.SmallerIsBetter,
				Multiple:  true,
			}, counters.GetAnonRss().GetAvg())

			pv.Append(perf.Metric{
				Name:      traceMetricMEM,
				Variant:   "file_avg",
				Unit:      "rss",
				Direction: perf.SmallerIsBetter,
				Multiple:  true,
			}, counters.GetFileRss().GetAvg())

			pv.Append(perf.Metric{
				Name:      traceMetricMEM,
				Variant:   "swap_avg",
				Unit:      "rss",
				Direction: perf.SmallerIsBetter,
				Multiple:  true,
			}, counters.GetSwap().GetAvg())

			break
		}
	}

	if foundTarget == false {
		s.Error("Failed to find the target process: ", targetProcessName)
	}
}

// PerfettoSystemTracingService is the function that collects perfetto trace results from the client.
func PerfettoSystemTracingService(fullCtx context.Context, s *testing.State) {
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

	pv := perf.NewValues()

	pc := platform.NewPerfettoSystemTracingServiceClient(cl.Conn)
	metrics := RunPerfettoAndRunMetric(ctx, pc, traceProcessorPath, perfettoConfigCPU, traceMetricCPU, s)

	ProcessCPUMetric(metrics.GetAndroidCpu(), pv, s)

	metrics = RunPerfettoAndRunMetric(ctx, pc, traceProcessorPath, perfettoConfigMEM, traceMetricMEM, s)

	ProcessMemMetric(metrics.GetAndroidMem(), pv, s)

	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}
