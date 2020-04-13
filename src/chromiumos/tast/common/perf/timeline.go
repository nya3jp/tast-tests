// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package perf

import (
	"context"

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

// NewTimeline creates a Timeline from a slice of TimelineDatasource, calling
// all the Setup methods.
func NewTimeline(ctx context.Context, sources ...TimelineDatasource) (*Timeline, error) {
	return perf.NewTimeline(ctx, sources...)
}

// NewTimelineWithPrefix creates a Timeline from a slice of TimelineDatasources,
// all created metrics will be prefixed with the passed prefix.
func NewTimelineWithPrefix(ctx context.Context, prefix string, sources ...TimelineDatasource) (*Timeline, error) {
	return perf.NewTimelineWithPrefix(ctx, prefix, sources...)
}
