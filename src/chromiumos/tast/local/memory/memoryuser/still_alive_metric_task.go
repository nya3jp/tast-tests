// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package memoryuser

import (
	"context"

	"chromiumos/tast/common/perf"
)

// KillableTask allows querying whether a task has been killed or not.
type KillableTask interface {
	StillAlive(context.Context, *TestEnv) bool
}

// StillAliveMetricTask is a MemoryTask that counts how many of a set of
// KillableTask are still alive, and records it as a perf.Metric.
type StillAliveMetricTask struct {
	tasks []KillableTask
	name  string
}

// StillAliveMetricTask is a MemoryTask.
var _ MemoryTask = (*StillAliveMetricTask)(nil)

// Run actually computes the metric.
func (t *StillAliveMetricTask) Run(ctx context.Context, testEnv *TestEnv) error {
	var stillAlive float64
	for _, sa := range t.tasks {
		if sa.StillAlive(ctx, testEnv) {
			stillAlive += 1.0
		}
	}
	testEnv.p.Set(perf.Metric{
		Name:      t.name,
		Unit:      "count",
		Direction: perf.BiggerIsBetter,
	}, stillAlive)
	return nil
}

// Close does nothing.
func (t *StillAliveMetricTask) Close(_ context.Context, _ *TestEnv) {
}

// String give MemoryUser a friendly string for logging.
func (t *StillAliveMetricTask) String() string {
	return "Still Alive Metric " + t.name
}

// NeedVM is false because we do not need a new Crostini VM spun up.
func (t *StillAliveMetricTask) NeedVM() bool {
	return false
}

// NewStillAliveMetricTask creates a new MemoryTask that records a perf.Metric
// for how many of a set of tasks are still alive.
func NewStillAliveMetricTask(tasks []KillableTask, name string) *StillAliveMetricTask {
	return &StillAliveMetricTask{tasks, name}
}
