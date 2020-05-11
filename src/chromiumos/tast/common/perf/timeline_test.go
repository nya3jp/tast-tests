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
	snapshotSleepDuration                           time.Duration
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

func (t *testTimelineDatasource) Snapshot(ctx context.Context, v *Values) error {
	tasttesting.Sleep(ctx, t.snapshotSleepDuration)
	if t.errSnapshot {
		return errSnapshot
	}
	t.snapshotCount++
	return nil
}

func (t *testTimelineDatasource) WaitForSnapshots(ctx context.Context, count int) error {
	return tasttesting.Poll(ctx, func(ctx context.Context) error {
		if t.snapshotCount < count {
			return errors.New("still waiting")
		}
		return nil
	}, &tasttesting.PollOptions{Interval: 10 * time.Millisecond})
}

type fakeTimer struct {
	clock      int64
	isSleeping bool
}

func (t *fakeTimer) Sleep(ctx context.Context, d time.Duration) error {
	timeBefore := t.clock
	timeAfter := timeBefore + d.Milliseconds()

	err := tasttesting.Poll(
		ctx,
		func(ctx context.Context) error {
			if t.clock < timeAfter {
				t.isSleeping = true
				return errors.New("still waiting")
			}
			return nil
		}, &tasttesting.PollOptions{Interval: 10 * time.Millisecond})

	t.isSleeping = false
	return err
}

func (t *fakeTimer) AdvanceClock(d time.Duration) {
	t.clock = t.clock + d.Milliseconds()
}

func (t *fakeTimer) WaitForSleep(ctx context.Context) error {
	return tasttesting.Poll(ctx, func(ctx context.Context) error {
		if !t.isSleeping {
			return errors.New("not sleeping yet")
		}
		return nil
	}, &tasttesting.PollOptions{Interval: 10 * time.Millisecond})
}

func TestTimeline(t *testing.T) {
	p := NewValues()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	timer := &fakeTimer{}

	d1 := &testTimelineDatasource{}
	d2 := &testTimelineDatasource{}

	tl, err := NewTimeline(ctx, []TimelineDatasource{d1, d2}, Interval(1*time.Second), WithTimer(timer))
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

	// Take 2 samples.
	timer.WaitForSleep(ctx)
	for i := 0; i < 2; i++ {
		timer.AdvanceClock(1200 * time.Millisecond)
		d1.WaitForSnapshots(ctx, i+1)
		timer.WaitForSleep(ctx)
	}

	if v, err := tl.StopRecording(); err != nil {
		t.Error("Error while recording: ", err)
	} else {
		p.Merge(v)
	}

	if d1.snapshotCount != 2 || d2.snapshotCount != 2 {
		t.Errorf("Wrong number of snapshots collected: %d, %d", d1.snapshotCount, d2.snapshotCount)
	}

	// Second round of recording.
	if err := tl.StartRecording(ctx); err != nil {
		t.Error("Failed to start recording: ", err)
	}

	// Take 3 more samples.
	timer.WaitForSleep(ctx)
	for i := 2; i < 5; i++ {
		timer.AdvanceClock(1200 * time.Millisecond)
		d1.WaitForSnapshots(ctx, i+1)
		timer.WaitForSleep(ctx)
	}

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

func TestTimelineSlowSnapshot(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	d := &testTimelineDatasource{snapshotSleepDuration: 500 * time.Millisecond}

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

	tasttesting.Sleep(ctx, 1500*time.Millisecond)

	if _, err := tl.StopRecording(); err == nil {
		t.Error("StopRecording should have failed")
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
