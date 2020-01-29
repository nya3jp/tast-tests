// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package powercap contains helper functions for reading
// /sys/devices/virtual/powercap based power information.
package powercap

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/perf"
)

// readFileToFloat reads a float from the contents of a file.
func readFileToFloat(filename string) (float64, error) {
	bytes, err := ioutil.ReadFile(filename)
	if err != nil {
		return 0.0, errors.Wrap(err, "unable to read float from file")
	}
	value, err := strconv.ParseFloat(strings.TrimSpace(string(bytes)), 64)
	if err != nil {
		return 0.0, errors.Wrap(err, "unable to parse float from file")
	}
	return value, nil
}

// RaplMetric tracks a single powercap metric. It contains enough information to
// compute power consumption between snapshots.
type RaplMetric struct {
	metric    perf.Metric
	path      string
	prevTime  time.Time
	prevValue float64
}

// NewRaplMetric creates a single powercap metric from the path to its dir.
func NewRaplMetric(energyPath string) (*RaplMetric, error) {
	const nameFile = "name"
	path := filepath.Dir(energyPath)
	namePath := filepath.Join(path, nameFile)
	nameBytes, err := ioutil.ReadFile(namePath)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to read name file %q for powercap energy file %q", namePath, energyPath)
	}
	name := strings.TrimSpace(string(nameBytes))
	return &RaplMetric{
		metric:    perf.Metric{Name: "rapl_" + name, Unit: "mW", Direction: perf.SmallerIsBetter, Multiple: true},
		path:      energyPath,
		prevValue: 0.0,
		prevTime:  time.Now(),
	}, nil
}

// Start reads the initial energy value so that the next Snapshot is relative
// to now.
func (m *RaplMetric) Start() error {
	thisTime := time.Now()
	thisValue, err := readFileToFloat(m.path)
	if err != nil {
		return errors.Wrap(err, "unable to read powercap value")
	}
	m.prevTime = thisTime
	m.prevValue = thisValue
	return nil
}

// Snapshot takes a snapshot of a single powercap metric by reading the current
// energy value, and computing average Watts consumed since the last snapshot.
func (m *RaplMetric) Snapshot(values *perf.Values) error {
	thisTime := time.Now()
	thisValue, err := readFileToFloat(m.path)
	if err != nil {
		return errors.Wrap(err, "unable to read powercap value")
	}
	deltaT := thisTime.Sub(m.prevTime).Seconds()
	const milliPerMicro = 0.001
	deltamJ := (thisValue - m.prevValue) * milliPerMicro
	values.Append(m.metric, deltamJ/deltaT)
	m.prevTime = thisTime
	m.prevValue = thisValue
	return nil
}

// RaplMetrics contains a RaplMetric for every metric on the DUT.
type RaplMetrics struct {
	metrics []*RaplMetric
}

// NewRaplMetrics creates powercap metrics for every metric available on the
// system.
func NewRaplMetrics() *RaplMetrics {
	return &RaplMetrics{}
}

// raplRoot is the root folder in sysfs containing all rapl metrics.
const raplRoot = "/sys/devices/virtual/powercap/intel-rapl/"

// Setup looks for folders containing energy_uj under raplRoot, and creates a
// RaplMetric for each.
func (ms *RaplMetrics) Setup() error {
	names := make(map[string]bool)
	err := filepath.Walk(raplRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		const energyFile = "energy_uj"
		if filepath.Base(path) != energyFile {
			return nil
		}
		metric, err := NewRaplMetric(path)
		if err != nil {
			return errors.Wrap(err, "unable to create powercap metric")
		}
		if _, exists := names[metric.metric.Name]; exists {
			return errors.Wrapf(err, "multiple metrics with name %q", metric.metric.Name)
		}
		names[metric.metric.Name] = true
		ms.metrics = append(ms.metrics, metric)
		return nil
	})
	if err != nil {
		return errors.Wrap(err, "failed to walk powercap files")
	}
	return nil
}

// Start calls Start on all powercap metrics so that initial energy levels can
// be read.
func (ms *RaplMetrics) Start() error {
	for _, m := range ms.metrics {
		if err := m.Start(); err != nil {
			return err
		}
	}
	return nil
}

// Snapshot snapshots all powercap metrics.
func (ms *RaplMetrics) Snapshot(values *perf.Values) error {
	for _, m := range ms.metrics {
		if err := m.Snapshot(values); err != nil {
			return err
		}
	}
	return nil
}

// RaplSupported returns true if powercap metrics are supported on this DUT.
func RaplSupported() bool {
	if _, err := os.Stat(raplRoot); err != nil {
		return false
	}
	return true
}
