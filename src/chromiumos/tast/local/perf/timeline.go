// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package perf

import "time"

// TimelineDatasource defines the lifecycle of a source of metrics that can be
// periodically sampled.
// Setup, called by TimelineBuilder.Build before the test begins, is for metric
// initialization code.
// Start, called by Timeline.Start when the test begins, is for resetting
// metrics without recording anything.
// Snapshot, called by Timeline.Snapshot, is used to write performance metrics.
type TimelineDatasource interface {
	Setup() error
	Start() error
	Snapshot(*Values) error
}

// timestampSource is the only default TimelineDatasource. Snapshot records the
// number of seconds from the beginning of the test.
type timestampSource struct {
	metric Metric
	begin  time.Time
}

// Setup conforms to TimelineDatasource, but does nothing.
func (t *timestampSource) Setup() error {
	return nil
}

// Start records the start time of the test.
func (t *timestampSource) Start() error {
	t.begin = time.Now()
	return nil
}

// Snapshot appends the current runtime of the test.
func (t *timestampSource) Snapshot(v *Values) error {
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

// Append adds a TimelineDatasource to the list of sources to be built.
func (t *TimelineBuilder) Append(source TimelineDatasource) {
	t.sources = append(t.sources, source)
}

// Build creates a Timeline from all the appended datasources.
func (t *TimelineBuilder) Build() (*Timeline, error) {
	sources := t.sources
	t.sources = []TimelineDatasource{}
	for _, s := range sources {
		if err := s.Setup(); err != nil {
			return nil, err
		}
	}
	return &Timeline{sources}, nil
}

// Timeline collects performance metrics periodically on a common timeline.
type Timeline struct {
	sources []TimelineDatasource
}

// Start starts metric collection on all datasources.
func (t *Timeline) Start() error {
	for _, s := range t.sources {
		if err := s.Start(); err != nil {
			return err
		}
	}
	return nil
}

// Snapshot takes a snapshot of all metrics.
func (t *Timeline) Snapshot(v *Values) error {
	for _, s := range t.sources {
		if err := s.Snapshot(v); err != nil {
			return err
		}
	}
	return nil
}
