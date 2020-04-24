// Copyright 2020 The Chromium OS Authors. All rights reserved.
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

// TimelineDatasource is an interface that is implemented to add a source of
// metrics to a Timeline.
type TimelineDatasource interface {
	Setup(ctx context.Context, prefix string) error
	Start(ctx context.Context) error
	Snapshot(ctx context.Context, values *Values) error
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

// Timeline collects performance metrics periodically on a common timeline.
type Timeline struct {
	sources  []TimelineDatasource
	interval time.Duration
}

// newTimelineOptions holds all optional parameters of NewTimeline.
type newTimelineOptions struct {
	// A prefix that is added to all metric names.
	Prefix string
	// The time duration between two subsequent metric snapshots. Default value is 10 seconds.
	Interval time.Duration
}

// NewTimelineOption sets an optional parameter of NewTimeline.
type newTimelineOption func(*newTimelineOptions)

// Interval sets the interval between two subsequent metric snapshots.
func Interval(interval time.Duration) newTimelineOption {
	return func(args *newTimelineOptions) {
		args.Interval = interval
	}
}

// Prefix sets prepends all metric names with a given string.
func Prefix(prefix string) newTimelineOption {
	return func(args *newTimelineOptions) {
		args.Prefix = prefix
	}
}

// NewTimeline creates a Timeline from a slice of TimelineDatasources. Metric names may be prefixed and callers can specify the time interval between two subsequent snapshots. This method calls the Setup method of each data source.
func NewTimeline(ctx context.Context, sources []TimelineDatasource, setters ...newTimelineOption) (*Timeline, error) {
	args := newTimelineOptions{Interval: 10 * time.Second}
	for _, setter := range setters {
		setter(&args)
	}

	ss := append(sources, &timestampSource{})
	for _, s := range ss {
		if err := s.Setup(ctx, args.Prefix); err != nil {
			return nil, errors.Wrap(err, "failed to setup TimelineDatasource")
		}
	}
	return &Timeline{sources: ss, interval: args.Interval}, nil
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

// CaptureWhile captures metrics as long as condition returns true. The capture interval can be set as a parameter of NewTimeline. This method blocks until condition returns false. condition is guaranteed to be executed once per interval.
func (t *Timeline) CaptureWhile(ctx context.Context, v *Values, condition func(int) (bool, error)) error {
	for i := 0; ; i++ {
		shouldContinue, err := condition(i)
		if err != nil {
			return err
		}

		if !shouldContinue {
			return nil
		}

		if err := testing.Sleep(ctx, t.interval); err != nil {
			return err
		}

		if err := t.snapshot(ctx, v); err != nil {
			return err
		}
	}
}

// WaitForChannel returns a condition function to be used with CaptureWhile. This function returns true until a value is read from c. A nil value indicates success. An error value indicates that something went wrong.
func WaitForChannel(c chan error) func(int) (bool, error) {
	return func(_ int) (bool, error) {
		select {
		case err := <-c:
			return false, err
		default:
			return true, nil
		}
	}
}

// WaitForCounter returns a condition function to be used with CaptureWhile. This function returns true for the first numIterations many times it called and false afterwards.
func WaitForCounter(numIterations int) func(int) (bool, error) {
	return func(i int) (bool, error) {
		return i < numIterations, nil
	}
}
