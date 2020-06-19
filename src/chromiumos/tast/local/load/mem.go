// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package load

import (
	"context"
	"fmt"

	"github.com/shirou/gopsutil/mem"

	"chromiumos/tast/common/perf"
)

// MemoryUsageSource is a perf.TimelineDatasource reporting the memory usage.
type MemoryUsageSource struct {
	name string
}

// NewMemoryUsageSource creates a new instance of MemoryUsageSource with the
// given name.
func NewMemoryUsageSource(name string) *MemoryUsageSource {
	return &MemoryUsageSource{name: name}
}

// Setup implements perf.TimelineDatasource.Setup.
func (s *MemoryUsageSource) Setup(ctx context.Context, prefix string) error {
	if prefix != "" {
		s.name = fmt.Sprintf("%s.%s", prefix, s.name)
	}
	return nil
}

// Start implements perf.TimelineDatasource.Start.
func (s *MemoryUsageSource) Start(ctx context.Context) error {
	return nil
}

// Snapshot implements perf.TimelineDatasource.Snapshot.
func (s *MemoryUsageSource) Snapshot(ctx context.Context, values *perf.Values) error {
	memInfo, err := mem.VirtualMemoryWithContext(ctx)
	if err != nil {
		return err
	}
	values.Append(perf.Metric{
		Name:      s.name,
		Unit:      "percent",
		Direction: perf.LowerIsBetter,
		Multiple:  true,
	}, memInfo.UsedPercent)

	return nil
}
