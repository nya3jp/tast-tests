// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package perf

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/memory/metrics"
	"chromiumos/tast/testing"
)

const (
	timeBetweenCheckpoints  = 5 * time.Minute
	timeoutBackgroundScript = 60 * time.Second
)

// MemoryInfoTracker is a helper to collect zram info.
type MemoryInfoTracker struct {
	base          *metrics.BaseMemoryStats
	lastslicebase *metrics.BaseMemoryStats
	arc           *arc.ARC
	stopc         chan struct{}
	stopackc      chan struct{}
	bgpv          *perf.Values
	lasterr       error
}

// NewMemoryTracker creates a new instance of MemoryInfoTracker.
// This will take a snapshot of memory stats, which is used as a base
// to subtract (delta) from future measurements.
func NewMemoryTracker(arcp *arc.ARC) *MemoryInfoTracker {
	// Gather parameters for deferred initialization.
	rv := &MemoryInfoTracker{
		base:     nil,
		arc:      arcp,
		stopc:    make(chan struct{}),
		stopackc: make(chan struct{}),
		bgpv:     perf.NewValues(),
	}

	return rv
}

// takeSnapshot collects traces and resets counters.
func (t *MemoryInfoTracker) takeSnapshot(ctx context.Context, iteration uint32) error {
	cpsuffix := fmt.Sprintf("_cp%d", (iteration + 1))
	slicesuffix := fmt.Sprintf("_slice%d", (iteration + 1))

	// Make a deep copy of the base metrics.
	priorBase := t.base.Clone()

	// Log metrics covering start to this point. Latest values appended to base.
	if err := metrics.LogMemoryStats(ctx, t.base, t.arc, t.bgpv, "", cpsuffix); err != nil {
		return errors.Wrapf(err, "unable to log memory stats at iteration %d", iteration)
	}

	// From the second run onwards.
	// Log metrics for the timespan between last checkpoint and now.
	metrics.LogMemoryStatsSlice(priorBase, t.base, t.bgpv, slicesuffix)

	return nil
}

// Start indicates that periodic memory tracking should start.
func (t *MemoryInfoTracker) Start(ctx context.Context) error {
	var err error
	if t == nil {
		return errors.New("memory tracker is not provided to start")
	}

	if t.base != nil {
		return errors.New("duplicate start detected")
	}

	t.base, err = metrics.NewBaseMemoryStats(ctx, t.arc)
	if err != nil {
		return errors.Wrap(err, "unable to retrieve base memory stats")
	}

	go func() {
		var iteration uint32
		testing.ContextLog(ctx, "mem_tracker: Background collection started, will sleep")
		done := false

		// Keep taking snapshots at regular intervals until told to stop
		for !done {
			select {
			case <-time.After(timeBetweenCheckpoints):
				testing.ContextLog(ctx, "mem_tracker: Will take snapshot")
				t.lasterr = t.takeSnapshot(ctx, iteration)
				if t.lasterr != nil {
					testing.ContextLog(ctx, "Error: failed to collect memory stats, stopping collection: ", err)
					done = true
				} else {
					testing.ContextLog(ctx, "mem_tracker: Snapshot OK, will sleep")
				}
			case <-t.stopc:
				testing.ContextLog(ctx, "mem_tracker: Background signaled to stop")
				done = true
			case <-ctx.Done():
				done = true
			}
			iteration++
		}

		// Let the foreground task know we are done.
		close(t.stopackc)
	}()

	return nil
}

// Stop indicates that periodic memory tracking must stop.
// It will also log metrics collected during test.
func (t *MemoryInfoTracker) Stop(ctx context.Context) error {
	ctxIsFinished := false
	if t == nil {
		return errors.New("memory tracker is not provided to stop")
	}

	if t.base == nil {
		return errors.New("memory tracker is not started")
	}

	close(t.stopc)

	// Wait for confirmation that background collection stopped.
	select {
	case <-time.After(timeoutBackgroundScript):
		return errors.New("timed out waiting for background memory script collection")
	case <-t.stopackc:
		testing.ContextLog(ctx, "mem_tracker: Background collection stopped gracefully")
		break
	case <-ctx.Done():
		ctxIsFinished = true
		break
	}

	if t.lasterr != nil {
		return errors.Wrap(t.lasterr, "background memory logging failed")
	}

	if !ctxIsFinished {
		// Collect one last time - this is for the whole test's span.
		return metrics.LogMemoryStats(ctx, t.base, t.arc, t.bgpv, "", "_final")
	}
	return nil
}

// Record stores the collected data into pv for further processing.
func (t *MemoryInfoTracker) Record(pv *perf.Values) {
	// Add all results collected previously in the background.
	pv.Merge(t.bgpv)
}
