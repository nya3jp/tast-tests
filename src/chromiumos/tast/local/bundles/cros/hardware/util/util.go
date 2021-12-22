// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package util provides utility functions used in cros/hardware package.
package util

import (
	"android.googlesource.com/platform/external/perfetto/protos/perfetto/metrics/github.com/google/perfetto/perfetto_proto"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/testing"
)

// ProcessCPUMetric extracts information of the target process in the
// cpu metric.
func ProcessCPUMetric(cpuMetric *perfetto_proto.AndroidCpuMetric, metricNamePrefix, targetProcessName string, pv *perf.Values, s *testing.State) {
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

			pv.Set(perf.Metric{
				Name:      metricNamePrefix + ".CPU_Megacycles",
				Unit:      "megacycles",
				Direction: perf.SmallerIsBetter,
			}, float64(metric.GetMcycles()))

			pv.Set(perf.Metric{
				Name:      metricNamePrefix + ".runtime",
				Unit:      "nanosecond",
				Direction: perf.SmallerIsBetter,
			}, float64(metric.GetRuntimeNs()))

			pv.Set(perf.Metric{
				Name:      metricNamePrefix + ".min_freq",
				Unit:      "kHz",
				Direction: perf.SmallerIsBetter,
			}, float64(metric.GetMinFreqKhz()))

			pv.Set(perf.Metric{
				Name:      metricNamePrefix + ".max_freq",
				Unit:      "kHz",
				Direction: perf.SmallerIsBetter,
			}, float64(metric.GetMaxFreqKhz()))

			pv.Set(perf.Metric{
				Name:      metricNamePrefix + ".avg_freq",
				Unit:      "kHz",
				Direction: perf.SmallerIsBetter,
			}, float64(metric.GetAvgFreqKhz()))

			break
		}
	}

	if foundTarget == false {
		s.Error("Failed to find the target process: ", targetProcessName)
	}
}

// ProcessMemMetric extracts information of the target process in the
// memory metric.
func ProcessMemMetric(memMetric *perfetto_proto.AndroidMemoryMetric, metricNamePrefix, targetProcessName string, pv *perf.Values, s *testing.State) {
	foundTarget := false
	for _, processMetric := range memMetric.GetProcessMetrics() {
		if processMetric.GetProcessName() == targetProcessName {
			foundTarget = true

			counters := processMetric.GetTotalCounters()
			s.Log("anon_avg in rss: ", counters.GetAnonRss().GetAvg())
			s.Log("file_avg in rss: ", counters.GetFileRss().GetAvg())
			s.Log("swap_avg in rss: ", counters.GetSwap().GetAvg())

			pv.Set(perf.Metric{
				Name:      metricNamePrefix + ".anon_avg",
				Unit:      "rss",
				Direction: perf.SmallerIsBetter,
			}, counters.GetAnonRss().GetAvg())

			pv.Set(perf.Metric{
				Name:      metricNamePrefix + ".file_avg",
				Unit:      "rss",
				Direction: perf.SmallerIsBetter,
			}, counters.GetFileRss().GetAvg())

			pv.Set(perf.Metric{
				Name:      metricNamePrefix + ".swap_avg",
				Unit:      "rss",
				Direction: perf.SmallerIsBetter,
			}, counters.GetSwap().GetAvg())

			break
		}
	}

	if foundTarget == false {
		s.Error("Failed to find the target process: ", targetProcessName)
	}
}
