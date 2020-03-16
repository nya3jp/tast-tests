// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/perf"
)

// RAPLMetrics records the energy consumption in Joules of the DUT.
type RAPLMetrics struct {
	snapshot *RAPLSnapshot
}

// Assert that RAPLMetrics can be used in perf.Timeline.
var _ perf.TimelineDatasource = &RAPLMetrics{}

// NewRAPLMetrics creates a timeline metric to collect Intel RAPL energy
// numbers.
func NewRAPLMetrics() *RAPLMetrics {
	return &RAPLMetrics{snapshot: nil}
}

// Setup creates a RAPLSnapshot which lets us sample energy numbers without
// worrying about overflow. We do this in Setup because there's some extra work
// scanning sysfs that might be expensive if done during the test.
func (r *RAPLMetrics) Setup(_ context.Context) error {
	snapshot, err := NewRAPLSnapshot()
	if err != nil {
		return errors.Wrap(err, "failed to create RAPL Snapshot")
	}
	r.snapshot = snapshot
	return nil
}

// Start collects initial energy numbers which we can use to compute the average
// power consumption between now and the first Snapshot.
func (r *RAPLMetrics) Start(_ context.Context) error {
	if r.snapshot == nil {
		// RAPL is not supported.
		return nil
	}
	if _, err := r.snapshot.DiffWithCurrentRAPLAndReset(); err != nil {
		return errors.Wrap(err, "failed to collect initial RAPL metrics")
	}
	return nil
}

// Snapshot computes the energy consumption between this and the previous
// snapshot, and reports them as metrics.
func (r *RAPLMetrics) Snapshot(_ context.Context, perfValues *perf.Values) error {
	if r.snapshot == nil {
		// RAPL is not supported.
		return nil
	}
	energy, err := r.snapshot.DiffWithCurrentRAPLAndReset()
	if err != nil {
		return errors.Wrap(err, "failed to create collect RAPL metrics")
	}
	energy.ReportPerfMetrics(perfValues, "rapl_")
	return nil
}
