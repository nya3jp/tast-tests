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
	clock                                           *fakeClock
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
	// Note: This function runs in a separate goroutine, so we have to be careful when accessing state that is also accessed by the main goroutine.

	if t.clock != nil {
		t.clock.AdvanceClock(t.snapshotSleepDuration)
	}

	if t.errSnapshot {
		return errSnapshot
	}

	t.snapshotCount++
	return nil
}

func WaitForSnapshots(ctx context.Context, count int, d *testTimelineDatasource, t *Timeline) error {
	return tasttesting.Poll(ctx, func(ctx context.Context) error {
		if t.RecordingStatus() != nil {
			// Recording has stopped. We can stop waiting.
			return nil
		}
		if d.snapshotCount < count {
			return errors.New("still waiting")
		}
		return nil
	}, &tasttesting.PollOptions{Interval: 10 * time.Millisecond})
}

type fakeClock struct {
	clock      time.Time
	isSleeping bool
}

func NewFakeClock() *fakeClock {
	return &fakeClock{clock: time.Now()}
}

func (t *fakeClock) Sleep(ctx context.Context, d time.Duration) error {
	targetTime := t.clock.Add(d)

	err := tasttesting.Poll(
		ctx,
		func(ctx context.Context) error {
			if t.clock.Before(targetTime) {
				t.isSleeping = true
				return errors.New("still waiting")
			}
			return nil
		}, &tasttesting.PollOptions{Interval: 10 * time.Millisecond})

	t.isSleeping = false
	return err
}

func (t *fakeClock) Now() time.Time {
	return t.clock
}

func (t *fakeClock) AdvanceClock(d time.Duration) {
	t.clock = t.clock.Add(d)
}

func WaitForSleep(ctx context.Context, c *fakeClock, t *Timeline) error {
	return tasttesting.Poll(ctx, func(ctx context.Context) error {
		if t.RecordingStatus() != nil {
			// Recording has stopped. We can stop waiting.
			return nil
		}
		if !c.isSleeping {
			return errors.New("not sleeping yet")
		}
		return nil
	}, &tasttesting.PollOptions{Interval: 10 * time.Millisecond})
}

func TestTimeline(t *testing.T) {
	p := NewValues()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	clock := NewFakeClock()

	d1 := &testTimelineDatasource{}
	d2 := &testTimelineDatasource{}

	tl, err := NewTimeline(ctx, []TimelineDatasource{d1, d2}, Interval(1*time.Second), WithClock(clock))
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
	WaitForSleep(ctx, clock, tl)
	for i := 0; i < 2; i++ {
		clock.AdvanceClock(1200 * time.Millisecond)
		WaitForSnapshots(ctx, i+1, d1, tl)
		WaitForSleep(ctx, clock, tl)
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
	WaitForSleep(ctx, clock, tl)
	for i := 2; i < 5; i++ {
		clock.AdvanceClock(1400 * time.Millisecond)
		WaitForSnapshots(ctx, i+1, d1, tl)
		WaitForSleep(ctx, clock, tl)
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

	clock := NewFakeClock()
	d := &testTimelineDatasource{snapshotSleepDuration: 500 * time.Millisecond, clock: clock}

	tl, err := NewTimeline(ctx, []TimelineDatasource{d}, Interval(200*time.Millisecond), WithClock(clock))
	if err != nil {
		t.Error("Failed to create Timeline: ", err)
	}

	if err := tl.Start(ctx); err != nil {
		t.Error("Failed to start timeline: ", err)
	}

	if err := tl.StartRecording(ctx); err != nil {
		t.Error("Failed to start recording: ", err)
	}

	// Take 2 samples.
	WaitForSleep(ctx, clock, tl)
	for i := 0; i < 2; i++ {
		clock.AdvanceClock(250 * time.Millisecond)
		WaitForSnapshots(ctx, i+1, d, tl)
		WaitForSleep(ctx, clock, tl)
	}

	if _, err := tl.StopRecording(); err == nil {
		t.Error("StopRecording should have failed")
	}
}

func TestTimelineNoStart(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	clock := NewFakeClock()
	d := &testTimelineDatasource{}

	tl, err := NewTimeline(ctx, []TimelineDatasource{d}, Interval(1*time.Second), WithClock(clock))
	if err != nil {
		t.Error("Failed to create Timeline: ", err)
	}

	if err := tl.StartRecording(ctx); err != nil {
		t.Error("Failed to start recording: ", err)
	}

	// Take 2 samples. Recording already stops after the taking the first sample.
	WaitForSleep(ctx, clock, tl)
	for i := 0; i < 2; i++ {
		clock.AdvanceClock(1100 * time.Millisecond)
		WaitForSnapshots(ctx, i+1, d, tl)
		WaitForSleep(ctx, clock, tl)
	}

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

	clock := NewFakeClock()
	d := &testTimelineDatasource{errSnapshot: true}

	tl, err := NewTimeline(ctx, []TimelineDatasource{d}, Interval(1*time.Second))
	if err != nil {
		t.Error("Failed to create Timeline: ", err)
	}

	if err := tl.Start(ctx); err != nil {
		t.Error("Failed to start timeline: ", err)
	}

	if err := tl.StartRecording(ctx); err != nil {
		t.Error("Failed to start recording: ", err)
	}

	// Take 2 samples. Recording already stops after the taking the first sample.
	WaitForSleep(ctx, clock, tl)
	for i := 0; i < 2; i++ {
		clock.AdvanceClock(1100 * time.Millisecond)
		WaitForSnapshots(ctx, i+1, d, tl)
		WaitForSleep(ctx, clock, tl)
	}

	if _, err := tl.StopRecording(); err == nil {
		t.Error("Snapshot should have failed")
	}
}
