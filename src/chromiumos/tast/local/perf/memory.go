// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package perf

import (
	"context"
	"time"

	"github.com/shirou/gopsutil/mem"

	"chromiumos/tast/common/perf"
)

// diffWait is the default duration to measure the baseline of memoryDataSource.
const diffWait = 5 * time.Second

// memoryDataSource is a perf.TimelineDatasource reporting the memory usage and its diff from certain point.
type memoryDataSource struct {
	name        string
	diffName    string
	percentName string
	previous    float64
}

// NewMemoryDataSource creates a new instance of memoryDataSource with the
// given name.
func NewMemoryDataSource(name, diffName, percentName string) *memoryDataSource {
	return &memoryDataSource{name: name, diffName: diffName, percentName: percentName}
}

// Setup implements perf.TimelineDatasource.Setup.
func (s *memoryDataSource) Setup(ctx context.Context, prefix string) error {
	s.name = prefix + s.name
	s.diffName = prefix + s.diffName
	s.percentName = prefix + s.percentName
	return nil
}

// Start implements perf.TimelineDatasource.Start.
func (s *memoryDataSource) Start(ctx context.Context) error {
	memInfo, err := mem.VirtualMemoryWithContext(ctx)
	if err != nil {
		return err
	}
	s.previous = float64(memInfo.Used)
	return nil
}

// Snapshot implements perf.TimelineDatasource.Snapshot.
func (s *memoryDataSource) Snapshot(ctx context.Context, values *perf.Values) error {
	memInfo, err := mem.VirtualMemoryWithContext(ctx)
	if err != nil {
		return err
	}
	used := float64(memInfo.Used)
	values.Append(perf.Metric{
		Name:      s.diffName,
		Unit:      "bytes",
		Direction: perf.SmallerIsBetter,
		Multiple:  true,
	}, used-s.previous)
	values.Append(perf.Metric{
		Name:      s.name,
		Unit:      "bytes",
		Direction: perf.SmallerIsBetter,
		Multiple:  true,
	}, float64(used))
	values.Append(perf.Metric{
		Name:      s.percentName,
		Unit:      "percent",
		Direction: perf.SmallerIsBetter,
		Multiple:  true,
	}, memInfo.UsedPercent)
	s.previous = used

	return nil
}

// Stop does nothing.
func (s *memoryDataSource) Stop(_ context.Context, values *perf.Values) error {
	return nil
}
