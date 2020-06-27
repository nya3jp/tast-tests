// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cuj

import (
	"context"
	"time"

	"github.com/shirou/gopsutil/mem"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// diffWait is the default duration to measure the baseline of memoryDiffDataSource.
const diffWait = 5 * time.Second

// memoryDiffDataSource is a perf.TimelineDatasource reporting the memory usage diff from certain point.
type memoryDiffDataSource struct {
	name     string
	baseline float64
}

// newMemoryDiffDataSource creates a new instance of memoryDiffDataSource with the
// given name.
func newMemoryDiffDataSource(name string) *memoryDiffDataSource {
	return &memoryDiffDataSource{name: name}
}

func (s *memoryDiffDataSource) PrepareBaseline(ctx context.Context, duration time.Duration) error {
	const interval = 100 * time.Millisecond

	// First, just wait for the half of duration to stabilize the baseline.
	if err := testing.Sleep(ctx, duration/2); err != nil {
		return errors.Wrap(err, "failed to stabilize")
	}
	var sum float64
	var count int

	// This uses testing.Poll to run a task continuously for a certain duration.
	// Since the task does finish with nil error, it should always finish with an
	// error. Failing with errContinuePolling is the expected way to finish.
	errContinuePolling := errors.New("continue polling")
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		memInfo, err := mem.VirtualMemoryWithContext(ctx)
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to obtain the memory stat"))
		}
		sum += memInfo.UsedPercent
		count++
		return errContinuePolling
	}, &testing.PollOptions{Timeout: duration / 2, Interval: interval}); err != nil && !errors.Is(err, errContinuePolling) {
		return errors.Wrap(err, "failed to collect the baseline data")
	}
	if ctx.Err() != nil {
		return errors.Wrap(ctx.Err(), "failed with main context failure")
	}
	if count == 0 {
		return errors.New("failed to collect the baseline data")
	}
	s.baseline = sum / float64(count)
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
	values.Append(perf.Metric{
		Name:      s.name,
		Unit:      "percent",
		Direction: perf.SmallerIsBetter,
		Multiple:  true,
	}, memInfo.UsedPercent-s.baseline)

	return nil
}
