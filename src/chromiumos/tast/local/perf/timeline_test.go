// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package perf

import (
	"context"
	"testing"

	"chromiumos/tast/errors"
)

type testTimelineDatasource struct {
	setUp, started, errSetup, errStart, errSnapshot bool
	snapshotCount                                   int
}

var errSetup = errors.New("setup should fail")

func (t *testTimelineDatasource) Setup(_ context.Context) error {
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

func TestTimeline(t *testing.T) {
	p := NewValues()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	d1 := &testTimelineDatasource{}
	d2 := &testTimelineDatasource{}

	tl, err := NewTimeline(ctx, d1, d2)
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

	if err := tl.Snapshot(ctx, p); err != nil {
		t.Error("Failed to snapshot timeline: ", err)
	}
	if d1.snapshotCount != 1 || d2.snapshotCount != 1 {
		t.Error("Wrong number of snapshots collected")
	}

	if err := tl.Snapshot(ctx, p); err != nil {
		t.Error("Failed to snapshot timeline: ", err)
	}
	if d1.snapshotCount != 2 || d2.snapshotCount != 2 {
		t.Error("Wrong number of snapshots collected")
	}

	timestamps := p.values[timestampMetric]
	if len(timestamps) != 2 {
		t.Fatalf("Wrong number of timestamps logged, got %d, expected 2", len(timestamps))
	}
	if timestamps[0] > timestamps[1] {
		t.Error("Timestamps logged in wrong order")
	}
}

func TestTimelineNoStart(t *testing.T) {
	p := NewValues()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tl, err := NewTimeline(ctx)
	if err != nil {
		t.Error("Failed to create Timeline: ", err)
	}

	if err := tl.Snapshot(ctx, p); err == nil {
		t.Error("Snapshot should have failed without calling Start first")
	}
}

func TestTimelineSetupFail(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	d := &testTimelineDatasource{errSetup: true}

	if _, err := NewTimeline(ctx, d); err == nil {
		t.Error("NewTimeline should have failed because of setup failure")
	}
}

func TestTimelineStartFail(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	d := &testTimelineDatasource{errStart: true}

	tl, err := NewTimeline(ctx, d)
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

	tl, err := NewTimeline(ctx, d)
	if err != nil {
		t.Error("Failed to create Timeline: ", err)
	}

	if err := tl.Start(ctx); err != nil {
		t.Error("Failed to start timeline: ", err)
	}

	if err := tl.Snapshot(ctx, p); err == nil {
		t.Error("Snapshot should have failed")
	}
}
