// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package util provides utility functions used in cros/hardware package.
package util

import (
	"android.googlesource.com/platform/external/perfetto/protos/perfetto/metrics/github.com/google/perfetto/perfetto_proto"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
)

// ProcessCPUMetric extracts information of the target process in the
// cpu metric.
func ProcessCPUMetric(cpuMetric *perfetto_proto.AndroidCpuMetric, metricNamePrefix, targetProcessName string, pv *perf.Values) error {
	for _, processInfo := range cpuMetric.GetProcessInfo() {
		if processInfo.GetName() == targetProcessName {
			metric := processInfo.GetMetrics()

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

			return nil
		}
	}

	return errors.Errorf("failed to find the target process: %s", targetProcessName)
}

// ProcessMemMetric extracts information of the target process in the
// memory metric.
func ProcessMemMetric(memMetric *perfetto_proto.AndroidMemoryMetric, metricNamePrefix, targetProcessName string, pv *perf.Values) error {
	for _, processMetric := range memMetric.GetProcessMetrics() {
		if processMetric.GetProcessName() == targetProcessName {
			counters := processMetric.GetTotalCounters()

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

			return nil
		}
	}

	return errors.Errorf("failed to find the target process: %s", targetProcessName)
}
