// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"context"
	"path"
	"path/filepath"
	"regexp"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
)

// CPUFrequencyMetrics records per-core CPU frequency of the DUT.
type CPUFrequencyMetrics struct {
	cpuFrequencyFiles map[string]string
	metrics           map[string]perf.Metric
}

// Assert that CPUFrequencyMetrics can be used in perf.Timeline.
var _ perf.TimelineDatasource = &CPUFrequencyMetrics{}

// NewCPUFrequencyMetrics creates a timeline metric to collect CPU Frequency.
func NewCPUFrequencyMetrics() *CPUFrequencyMetrics {
	return &CPUFrequencyMetrics{nil, make(map[string]perf.Metric)}
}

// Setup determines which CPUs should be queried.
func (cs *CPUFrequencyMetrics) Setup(ctx context.Context, prefix string) error {
	cpuFrequencyFiles, err := getCPUFrequencyFiles(ctx)
	if err != nil {
		return errors.Wrap(err, "error finding cpuidles")
	}
	cs.cpuFrequencyFiles = cpuFrequencyFiles
	return nil
}

// getCPUFrequencyFiles returns a mapping from cpus to files
// containing the frequency of each core at the time.
func getCPUFrequencyFiles(ctx context.Context) (map[string]string, error) {
	ret := make(map[string]string)

	const cpuDirs = "/sys/devices/system/cpu/cpu*"
	dirs, err := filepath.Glob(cpuDirs)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get cpu names")
	}

	for _, dir := range dirs {
		if match, err := regexp.MatchString(`^cpu\d+$`, path.Base(dir)); err != nil {
			return nil, errors.Wrap(err, "error trying to match cpu name")
		} else if !match {
			continue
		}
		cpuName := filepath.Base(dir)
		fileName := path.Join(dir, "cpufreq/scaling_cur_freq")
		ret[cpuName] = fileName
	}

	return ret, nil
}

// readCPUFrequency reads the cpuidle timings and return a mapping from cpu idle states and cpu names
// to the time spent in the state & cpu pairs so far.
func readCPUFrequency(cpuFrequencyFiles map[string]string) (map[string](int64), error) {
	ret := make(map[string](int64))
	for cpuName, file := range cpuFrequencyFiles {
		t, err := readInt64(file)
		if err != nil {
			return nil, errors.Wrap(err, "failed to read cpu frequency")
		}
		ret[cpuName] = t
	}
	return ret, nil
}

// Start initalizes frequency metric for each cpu.
func (cs *CPUFrequencyMetrics) Start(ctx context.Context) error {

	for cpuName := range cs.cpuFrequencyFiles {

		cs.metrics[cpuName+"-frequency"] = perf.Metric{Name: cpuName + "-frequency", Unit: "MHz",
			Direction: perf.SmallerIsBetter, Multiple: true}

	}

	return nil
}

// Snapshot gets the frequency value of each core.
func (cs *CPUFrequencyMetrics) Snapshot(ctx context.Context, values *perf.Values) error {

	stats, err := readCPUFrequency(cs.cpuFrequencyFiles)
	if err != nil {
		return errors.Wrap(err, "failed to collect metrics")
	}

	for cpuName, frequency := range stats {
		// Use MHz
		values.Append(cs.metrics[cpuName+"-frequency"], float64(frequency)/1000.)

	}
	return nil
}

// Stop does nothing.
func (cs *CPUFrequencyMetrics) Stop(ctx context.Context, values *perf.Values) error {
	return nil
}
