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

const (
	targetProcessName = "/usr/sbin/iioservice"
)

// ProcessCPUMetric extracts information of the target process in the
// cpu metric.
func ProcessCPUMetric(cpuMetric *perfetto_proto.AndroidCpuMetric, testName string, s *testing.State) {
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

			pv := perf.NewValues()
			pv.Set(perf.Metric{
				Name:      testName + ".CPU_Megacycles",
				Unit:      "megacycles",
				Direction: perf.SmallerIsBetter,
			}, float64(metric.GetMcycles()))

			pv.Set(perf.Metric{
				Name:      testName + ".runtime",
				Unit:      "nanosecond",
				Direction: perf.SmallerIsBetter,
			}, float64(metric.GetRuntimeNs()))

			pv.Set(perf.Metric{
				Name:      testName + ".min_freq",
				Unit:      "kHz",
				Direction: perf.SmallerIsBetter,
			}, float64(metric.GetMinFreqKhz()))

			pv.Set(perf.Metric{
				Name:      testName + ".max_freq",
				Unit:      "kHz",
				Direction: perf.SmallerIsBetter,
			}, float64(metric.GetMaxFreqKhz()))

			pv.Set(perf.Metric{
				Name:      testName + ".avg_freq",
				Unit:      "kHz",
				Direction: perf.SmallerIsBetter,
			}, float64(metric.GetAvgFreqKhz()))

			if err := pv.Save(s.OutDir()); err != nil {
				s.Error("Failed to save perf data: ", err)
			}

			break
		}
	}

	if foundTarget == false {
		s.Error("Failed to find the target process: ", targetProcessName)
	}
}

// ProcessMemMetric extracts information of the target process in the
// memory metric.
func ProcessMemMetric(memMetric *perfetto_proto.AndroidMemoryMetric, testName string, s *testing.State) {
	foundTarget := false
	for _, processMetric := range memMetric.GetProcessMetrics() {
		if processMetric.GetProcessName() == targetProcessName {
			foundTarget = true

			counters := processMetric.GetTotalCounters()
			s.Log("anon_avg in rss: ", counters.GetAnonRss().GetAvg())
			s.Log("file_avg in rss: ", counters.GetFileRss().GetAvg())
			s.Log("swap_avg in rss: ", counters.GetSwap().GetAvg())

			pv := perf.NewValues()
			pv.Set(perf.Metric{
				Name:      testName + ".anon_avg",
				Unit:      "rss",
				Direction: perf.SmallerIsBetter,
			}, counters.GetAnonRss().GetAvg())

			pv.Set(perf.Metric{
				Name:      testName + ".file_avg",
				Unit:      "rss",
				Direction: perf.SmallerIsBetter,
			}, counters.GetFileRss().GetAvg())

			pv.Set(perf.Metric{
				Name:      testName + ".swap_avg",
				Unit:      "rss",
				Direction: perf.SmallerIsBetter,
			}, counters.GetSwap().GetAvg())

			if err := pv.Save(s.OutDir()); err != nil {
				s.Error("Failed to save perf data: ", err)
			}

			break
		}
	}

	if foundTarget == false {
		s.Error("Failed to find the target process: ", targetProcessName)
	}
}
