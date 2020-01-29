// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package powercap contains helper functions for reading
// /sys/devices/virtual/powercap based power information.
package powercap

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/local/power"
)

// RAPLMetrics computes the power consumption in Watts of the DUT.
type RAPLMetrics struct {
	snapshot   *power.RAPLSnapshot
	prevValues *power.RAPLValues
	prevTime   time.Time
}

// NewRAPLMetrics creates a timeline metric to collect Intel RAPL power numbers.
func NewRAPLMetrics() perf.TimelineDatasource {
	return &RAPLMetrics{snapshot: nil}
}

// Setup creates a RAPLSnapshot which lets us sample energy numbers without
// worrying about overflow.
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
	r.prevTime = time.Now()
	prevValues, err := r.snapshot.DiffWithCurrentRAPL()
	if err != nil {
		return errors.Wrap(err, "failed to collect initial RAPL metrics")
	}
	r.prevValues = prevValues
	return nil
}

// Snapshot computes the average power consumption between this and the previous
// snapshot by diffing two energy numbers and dividing by time.
func (r *RAPLMetrics) Snapshot(_ context.Context, perfValues *perf.Values) error {
	now := time.Now()
	curr, err := r.snapshot.DiffWithCurrentRAPL()
	if err != nil {
		return errors.Wrap(err, "failed to create collect RAPL metrics")
	}

	dt := now.Sub(r.prevTime)
	prev := r.prevValues
	for _, e := range []struct {
		name  string
		value float64
	}{
		{"Package0", curr.Package0 - prev.Package0},
		{"Core", curr.Core - prev.Core},
		{"Uncore", curr.Uncore - prev.Uncore},
		{"DRAM", curr.DRAM - prev.DRAM},
		{"Psys", curr.Psys - prev.Psys},
	} {
		perfValues.Append(perf.Metric{
			Name:      "rapl_" + e.name,
			Unit:      "Watts",
			Direction: perf.SmallerIsBetter,
			Multiple:  true,
		}, (e.value / dt.Seconds()))
	}

	return nil
}
