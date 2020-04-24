// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package perf

import (
	"context"
	"testing"
	"time"

	"chromiumos/tast/errors"
	tasttesting "chromiumos/tast/testing"
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

func TestTimeline(t *testing.T) {
	p := NewValues()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	d1 := &testTimelineDatasource{}
	d2 := &testTimelineDatasource{}

	tl, err := NewTimeline(ctx, []TimelineDatasource{d1, d2}, Interval(200*time.Millisecond))
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

	// First round of recording.
	if err := tl.StartRecording(ctx); err != nil {
		t.Error("Failed to start recording: ", err)
	}

	tasttesting.Sleep(ctx, 450*time.Millisecond)

	if v, err := tl.StopRecording(); err != nil {
		t.Error("Error while recording: ", err)
	} else {
		p.Merge(v)
	}

	if d1.snapshotCount != 2 || d2.snapshotCount != 2 {
		t.Error("Wrong number of snapshots collected")
	}

	// Second round of recording.
	if err := tl.StartRecording(ctx); err != nil {
		t.Error("Failed to start recording: ", err)
	}

	tasttesting.Sleep(ctx, 650*time.Millisecond)

	if v, err := tl.StopRecording(); err != nil {
		t.Error("Error while recording: ", err)
	} else {
		p.Merge(v)
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
	for i := 0; i < len(timestamps)-1; i++ {
		if timestamps[i] > timestamps[i+1] {
			t.Errorf("Timestamps logged in wrong order, expected %f < %f", timestamps[i], timestamps[i+1])
		}
	}
}

func TestTimelineStartRecordingTwice(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	d := &testTimelineDatasource{}

	tl, err := NewTimeline(ctx, []TimelineDatasource{d}, Interval(200*time.Millisecond))
	if err != nil {
		t.Error("Failed to create Timeline: ", err)
	}

	if err := tl.Start(ctx); err != nil {
		t.Error("Failed to start timeline: ", err)
	}

	if err := tl.StartRecording(ctx); err != nil {
		t.Error("Failed to start recording: ", err)
	}

	if err := tl.StartRecording(ctx); err == nil {
		t.Error("StartRecording should have failed")
	}
}

func TestTimelineNoStart(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tl, err := NewTimeline(ctx, []TimelineDatasource{}, Interval(1*time.Millisecond))
	if err != nil {
		t.Error("Failed to create Timeline: ", err)
	}

	if err := tl.StartRecording(ctx); err != nil {
		t.Error("Failed to start recording: ", err)
	}

	tasttesting.Sleep(ctx, 50*time.Millisecond)

	if _, err := tl.StopRecording(); err == nil {
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

	if err := tl.StartRecording(ctx); err != nil {
		t.Error("Failed to start recording: ", err)
	}

	tasttesting.Sleep(ctx, 50*time.Millisecond)

	if _, err := tl.StopRecording(); err == nil {
		t.Error("Snapshot should have failed")
	}
}
