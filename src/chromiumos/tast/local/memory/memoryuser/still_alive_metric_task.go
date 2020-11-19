// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package memoryuser

import (
	"context"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// KillableTask allows querying whether a task has been killed or not.
type KillableTask interface {
	StillAlive(context.Context, *TestEnv) bool
}

// StillAliveMetricTask is a MemoryTask that counts how many of a set of
// KillableTask are still alive, and records it as a perf.Metric.
type StillAliveMetricTask struct {
	tasks      []KillableTask
	name       string
	stillAlive int
}

// StillAliveMetricTask is a MemoryTask.
var _ MemoryTask = (*StillAliveMetricTask)(nil)

// Run actually computes the metric.
func (t *StillAliveMetricTask) Run(ctx context.Context, testEnv *TestEnv) error {
	t.stillAlive = 0
	for _, sa := range t.tasks {
		if sa.StillAlive(ctx, testEnv) {
			t.stillAlive++
		}
	}
	testing.ContextLogf(ctx, "Still alive metric %q: %d", t.name, t.stillAlive)
	testEnv.p.Set(perf.Metric{
		Name:      t.name,
		Unit:      "count",
		Direction: perf.BiggerIsBetter,
	}, float64(t.stillAlive))
	return nil
}

// Close does nothing.
func (t *StillAliveMetricTask) Close(_ context.Context, _ *TestEnv) {
}

// String gives MemoryUser a friendly string for logging.
func (t *StillAliveMetricTask) String() string {
	return "Still Alive Metric " + t.name
}

// NeedVM is false because we do not need a new Crostini VM spun up.
func (t *StillAliveMetricTask) NeedVM() bool {
	return false
}

// StillAlive returns how many tasks were reported to be still alive. Returns
// -1 if this task hasn't run yet.
func (t *StillAliveMetricTask) StillAlive() int {
	return t.stillAlive
}

// NewStillAliveMetricTask creates a new MemoryTask that records a perf.Metric
// for how many of a set of tasks are still alive.
func NewStillAliveMetricTask(tasks []KillableTask, name string) *StillAliveMetricTask {
	return &StillAliveMetricTask{tasks, name, -1}
}

// MinStillAliveMetricTask is a MemoryTask that reports the minimum value
// reported by a slice of StillAliveMetricTasks.
type MinStillAliveMetricTask struct {
	tasks []*StillAliveMetricTask
	name  string
}

// MinStillAliveMetricTask is a MemoryTask.
var _ MemoryTask = (*MinStillAliveMetricTask)(nil)

// Run actually computes the metric.
func (t *MinStillAliveMetricTask) Run(ctx context.Context, testEnv *TestEnv) error {
	if len(t.tasks) < 1 {
		return errors.New("MinStillAliveMetricTask needs at least 1 StillAliveMetricTask")
	}
	min := t.tasks[0].StillAlive()
	for _, sa := range t.tasks[1:] {
		stillAlive := sa.StillAlive()
		if stillAlive < min {
			min = stillAlive
		}
	}
	testing.ContextLogf(ctx, "Min still alive metric %q: %d", t.name, min)
	testEnv.p.Set(perf.Metric{
		Name:      t.name,
		Unit:      "count",
		Direction: perf.BiggerIsBetter,
	}, float64(min))
	return nil
}

// Close does nothing.
func (t *MinStillAliveMetricTask) Close(_ context.Context, _ *TestEnv) {
}

// String gives MemoryUser a friendly string for logging.
func (t *MinStillAliveMetricTask) String() string {
	return "Min Still Alive Metric " + t.name
}

// NeedVM is false because we do not need a new Crostini VM spun up.
func (t *MinStillAliveMetricTask) NeedVM() bool {
	return false
}

// NewMinStillAliveMetricTask creates a new MemoryTask that records a
// perf.Metric for the minimum value of a set of StillAliveMetricsTasks.
func NewMinStillAliveMetricTask(tasks []*StillAliveMetricTask, name string) *MinStillAliveMetricTask {
	return &MinStillAliveMetricTask{tasks, name}
}
