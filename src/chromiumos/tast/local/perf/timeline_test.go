// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package perf

import (
	"context"
	"testing"
	"time"

	"chromiumos/tast/errors"
)

type testTimelineDatasource struct {
	setUp, started, errSetup, errStart, errSnapshot bool
	snapshotCount                                   int
}

var errSetup = errors.New("setup should fail")

func (t *testTimelineDatasource) Setup(_ context.Context, _ string) error {
	if t.errSetup {
		return errSetup
	}
	t.setUp = true
	return nil
}

var errStart = errors.New("start should fail")

func (t *testTimelineDatasource) Start(_ context.Context) error {
	if t.errStart {
		return errStart
	}
	t.started = true
	return nil
}

var errSnapshot = errors.New("snapshot should fail")

func (t *testTimelineDatasource) Snapshot(_ context.Context, v *Values) error {
	if t.errSnapshot {
		return errSnapshot
	}
	t.snapshotCount++
	return nil
}

func TestTimelineCaptureTimePeriod(t *testing.T) {
	p := NewValues()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	d1 := &testTimelineDatasource{}
	d2 := &testTimelineDatasource{}

	tl, err := NewTimeline(ctx, []TimelineDatasource{d1, d2}, Interval(1*time.Millisecond))
	if err != nil {
		t.Error("Failed to create Timeline: ", err)
	}
	if !d1.setUp || !d2.setUp {
		t.Error("Failed to set up both datasources")
	}

	if err := tl.Start(ctx); err != nil {
		t.Error("Failed to start timeline: ", err)
	}
	if !d1.started || !d2.started {
		t.Error("Failed to call start on both datasources")
	}

	if err := tl.CaptureTimePeriod(ctx, p, 2*time.Millisecond); err != nil {
		t.Error("Failed to snapshot timeline: ", err)
	}
	if d1.snapshotCount != 2 || d2.snapshotCount != 2 {
		t.Error("Wrong number of snapshots collected")
	}

	if err := tl.CaptureTimePeriod(ctx, p, 3*time.Millisecond); err != nil {
		t.Error("Failed to snapshot timeline: ", err)
	}
	if d1.snapshotCount != 5 || d2.snapshotCount != 5 {
		t.Error("Wrong number of snapshots collected")
	}

	var timestamps []float64
	for k, v := range p.values {
		if k.Name == "t" {
			timestamps = v
		}
	}
	if timestamps == nil {
		t.Fatal("Could not find timestamps metric")
	}
	if len(timestamps) != 5 {
		t.Fatalf("Wrong number of timestamps logged, got %d, expected 5", len(timestamps))
	}
	if timestamps[0] > timestamps[1] {
		t.Error("Timestamps logged in wrong order")
	}
}

func TestTimelineCaptureWhile(t *testing.T) {
	const (
		numIterations    = 5
		iteationDuration = 100 * time.Millisecond
	)

	p := NewValues()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	d1 := &testTimelineDatasource{}
	d2 := &testTimelineDatasource{}

	tl, err := NewTimeline(ctx, []TimelineDatasource{d1, d2}, Interval(iteationDuration))
	if err != nil {
		t.Error("Failed to create Timeline: ", err)
	}
	if !d1.setUp || !d2.setUp {
		t.Error("Failed to set up both datasources")
	}

	if err := tl.Start(ctx); err != nil {
		t.Error("Failed to start timeline: ", err)
	}
	if !d1.started || !d2.started {
		t.Error("Failed to call start on both datasources")
	}

	iterations := 0
	timeBefore := time.Now()

	err = tl.CaptureWhile(ctx, p, func() (bool, error) {
		iterations = iterations + 1
		return iterations <= numIterations, nil
	})
	timeDuration := time.Now().Sub(timeBefore)

	if err != nil {
		t.Error("Failed to snapshot timeline: ", err)
	}

	if d1.snapshotCount != numIterations || d2.snapshotCount != numIterations {
		t.Error("Wrong number of snapshots collected")
	}

	if timeDuration < numIterations*iteationDuration {
		t.Error("Snapshots collected too fast")
	}

	var timestamps []float64
	for k, v := range p.values {
		if k.Name == "t" {
			timestamps = v
		}
	}
	if timestamps == nil {
		t.Fatal("Could not find timestamps metric")
	}
	if len(timestamps) != numIterations {
		t.Fatalf("Wrong number of timestamps logged, got %d, expected %d", len(timestamps), numIterations)
	}
	if timestamps[0] > timestamps[1] {
		t.Error("Timestamps logged in wrong order")
	}
}

func TestTimelineNoStart(t *testing.T) {
	p := NewValues()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tl, err := NewTimeline(ctx, []TimelineDatasource{}, Interval(1*time.Millisecond))
	if err != nil {
		t.Error("Failed to create Timeline: ", err)
	}

	if err := tl.CaptureTimePeriod(ctx, p, 2*time.Millisecond); err == nil {
		t.Error("Snapshot should have failed without calling Start first")
	}
}

func TestTimelineSetupFail(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	d := &testTimelineDatasource{errSetup: true}

	if _, err := NewTimeline(ctx, []TimelineDatasource{d}); err == nil {
		t.Error("NewTimeline should have failed because of setup failure")
	}
}

func TestTimelineStartFail(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	d := &testTimelineDatasource{errStart: true}

	tl, err := NewTimeline(ctx, []TimelineDatasource{d})
	if err != nil {
		t.Error("Failed to create Timeline: ", err)
	}

	if err := tl.Start(ctx); err == nil {
		t.Error("Start should have failed")
	}
}

func TestTimelineSnapshotFail(t *testing.T) {
	p := NewValues()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	d := &testTimelineDatasource{errSnapshot: true}

	tl, err := NewTimeline(ctx, []TimelineDatasource{d}, Interval(1*time.Millisecond))
	if err != nil {
		t.Error("Failed to create Timeline: ", err)
	}

	if err := tl.Start(ctx); err != nil {
		t.Error("Failed to start timeline: ", err)
	}

	if err := tl.CaptureTimePeriod(ctx, p, 2*time.Millisecond); err == nil {
		t.Error("Snapshot should have failed")
	}
}
