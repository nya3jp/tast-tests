// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package perf

import (
	"context"
	"time"
)

// Timeline datasources provide periodic performance metrics collected a the
// same time during a test.
// To provide metrics, a datasource only needs to provide a Snapshot method,
// all other lifecycle methods are optional. Lifecycle methods are called in
// the following order:
// Setup    - do any potentially expensive metric initialization. Called from
//            TimelineBuilder.Build
// Start    - capture any initial state needed to start metrics collection.
//            Called from Timeline.Start.
// Snapshot - Log one data point. Called from Timeline.Snapshot.

// TimelineDatasource contains the only mandatory method for a datasource,
// Snapshot.
type TimelineDatasource interface {
	Snapshot(context.Context, *Values) error
}

// timelineDatasourceSetup is used to detect if a datasource needs a Setup call
// when TimelineBuilder builds a Timeline.
type timelineDatasourceSetup interface {
	Setup(context.Context) error
}

// timelineDatasourceStart is used to detect if a datasource needs a Start call
// when Timeline.Start is called.
type timelineDatasourceStart interface {
	Start(context.Context) error
}

// timestampSource is the only default TimelineDatasource. Snapshot records the
// number of seconds from the beginning of the test.
type timestampSource struct {
	metric Metric
	begin  time.Time
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

// NewTimelineBuilder creates a builder ready to be filled with timeline
// datasources. A timestampSource is added to record time with every Sample.
func NewTimelineBuilder() *TimelineBuilder {
	return &TimelineBuilder{sources: []TimelineDatasource{
		&timestampSource{
			metric: Metric{Name: "t", Unit: "s", Multiple: true},
		},
	}}
}

// Append adds TimelineDatasources to the list of sources to be built.
func (t *TimelineBuilder) Append(sources ...TimelineDatasource) {
	t.sources = append(t.sources, sources...)
}

// Build creates a Timeline from all the appended datasources, calling their
// Setup method if it exists.
func (t *TimelineBuilder) Build(ctx context.Context) (*Timeline, error) {
	sources := t.sources
	t.sources = []TimelineDatasource{}
	for _, s := range sources {
		setup, hasSetup := s.(timelineDatasourceSetup)
		if !hasSetup {
			continue
		}
		if err := setup.Setup(ctx); err != nil {
			return nil, err
		}
	}
	return &Timeline{sources}, nil
}

// Timeline collects performance metrics periodically on a common timeline.
type Timeline struct {
	sources []TimelineDatasource
}

// Start starts metric collection on all datasources that need it.
func (t *Timeline) Start(ctx context.Context) error {
	for _, s := range t.sources {
		start, hasStart := s.(timelineDatasourceStart)
		if !hasStart {
			continue
		}
		if err := start.Start(ctx); err != nil {
			return err
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
