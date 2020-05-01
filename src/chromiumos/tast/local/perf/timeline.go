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
// DEPRECATED: please use common/perf package instead.
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
// DEPRECATED: please use common/perf package instead.
type Timeline struct {
	sources         []TimelineDatasource
	interval        time.Duration
	cancelRecording context.CancelFunc
	recordingValues *Values
	recordingStatus chan error
}

// NewTimelineOptions holds all optional parameters of NewTimeline.
type NewTimelineOptions struct {
	// A prefix that is added to all metric names.
	Prefix string
	// The time duration between two subsequent metric snapshots. Default value is 10 seconds.
	Interval time.Duration
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

// NewTimeline creates a Timeline from a slice of TimelineDatasources. Metric names may be prefixed and callers can specify the time interval between two subsequent snapshots. This method calls the Setup method of each data source.
// DEPRECATED: please use common/perf package instead.
func NewTimeline(ctx context.Context, sources []TimelineDatasource, setters ...NewTimelineOption) (*Timeline, error) {
	args := NewTimelineOptions{Interval: 10 * time.Second}
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

// Snapshot takes a snapshot of all metrics.
// DEPRECATED: This will be removed as soon as tests in tast-tests-private are updated to the new API.
func (t *Timeline) Snapshot(ctx context.Context, v *Values) error {
	return t.snapshot(ctx, v)
}

// StartRecording starts capturing metrics in a goroutine. The sampling interval is specified as a parameter of NewTimeline. StartRecording may not be called twice, unless StopRecording is called in-between.
func (t *Timeline) StartRecording(ctx context.Context) error {
	if t.recordingStatus != nil {
		return errors.New("already recording")
	}

	ctx, t.cancelRecording = context.WithCancel(ctx)
	t.recordingValues = NewValues()
	t.recordingStatus = make(chan error, 2)

	go func() {
		for {
			if err := testing.Sleep(ctx, t.interval); err != nil {
				t.recordingStatus <- nil
				return
			}

			if err := t.snapshot(ctx, t.recordingValues); err != nil {
				t.recordingStatus <- err
				return
			}
		}
	}()

	return nil
}

// StopRecording stops capturing metrics and returns the captured metrics.
func (t *Timeline) StopRecording() (*Values, error) {
	if t.recordingStatus == nil {
		return nil, errors.New("not recording yet")
	}

	t.cancelRecording()
	err := <-t.recordingStatus
	if err != nil {
		return nil, err
	}

	result := t.recordingValues
	t.recordingValues = nil
	t.recordingStatus = nil

	return result, nil
}
