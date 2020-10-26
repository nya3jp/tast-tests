// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cuj

import (
	"context"
	"time"

	"github.com/shirou/gopsutil/mem"

	"chromiumos/tast/common/perf"
)

// diffWait is the default duration to measure the baseline of memoryDiffDataSource.
const diffWait = 5 * time.Second

// memoryDiffDataSource is a perf.TimelineDatasource reporting the memory usage diff from certain point.
type memoryDiffDataSource struct {
	name     string
	previous float64
}

// newMemoryDiffDataSource creates a new instance of memoryDiffDataSource with the
// given name.
func newMemoryDiffDataSource(name string) *memoryDiffDataSource {
	return &memoryDiffDataSource{name: name}
}

func (s *memoryDiffDataSource) SetPrevious(ctx context.Context) error {
	memInfo, err := mem.VirtualMemoryWithContext(ctx)
	if err != nil {
		return err
	}
	s.previous = float64(memInfo.Used)
	return nil
}

// Setup implements perf.TimelineDatasource.Setup.
func (s *memoryDiffDataSource) Setup(ctx context.Context, prefix string) error {
	s.name = prefix + s.name
	return nil
}

// Start implements perf.TimelineDatasource.Start.
func (s *memoryDiffDataSource) Start(ctx context.Context) error {
	return nil
}

// Snapshot implements perf.TimelineDatasource.Snapshot.
func (s *memoryDiffDataSource) Snapshot(ctx context.Context, values *perf.Values) error {
	memInfo, err := mem.VirtualMemoryWithContext(ctx)
	if err != nil {
		return err
	}
	used := float64(memInfo.Used)
	values.Append(perf.Metric{
		Name:      s.name,
		Unit:      "bytes",
		Direction: perf.SmallerIsBetter,
		Multiple:  true,
	}, used-s.previous)
	s.previous = used

	return nil
}
