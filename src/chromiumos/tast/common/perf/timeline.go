// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package perf

import (
	"context"
	"time"

	"chromiumos/tast/local/perf"
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
type TimelineDatasource = perf.TimelineDatasource

// Timeline collects performance metrics periodically on a common timeline.
type Timeline = perf.Timeline

// NewTimelineOptions holds all optional parameters of NewTimeline.
type NewTimelineOptions = perf.NewTimelineOptions

// NewTimelineOption sets an optional parameter of NewTimeline.
type NewTimelineOption = perf.NewTimelineOption

// Interval sets the interval between two subsequent metric snapshots.
func Interval(interval time.Duration) NewTimelineOption {
	return perf.Interval(interval)
}

// Prefix sets prepends all metric names with a given string.
func Prefix(prefix string) NewTimelineOption {
	return perf.Prefix(prefix)
}

// NewTimeline creates a Timeline from a slice of TimelineDatasources. Metric names may be prefixed and callers can specify the time interval between two subsequent snapshots. This method calls the Setup method of each data source.
func NewTimeline(ctx context.Context, sources []TimelineDatasource, setters ...NewTimelineOption) (*Timeline, error) {
	return perf.NewTimeline(ctx, sources, setters...)
}
