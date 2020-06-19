// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package load

import (
	"context"
	"fmt"

	"github.com/shirou/gopsutil/cpu"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
)

// CPUUsageSource is an implementation of perf.TimelineDataSource which reports
// the CPU usage.
type CPUUsageSource struct {
	name     string
	perCPU   bool
	cpuCount int
}

// NewCPUUsageSource creates a new instance of CPUUsageSource for the given
// metric name. If perCPU is true, the reports are recorded per-CPU (each report
// with variant). Otherwise, it reports the overall CPU usage.
func NewCPUUsageSource(name string, perCPU bool) *CPUUsageSource {
	if name == "" {
		name = "CPUUsage"
	}
	return &CPUUsageSource{name: name, perCPU: perCPU}
}

// Setup implements perf.TimelineDatasource.Setup.
func (s *CPUUsageSource) Setup(ctx context.Context, prefix string) error {
	if prefix != "" {
		s.name = fmt.Sprintf("%s.%s", prefix, s.name)
	}
	return nil
}

// Start implements perf.TimelineDatasource.Start.
func (s *CPUUsageSource) Start(ctx context.Context) error {
	percents, err := cpu.PercentWithContext(ctx, 0, s.perCPU)
	if err != nil {
		return err
	}
	s.cpuCount = len(percents)
	return nil
}

// Snapshot implements perf.TimelineDatasource.Snapshot.
func (s *CPUUsageSource) Snapshot(ctx context.Context, values *perf.Values) error {
	percents, err := cpu.PercentWithContext(ctx, 0, s.perCPU)
	if err != nil {
		return err
	}
	if len(percents) != s.cpuCount {
		return errors.Errorf("the number of cpu has changed, originally %d but now %d", s.cpuCount, len(percents))
	}
	if s.perCPU {
		for i, percent := range percents {
			values.Append(perf.Metric{
				Name:      s.name,
				Variant:   fmt.Sprintf("cpu-%d", i),
				Multiple:  true,
				Unit:      "percent",
				Direction: perf.SmallerIsBetter,
			}, percent)
		}
	} else {
		values.Append(perf.Metric{
			Name:      s.name,
			Multiple:  true,
			Unit:      "percent",
			Direction: perf.SmallerIsBetter,
		}, percents[0])
	}
	return nil
}
