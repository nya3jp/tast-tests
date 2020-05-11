// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package perf

import (
	"context"
	"testing"
	"time"

	"golang.org/x/sync/semaphore"

	"chromiumos/tast/errors"
)

type testTimelineDatasource struct {
	setUp, started, errSetup, errStart, errSnapshot bool
	snapshotCount                                   int
	snapshotChannel                                 chan int
	snapshotDuration                                time.Duration
	clock                                           *fakeClock
}

func newDatasource() *testTimelineDatasource {
	ds := &testTimelineDatasource{}
	ds.snapshotChannel = make(chan int, 100)
	return ds
}

var errSetup = errors.New("setup should fail")

func (d *testTimelineDatasource) Setup(_ context.Context, _ string) error {
	if d.errSetup {
		return errSetup
	}
	d.setUp = true
	return nil
}

var errStart = errors.New("start should fail")

func (d *testTimelineDatasource) Start(_ context.Context) error {
	if d.errStart {
		return errStart
	}
	d.started = true
	return nil
}

var errSnapshot = errors.New("snapshot should fail")

func (d *testTimelineDatasource) Snapshot(ctx context.Context, v *Values) error {
	// Note: This function runs in a separate goroutine, so we have to be careful when accessing state that is also accessed by the main goroutine.

	if d.clock != nil {
		d.clock.Advance(d.snapshotDuration)
	}

	if d.errSnapshot {
		return errSnapshot
	}

	d.snapshotCount++
	d.snapshotChannel <- d.snapshotCount - 1
	return nil
}

func (d *testTimelineDatasource) WaitForSnapshot(t *Timeline) {
	select {
	case <-d.snapshotChannel:
	case err := <-t.recordingStatus:
		t.recordingStatus <- err
	}
}

type fakeClock struct {
	clock        time.Time
	sleepChannel chan interface{}
	sem          *semaphore.Weighted
}

func NewFakeClock() *fakeClock {
	return &fakeClock{clock: time.Now(), sleepChannel: make(chan interface{}), sem: NewSleepSemaphore()}
}

func NewSleepSemaphore() *semaphore.Weighted {
	s := semaphore.NewWeighted(1000000)
	s.Acquire(context.Background(), 1000000)
	return s
}

func (c *fakeClock) Sleep(ctx context.Context, d time.Duration) error {
	n := d.Milliseconds()
	if c.sem.TryAcquire(n) {
		// Sanity check: Fake clock was advanced before this Sleep() was requested.
		panic(errors.New("Incorrect usage of testing API"))
	}

	// Signal that this goroutine is now sleeping.
	close(c.sleepChannel)

	// Wait for fake clock to advance.
	err := c.sem.Acquire(ctx, n)

	// Create new semaphore. This ensures that the semaphore is in a consistent state. This is necessary because StopRecording() cancels the context, which interrupts Acquire().
	c.sem = NewSleepSemaphore()

	// Create a new channel, so that other goroutines can wait for the next sleep.
	c.sleepChannel = make(chan interface{})

	return err
}

func (c *fakeClock) Now() time.Time {
	return c.clock
}

func (c *fakeClock) Advance(d time.Duration) {
	c.clock = c.clock.Add(d)
	c.sem.Release(d.Milliseconds())
}

func (c *fakeClock) WaitForSleep(t *Timeline) {
	select {
	case <-c.sleepChannel:
	case err := <-t.recordingStatus:
		t.recordingStatus <- err
	}
}

func TestTimeline(t *testing.T) {
	p := NewValues()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	clock := NewFakeClock()

	d1 := newDatasource()
	d2 := newDatasource()

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
	clock.WaitForSleep(tl)
	for i := 0; i < 2; i++ {
		clock.Advance(1200 * time.Millisecond)
		d1.WaitForSnapshot(tl)
		clock.WaitForSleep(tl)
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
	clock.WaitForSleep(tl)
	for i := 2; i < 5; i++ {
		clock.Advance(1400 * time.Millisecond)
		d1.WaitForSnapshot(tl)
		clock.WaitForSleep(tl)
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

	d := newDatasource()

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
	d := newDatasource()
	d.snapshotDuration = 500 * time.Millisecond
	d.clock = clock

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
	clock.WaitForSleep(tl)
	for i := 0; i < 2; i++ {
		clock.Advance(250 * time.Millisecond)
		d.WaitForSnapshot(tl)
		clock.WaitForSleep(tl)
	}

	if _, err := tl.StopRecording(); err == nil {
		t.Error("StopRecording should have failed")
	}
}

func TestTimelineNoStart(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	clock := NewFakeClock()
	d := newDatasource()

	tl, err := NewTimeline(ctx, []TimelineDatasource{d}, Interval(1*time.Second), WithClock(clock))
	if err != nil {
		t.Error("Failed to create Timeline: ", err)
	}

	if err := tl.StartRecording(ctx); err != nil {
		t.Error("Failed to start recording: ", err)
	}

	// Take 2 samples. Recording already stops after the taking the first sample.
	clock.WaitForSleep(tl)
	for i := 0; i < 2; i++ {
		clock.Advance(1100 * time.Millisecond)
		d.WaitForSnapshot(tl)
		clock.WaitForSleep(tl)
	}

	if _, err := tl.StopRecording(); err == nil {
		t.Error("Snapshot should have failed without calling Start first")
	}
}

func TestTimelineSetupFail(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	d := newDatasource()
	d.errSetup = true

	if _, err := NewTimeline(ctx, []TimelineDatasource{d}); err == nil {
		t.Error("NewTimeline should have failed because of setup failure")
	}
}

func TestTimelineStartFail(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	d := newDatasource()
	d.errStart = true

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
	d := newDatasource()
	d.errSnapshot = true

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
	clock.WaitForSleep(tl)
	for i := 0; i < 2; i++ {
		clock.Advance(1100 * time.Millisecond)
		d.WaitForSnapshot(tl)
		clock.WaitForSleep(tl)
	}

	if _, err := tl.StopRecording(); err == nil {
		t.Error("Snapshot should have failed")
	}
}
