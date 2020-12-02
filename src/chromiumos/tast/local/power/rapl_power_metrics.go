// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"context"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
)

const package0PowerConstraintName = "package-0-pl"

// RAPLPowerMetrics records the power consumption in Watt of the DUT.
type RAPLPowerMetrics struct {
	snapshot *RAPLSnapshot
	metrics  map[string]perf.Metric
	prefix   string
}

// Assert that RAPLPowerMetrics can be used in perf.Timeline.
var _ perf.TimelineDatasource = &RAPLPowerMetrics{}

// NewRAPLPowerMetrics creates a timeline metric to collect Intel RAPL energy
// numbers.
func NewRAPLPowerMetrics() *RAPLPowerMetrics {
	return &RAPLPowerMetrics{nil, make(map[string]perf.Metric), ""}
}

// NewRAPLPowerMetricsWithPrefix creates a timeline metric to collect Intel
// RAPL energy numbers. The given prefix will be put in front of metric name.
func NewRAPLPowerMetricsWithPrefix(prefix string) *RAPLPowerMetrics {
	return &RAPLPowerMetrics{nil, make(map[string]perf.Metric), prefix}
}

// Setup creates a RAPLSnapshot which lets us sample energy numbers without
// worrying about overflow. We do this in Setup because there's some extra work
// scanning sysfs that might be expensive if done during the test.
func (r *RAPLPowerMetrics) Setup(_ context.Context, prefix string) error {
	snapshot, err := NewRAPLSnapshot()
	if err != nil {
		return errors.Wrap(err, "failed to create RAPL Snapshot")
	}
	r.snapshot = snapshot
	r.prefix = prefix
	return nil
}

// Start collects initial energy numbers which we can use to compute the average
// power consumption between now and the first Snapshot.
func (r *RAPLPowerMetrics) Start(_ context.Context) error {
	if r.snapshot == nil {
		// RAPL is not supported.
		return nil
	}
	if _, err := r.snapshot.DiffWithCurrentRAPLAndReset(); err != nil {
		return errors.Wrap(err, "failed to collect initial RAPL metrics")
	}
	for name := range r.snapshot.start.joules {
		r.metrics[name] = perf.Metric{Name: r.prefix + name, Unit: "W",
			Direction: perf.SmallerIsBetter, Multiple: true}
	}
	r.metrics[package0PowerConstraintName] = perf.Metric{Name: r.prefix + package0PowerConstraintName, Unit: "W", Direction: perf.SmallerIsBetter, Multiple: true}
	return nil
}

// Snapshot computes the energy consumption between this and the previous
// snapshot, and reports them as metrics.
func (r *RAPLPowerMetrics) Snapshot(_ context.Context, values *perf.Values) error {
	if r.snapshot == nil {
		// RAPL is not supported.
		return nil
	}
	energy, err := r.snapshot.DiffWithCurrentRAPLAndReset()
	if err != nil {
		return errors.Wrap(err, "failed to collect initial RAPL metrics")
	}
	for name := range r.metrics {
		if name == package0PowerConstraintName {
			values.Append(r.metrics[package0PowerConstraintName], energy.package0PowerConstraint)
		} else {
			// Report Converted values from Joules to Watt
			values.Append(r.metrics[name], energy.joules[name]/energy.duration.Seconds())
		}
	}
	return nil
}
