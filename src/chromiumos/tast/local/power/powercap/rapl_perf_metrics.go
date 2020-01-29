// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package powercap contains helper functions for reading
// /sys/devices/virtual/powercap based power information.
package powercap

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/local/power"
)

// RAPLMetrics computes the power consumption in Joules of the DUT.
type RAPLMetrics struct {
	snapshot *power.RAPLSnapshot
}

// NewRAPLMetrics creates a timeline metric to collect Intel RAPL energy
// numbers.
func NewRAPLMetrics() perf.TimelineDatasource {
	return &RAPLMetrics{snapshot: nil}
}

// Setup creates a RAPLSnapshot which lets us sample energy numbers without
// worrying about overflow. We do this in Setup because there's some extra work
// scanning sysfs that might be expensive if done during the test.
func (r *RAPLMetrics) Setup(_ context.Context) error {
	snapshot, err := power.NewRAPLSnapshot()
	if err != nil {
		return errors.Wrap(err, "failed to create RAPL Snapshot")
	}
	r.snapshot = snapshot
	return nil
}

// Start collects initial energy numbers which we can use to compute the average
// power consumption between now and the first Snapshot.
func (r *RAPLMetrics) Start(_ context.Context) error {
	if _, err := r.snapshot.DiffWithCurrentRAPL(true); err != nil {
		return errors.Wrap(err, "failed to collect initial RAPL metrics")
	}
	return nil
}

// Snapshot computes the energy consumption between this and the previous
// snapshot, and reports them as metrics.
func (r *RAPLMetrics) Snapshot(_ context.Context, perfValues *perf.Values) error {
	energy, err := r.snapshot.DiffWithCurrentRAPL(true)
	if err != nil {
		return errors.Wrap(err, "failed to create collect RAPL metrics")
	}
	energy.ReportPerfMetrics(perfValues, "rapl_")
	return nil
}
