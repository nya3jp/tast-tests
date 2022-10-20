// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package perf

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// Timeline datasources provide periodic performance metrics collected at the
// same time during a test.
// Lifecycle methods are called in the following order:
// Setup    - do any potentially expensive metric initialization. Called from
//            TimelineBuilder.Build
// Start    - capture any initial state needed to start metrics collection.
//            Called from Timeline.Start.
// Snapshot - Log one data point. Called from Timeline.Snapshot.
// Stop     - Log one last data point. Called from Timeline.StopRecording if
//            there haven't been any errors during snapshotting.

// TimelineDatasource is an interface that is implemented to add a source of
// metrics to a Timeline.
type TimelineDatasource interface {
	Setup(ctx context.Context, prefix string) error
	Start(ctx context.Context) error
	Snapshot(ctx context.Context, values *Values) error
	Stop(ctx context.Context, values *Values) error
}

// timestampSource is the only default TimelineDatasource. Snapshot records the
// number of seconds from the beginning of the test.
type timestampSource struct {
	begin   time.Time
	started bool
	metric  Metric
}

// Setup created the metric used for recording the timestamps with the correct
// prefix.
func (t *timestampSource) Setup(_ context.Context, prefix string) error {
	t.metric = Metric{
		Name:     prefix + "t",
		Unit:     "s",
		Variant:  DefaultVariantName,
		Multiple: true,
	}
	return nil
}

// Start records the start time of the test.
func (t *timestampSource) Start(_ context.Context) error {
	t.started = true
	t.begin = time.Now()
	return nil
}

// Snapshot appends the current runtime of the test.
func (t *timestampSource) Snapshot(_ context.Context, v *Values) error {
	if !t.started {
		return errors.New("failed to snapshot Timeline, Start wasn't called")
	}
	v.Append(t.metric, time.Now().Sub(t.begin).Seconds())
	return nil
}

// Stop does nothing.
func (t *timestampSource) Stop(_ context.Context, v *Values) error {
	return nil
}

// Clock implementations are used for waiting a certain time duration. In unit tests, fake Clocks should be used to avoid race conditions.
type Clock interface {
	Sleep(ctx context.Context, d time.Duration) error
	Now() time.Time
}

// defaultClock uses the default Sleep()/Now() implementations.
type defaultClock struct{}

// Sleep waits for a certain time duration.
func (t defaultClock) Sleep(ctx context.Context, d time.Duration) error {
	return testing.Sleep(ctx, d)
}

func (t defaultClock) Now() time.Time {
	return time.Now()
}

// Timeline collects performance metrics periodically on a common timeline.
type Timeline struct {
	sources           []TimelineDatasource
	interval          time.Duration
	cancelRecording   context.CancelFunc
	recordingValues   *Values
	recordingStatus   chan error
	clock             Clock
	enableGracePeriod bool
	snapshotsSkipped  int
	prefix            string
}

// NewTimelineOptions holds all optional parameters of NewTimeline.
type NewTimelineOptions struct {
	// A prefix that is added to all metric names.
	Prefix string
	// The time duration between two subsequent metric snapshots. Default value is 10 seconds.
	Interval time.Duration
	// A different Clock implementation is used in Timeline unit tests to avoid sleeping in test code.
	Clock Clock
	// Whether or not we allow for a grace period when taking snapshots.
	EnableGracePeriod bool
}

// NewTimelineOption sets an optional parameter of NewTimeline.
type NewTimelineOption func(*NewTimelineOptions)

// Interval sets the interval between two subsequent metric snapshots.
func Interval(interval time.Duration) NewTimelineOption {
	return func(args *NewTimelineOptions) {
		args.Interval = interval
	}
}

// Prefix sets prepends all metric names with a given string.
func Prefix(prefix string) NewTimelineOption {
	return func(args *NewTimelineOptions) {
		args.Prefix = prefix
	}
}

// WithClock sets a Clock implementation.
func WithClock(clock Clock) NewTimelineOption {
	return func(args *NewTimelineOptions) {
		args.Clock = clock
	}
}

// EnableGracePeriod sets the timeline to allow for a 1-interval grace
// period to take a snapshot.
func EnableGracePeriod() NewTimelineOption {
	return func(args *NewTimelineOptions) {
		args.EnableGracePeriod = true
	}
}

