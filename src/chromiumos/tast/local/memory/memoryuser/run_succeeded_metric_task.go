// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package memoryuser

import (
	"context"

	"chromiumos/tast/common/perf"
)

// SilentFailTask allows tasks to report if they successfully ran. Used by tasks
// that are allowed to fail without stopping the test.
type SilentFailTask interface {
	// Succeeded returns true iff MemoryTask.Run completed without errors.
	Succeeded() bool
}

// RunSucceededMetricTask computes how many of a set of SilentFailTasks
// succeeded.
type RunSucceededMetricTask struct {
	tasks []SilentFailTask
	name  string
}

// RunSucceededMetricTask is a MemoryTask.
var _ MemoryTask = (*RunSucceededMetricTask)(nil)

// Run actually computes the metric.
func (t *RunSucceededMetricTask) Run(ctx context.Context, testEnv *TestEnv) error {
	var succeeded float64
	for _, sf := range t.tasks {
		if sf.Succeeded() {
			succeeded += 1.0
		}
	}
	testEnv.p.Set(perf.Metric{
		Name:      t.name,
		Unit:      "count",
		Direction: perf.BiggerIsBetter,
	}, succeeded)
	return nil
}

// Close does nothing.
func (t *RunSucceededMetricTask) Close(_ context.Context, _ *TestEnv) {
}

// String give MemoryUser a friendly string for logging.
func (t *RunSucceededMetricTask) String() string {
	return "Run Succeeded Metric " + t.name
}

// NeedVM is false because we do not need a new Crostini VM spun up.
func (t *RunSucceededMetricTask) NeedVM() bool {
	return false
}

// NewRunSucceededMetricTask creates a new MemoryTask that records a perf.Metric
// for how many of a set of tasks ran successfully.
func NewRunSucceededMetricTask(tasks []SilentFailTask, name string) *RunSucceededMetricTask {
	return &RunSucceededMetricTask{tasks, name}
}
