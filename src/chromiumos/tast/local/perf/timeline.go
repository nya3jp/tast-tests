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

// TimelineDatasource contains the only mandatory method for a datasource,
// Snapshot.
type TimelineDatasource interface {
	Setup(context.Context) error
	Start(context.Context) error
	Snapshot(context.Context, *Values) error
}

// timestampSource is the only default TimelineDatasource. Snapshot records the
// number of seconds from the beginning of the test.
type timestampSource struct {
	metric Metric
	begin  time.Time
}

// Setup does nothing, but is needed to be a TimelineDatasource.
func (t *timestampSource) Setup(_ context.Context) error {
	return nil
}

// Start records the start time of the test.
func (t *timestampSource) Start(_ context.Context) error {
	t.begin = time.Now()
	return nil
}

// Snapshot appends the current runtime of the test.
func (t *timestampSource) Snapshot(_ context.Context, v *Values) error {
	v.Append(t.metric, time.Now().Sub(t.begin).Seconds())
	return nil
}

// TimelineBuilder helps create TimelineDatasources by consolidating setup code
// into one call so test code only has to check one error.
type TimelineBuilder struct {
	sources []TimelineDatasource
}

// Timeline collects performance metrics periodically on a common timeline.
type Timeline struct {
	sources []TimelineDatasource
}

// NewTimeline creates a Timeline from a slice of TimelineDatasource, calling
// all the Setup methods.
func NewTimeline(ctx context.Context, sources ...TimelineDatasource) (*Timeline, error) {
	for _, s := range sources {
		if err := s.Setup(ctx); err != nil {
			return nil, errors.Wrap(err, "failed to setup TimelineDatasource")
		}
	}
	return &Timeline{sources}, nil
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
