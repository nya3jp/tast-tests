// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package perf

import (
	"context"
	"math"
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
	// A value is added to this channel each time a snapshot is taken. Tests should take no more snapshots than the buffer size of this channel.
	ds.snapshotChannel = make(chan int, 1000)
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
	if d.clock != nil {
		// Simulate a slow snapshot. The main goroutine does not advance the clock until this function returns.
		d.clock.Advance(d.snapshotDuration)
	}

	if d.errSnapshot {
		return errSnapshot
	}

	d.snapshotCount++

	select {
	case d.snapshotChannel <- d.snapshotCount - 1:
	default:
	}

	return nil
}

// WaitForSnapshot waits until a snapshot is taken or until the snapshotting goroutine returns.
func (d *testTimelineDatasource) WaitForSnapshot() {
	select {
	case <-d.snapshotChannel:
	case <-time.After(5 * time.Second):
		panic("WaitForSnapshot timed out")
	}
}

func (t *Timeline) WaitForSnapshottingDone() {
	select {
	case err := <-t.recordingStatus:
		t.recordingStatus <- err
	case <-time.After(5 * time.Second):
		panic("WaitForSnapshottingDone timed out")
	}
}

// fakeClock maintains a fake clock for simulating Sleep() statements.
type fakeClock struct {
	// clock is the current time of the fake clock.
	clock time.Time
	// sleepChannel can be used by other goroutines to check if a goroutine a sleeping using this clock. When a goroutine starts sleeping, this channel is closed.
	sleepChannel chan struct{}
	// sem is semaphore whose value represents the amount of Advance()-ed fake time in milliseconds. A sleeping goroutine will wait for fake time to advance by Acquire()-ing from this semaphore.
	sem *semaphore.Weighted
}

// NewFakeClock returns a new fake clock for Sleep() statements in unit tests.
func NewFakeClock() *fakeClock {
	return &fakeClock{clock: time.Unix(0, 0), sleepChannel: make(chan struct{})}
}

// Advance increases the fake time and may wake up Sleep()-ing goroutines that are blocked on the semaphore.
func (c *fakeClock) Advance(d time.Duration) {
	c.clock = c.clock.Add(d)
	c.sem.Release(d.Milliseconds())
}

// Now returns the fake time.
func (c *fakeClock) Now() time.Time {
	return c.clock
}

// Sleep sleeps until a fake time of d has passed. No more than one goroutine may call this, otherwise this function panics. Sleep returns with an error if the context is canceled or done.
func (c *fakeClock) Sleep(ctx context.Context, d time.Duration) error {
	// Create a semaphore with no remaining capacity.
	sem := semaphore.NewWeighted(math.MaxInt64)
	sem.Acquire(context.Background(), math.MaxInt64)
	c.sem = sem

	// Signal that this goroutine is now sleeping.
	close(c.sleepChannel)

	// Wait for the fake clock to advance.
	err := c.sem.Acquire(ctx, d.Milliseconds())

	// Create a new channel, so that other goroutines can wait for the next sleep.
	c.sleepChannel = make(chan struct{})

	return err
}

// WaitForSleep waits until a goroutine starts fake-sleeping or until the snapshotting goroutine returns.
func (c *fakeClock) WaitForSleep() {
	select {
	case <-c.sleepChannel:
	case <-time.After(5 * time.Second):
		panic("WaitForSleep timed out")
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
	clock.WaitForSleep()
	for i := 0; i < 2; i++ {
		clock.Advance(1050 * time.Millisecond)
		d1.WaitForSnapshot()
		clock.WaitForSleep()
	}

	if v, err := tl.StopRecording(); err != nil {
		t.Error("Error while recording: ", err)
	} else {
		p.Merge(v)
	}

	if d1.snapshotCount != 2 || d2.snapshotCount != 2 {
		t.Errorf("Wrong number of snapshots collected: got (%d, %d), expected (2, 2)", d1.snapshotCount, d2.snapshotCount)
	}

	// Second round of recording.
	if err := tl.StartRecording(ctx); err != nil {
		t.Error("Failed to start recording: ", err)
	}

	// Take 3 more samples.
	clock.WaitForSleep()
	for i := 2; i < 5; i++ {
		clock.Advance(1070 * time.Millisecond)
		d1.WaitForSnapshot()
		clock.WaitForSleep()
	}

	if v, err := tl.StopRecording(); err != nil {
		t.Error("Error while recording: ", err)
	} else {
		p.Merge(v)
	}

	if d1.snapshotCount != 5 || d2.snapshotCount != 5 {
		t.Errorf("Wrong number of snapshots collected: got (%d, %d), expected (5, 5)", d1.snapshotCount, d2.snapshotCount)
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

	// Try to take 1 sample.
	clock.WaitForSleep()
	clock.Advance(210 * time.Millisecond)
	tl.WaitForSnapshottingDone()

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

	// Try to take 1 sample.
	clock.WaitForSleep()
	clock.Advance(1100 * time.Millisecond)
	tl.WaitForSnapshottingDone()

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

	tl, err := NewTimeline(ctx, []TimelineDatasource{d}, Interval(1*time.Second), WithClock(clock))
	if err != nil {
		t.Error("Failed to create Timeline: ", err)
	}

	if err := tl.Start(ctx); err != nil {
		t.Error("Failed to start timeline: ", err)
	}

	if err := tl.StartRecording(ctx); err != nil {
		t.Error("Failed to start recording: ", err)
	}

	// Try to take 1 sample.
	clock.WaitForSleep()
	clock.Advance(1100 * time.Millisecond)
	tl.WaitForSnapshottingDone()

	if _, err := tl.StopRecording(); err == nil {
		t.Error("Snapshot should have failed")
	}
}