// NewTimeline creates a Timeline from a slice of TimelineDatasources. Metric names may be prefixed and callers can specify the time interval between two subsequent snapshots. This method calls the Setup method of each data source.
func NewTimeline(ctx context.Context, sources []TimelineDatasource, setters ...NewTimelineOption) (*Timeline, error) {
	args := NewTimelineOptions{Interval: 10 * time.Second, Clock: &defaultClock{}}
	for _, setter := range setters {
		setter(&args)
	}

	ss := append(sources, &timestampSource{})
	for _, s := range ss {
		if err := s.Setup(ctx, args.Prefix); err != nil {
			return nil, errors.Wrap(err, "failed to setup TimelineDatasource")
		}
	}
	return &Timeline{
		sources:           ss,
		interval:          args.Interval,
		clock:             args.Clock,
		enableGracePeriod: args.EnableGracePeriod,
		prefix:            args.Prefix,
	}, nil
}

// Start starts metric collection on all datasources.
func (t *Timeline) Start(ctx context.Context) error {
	for _, s := range t.sources {
		if err := s.Start(ctx); err != nil {
			return errors.Wrap(err, "failed to start TimelineDatasource")
		}
	}
	return nil
}

// snapshot takes a snapshot of all metrics.
func (t *Timeline) snapshot(ctx context.Context, v *Values) error {
	for _, s := range t.sources {
		if err := s.Snapshot(ctx, v); err != nil {
			return err
		}
	}
	return nil
}

// stop gives the metrics last chance to report values before test finishes.
func (t *Timeline) stop(ctx context.Context, v *Values) error {
	for _, s := range t.sources {
		if err := s.Stop(ctx, v); err != nil {
			return err
		}
	}
	return nil
}

// StartRecording starts capturing metrics in a goroutine. The sampling
// interval is specified as a parameter of NewTimeline. StartRecording
// may not be called twice, unless StopRecording is called in-between.
func (t *Timeline) StartRecording(ctx context.Context) error {
	if t.recordingStatus != nil {
		return errors.New("already recording")
	}

	ctx, t.cancelRecording = context.WithCancel(ctx)
	t.recordingValues = NewValues()
	t.recordingStatus = make(chan error, 1)

	go func() {
		for nextTime := t.clock.Now().Add(t.interval); ; nextTime = nextTime.Add(t.interval) {
			sleepTime := nextTime.Sub(t.clock.Now())
			if sleepTime < 0 {
				if !t.enableGracePeriod {
					t.recordingStatus <- errors.Errorf("failed to take snapshot; trying to snapshot every %v, but taking the last snapshot already took more time", t.interval)
					return
				}

				// If the snapshot took more time than the timeline
				// interval, give a 1-interval grace period to complete the
				// snapshot. If a snapshot takes longer than 2 intervals,
				// fail the timeline collection.
				sleepTime += t.interval
				if sleepTime < 0 {
					t.recordingStatus <- errors.Errorf("failed to take snapshot; trying to snapshot every %v with 1-interval grace period, but taking the last snapshot already took more than 2 intervals", t.interval)
					return
				}

				testing.ContextLogf(ctx, "Skipping snapshot because the last snapshot took more than the %v interval, but completed within the 1-interval grace period", t.interval)
				nextTime = nextTime.Add(t.interval)
				t.snapshotsSkipped++
			}

			if err := t.clock.Sleep(ctx, sleepTime); err != nil {
				t.recordingStatus <- nil
				return
			}

			val := NewValues()
			if err := t.snapshot(ctx, val); err != nil {
				if ctx.Err() != nil {
					t.recordingStatus <- nil
				} else {
					// Actual error during snapshotting.
					t.recordingStatus <- err
				}
				return
			}
			t.recordingValues.Merge(val)
		}
	}()

	return nil
}

// StopRecording stops capturing metrics and returns the captured metrics.
func (t *Timeline) StopRecording(ctx context.Context) (*Values, error) {
	if t.recordingStatus == nil {
		return nil, errors.New("not recording yet")
	}

	t.cancelRecording()

	var err error
	select {
	case err = <-t.recordingStatus:
	case <-time.After(2 * time.Minute):
		panic("StopRecording timed out")
	}

	if err != nil {
		return nil, err
	}

	val := NewValues()
	if err := t.stop(ctx, val); err != nil {
		return nil, err
	}

	if t.enableGracePeriod {
		prefix := "DefaultTimeline."
		if len(t.prefix) > 0 {
			prefix = t.prefix
		}

		val.Set(Metric{
			Name:      prefix + "snapshotsSkipped",
			Unit:      "count",
			Direction: SmallerIsBetter,
		}, float64(t.snapshotsSkipped))
	}
	t.recordingValues.Merge(val)

	result := t.recordingValues
	t.recordingValues = nil
	t.recordingStatus = nil

	return result, nil
}
