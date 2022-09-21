// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"context"
	"regexp"
	"strconv"
	"strings"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
)

// CPUUsageJiffies stores a snapshot of total (load+idle) and load jiffy
// counters from /proc/stat.
type CPUUsageJiffies struct {
	total int64
	// load is the sum of user, nice, system, iowait, irq, softirq, steal,
	// guest.
	load int64
}

// ProcfsCPUMetrics holds the CPU metrics read from procfs.
type ProcfsCPUMetrics struct {
	metric      perf.Metric
	lastJiffies CPUUsageJiffies
}

// Assert that ProcfsCPUMetrics can be used in perf.Timeline.
var _ perf.TimelineDatasource = &ProcfsCPUMetrics{}

// NewProcfsCPUMetrics creates a struct to capture CPU metrics with procfs.
func NewProcfsCPUMetrics() *ProcfsCPUMetrics {
	return &ProcfsCPUMetrics{}
}

// Setup creates the metric.
func (c *ProcfsCPUMetrics) Setup(ctx context.Context, prefix string) error {
	c.metric = perf.Metric{Name: "cpu_usage", Unit: "ratio", Direction: perf.SmallerIsBetter, Multiple: true}
	return nil
}

// Start takes the first snapshot of CPU metrics.
func (c *ProcfsCPUMetrics) Start(ctx context.Context) error {
	jiffies, err := readJiffies(ctx)
	if err != nil {
		return errors.Wrap(err, "unable to read CPU usage from /proc/stat")
	}

	c.lastJiffies = jiffies
	return nil
}

// readJiffies reads CPU load and calculates total jiffies from /proc/stat.
// This file contains a line for each CPU (core) and an aggregated value of
// all CPUs. The aggregated value (first line) is used in readJiffies.
// Therefore, a reported CPU load value of 1.0 indicates 100% CPU usage on all
// CPUs.
func readJiffies(ctx context.Context) (CPUUsageJiffies, error) {
	line, err := readFirstLine(ctx, "/proc/stat")
	if err != nil {
		return CPUUsageJiffies{}, err
	}

	// Remove duplicate whitespace.
	spaceRegexp := regexp.MustCompile(`\s+`)
	p := strings.Split(spaceRegexp.ReplaceAllString(line, " "), " ")

	var load int64
	var total int64
	// Line format (number of columns depends on kernel version):
	// cpu_id user nice system idle iowait irq softirq steal guest
	for i := 1; i < len(p); i++ {
		v, err := strconv.ParseInt(p[i], 10, 64)
		if err != nil {
			return CPUUsageJiffies{}, errors.Wrapf(err, "unexpected token in /proc/stat: %s", p[i])
		}

		total += v
		if i != 4 {
			load += v
		}
	}

	return CPUUsageJiffies{total: total, load: load}, nil
}

// Snapshot takes a snapshot of CPU metrics.
func (c *ProcfsCPUMetrics) Snapshot(ctx context.Context, values *perf.Values) error {
	jiffies, err := readJiffies(ctx)
	if err != nil {
		return errors.Wrap(err, "unable to read CPU usage from /proc/stat")
	}

	used := float64(c.lastJiffies.load - jiffies.load)
	total := float64(c.lastJiffies.total - jiffies.total)
	values.Append(c.metric, used/total)

	c.lastJiffies = jiffies
	return nil
}

// Stop does nothing.
func (c *ProcfsCPUMetrics) Stop(ctx context.Context, values *perf.Values) error {
	return nil
}
