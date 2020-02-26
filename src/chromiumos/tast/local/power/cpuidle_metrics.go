// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strconv"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/perf"
)

type cpuidleMetric struct {
	dir                     string
	prevUsage, prevTime     uint64
	usageMetric, timeMetric perf.Metric
}

func (m *cpuidleMetric) readMetric(name string) (uint64, error) {
	bytes, err := ioutil.ReadFile(filepath.Join(m.dir, name))
	if err != nil {
		return 0.0, errors.Wrapf(err, "failed to read %q from %q", name, m.dir)
	}
	val, err := strconv.ParseUint(strings.TrimSpace(string(bytes)), 10, 64)
	if err != nil {
		return 0.0, errors.Wrapf(err, "failed to parse %q from %q", name, m.dir)
	}
	return val, nil
}

func (m *cpuidleMetric) start() error {
	return m.snapshot(nil)
}

func (m *cpuidleMetric) snapshot(v *perf.Values) error {
	const timePath = "time"
	time, err := m.readMetric(timePath)
	if err != nil {
		return err
	}

	const usagePath = "usage"
	usage, err := m.readMetric(usagePath)
	if err != nil {
		return err
	}

	if v != nil {
		v.Append(m.timeMetric, float64(time-m.prevTime))
		v.Append(m.usageMetric, float64(usage-m.prevUsage))
	}

	m.prevTime = time
	m.prevUsage = usage
	return nil
}

// CpuidleMetrics is a perf.TimelineDatasource for tracking cpu idle states
// during a test.
type CpuidleMetrics struct {
	metrics []*cpuidleMetric
}

var _ perf.TimelineDatasource = &CpuidleMetrics{}

func cpuidleMetricNamePrefix(cpu, state string) (string, error) {
	name, err := ioutil.ReadFile(filepath.Join(state, "name"))
	if err != nil {
		return "", errors.Wrap(err, "failed to read cpuidle state name")
	}
	return fmt.Sprintf("%s_%s", filepath.Base(cpu), strings.TrimSpace(string(name))), nil
}

// Setup finds all cpuidle/state folders for all CPUs on the system, and creates
// a cpuidleMetric for each so they can quickly generate metrics in Snapshot.
func (m *CpuidleMetrics) Setup(_ context.Context) error {
	const cpuPath = "/sys/devices/system/cpu/cpu*"
	cpus, err := filepath.Glob(cpuPath)
	if err != nil {
		return errors.Wrap(err, "failed to enumerate cpus in sysfs")
	}
	var metrics []*cpuidleMetric
	for _, cpu := range cpus {
		const cpuidlePath = "cpuidle/state*"
		states, err := filepath.Glob(filepath.Join(cpu, cpuidlePath))
		if err != nil {
			return errors.Wrapf(err, "failed to enumerate idle states under %q", cpu)
		}
		for _, state := range states {
			prefix, err := cpuidleMetricNamePrefix(cpu, state)
			if err != nil {
				return err
			}
			metrics = append(metrics, &cpuidleMetric{
				dir: state,
				usageMetric: perf.Metric{
					Name: prefix + "_usage", Unit: "count", Direction: perf.SmallerIsBetter, Multiple: true,
				},
				timeMetric: perf.Metric{
					Name: prefix + "_time", Unit: "us", Direction: perf.BiggerIsBetter, Multiple: true,
				},
			})
		}
	}
	m.metrics = metrics
	return nil
}

// Start collects initial results for all cpuidleMetric structs.
func (m *CpuidleMetrics) Start(_ context.Context) error {
	for _, metric := range m.metrics {
		if err := metric.start(); err != nil {
			return err
		}
	}
	return nil
}

// Snapshot computes the number of microseconds spent in, and count of entries
// to every sleep state for every CPU on the system.
func (m *CpuidleMetrics) Snapshot(_ context.Context, v *perf.Values) error {
	for _, metric := range m.metrics {
		if err := metric.snapshot(v); err != nil {
			return err
		}
	}
	return nil
}

// NewCpuidleMetrics creates a TimelineDatasource that collects stats on which
// idle states are used during a test.
func NewCpuidleMetrics() *CpuidleMetrics {
	return &CpuidleMetrics{}
}
