// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package perf

import (
	"context"
	"time"

	"chromiumos/tast/errors"
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
	Setup(context.Context, string) error
	Start(context.Context) error
	Snapshot(context.Context, *Values) error
}

// timestampSource is the only default TimelineDatasource. Snapshot records the
// number of seconds from the beginning of the test.
type timestampSource struct {
	begin   time.Time
	started bool
	metric  Metric
}

// Setup does nothing, but is needed to be a TimelineDatasource.
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
	sources []TimelineDatasource
}

// NewTimeline creates a Timeline from a slice of TimelineDatasource, calling
// all the Setup methods.
func NewTimeline(ctx context.Context, sources ...TimelineDatasource) (*Timeline, error) {
	return NewTimelineWithPrefix(ctx, "", sources...)
}

// NewTimelineWithPrefix creates a Timeline from a slice of TimelineDatasources,
// all created metrics will be prefixed with the passed prefix.
func NewTimelineWithPrefix(ctx context.Context, prefix string, sources ...TimelineDatasource) (*Timeline, error) {
	ss := append(sources, &timestampSource{})
	for _, s := range ss {
		if err := s.Setup(ctx, prefix); err != nil {
			return nil, errors.Wrap(err, "failed to setup TimelineDatasource")
		}
	}
	return &Timeline{sources: ss}, nil
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

// Snapshot takes a snapshot of all metrics.
func (t *Timeline) Snapshot(ctx context.Context, v *Values) error {
	for _, s := range t.sources {
		if err := s.Snapshot(ctx, v); err != nil {
			return err
		}
	}
	return nil
}
