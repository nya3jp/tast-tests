// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package mtbf implements a library used for MTBF testing.
package mtbf

import (
	"context"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	perfSrc "chromiumos/tast/local/perf"
)

const (
	metricPrefix  = "TPS."
	checkInterval = time.Second
)

// Recorder is a utility to measure various metrics for CUJ-style tests.
type Recorder struct {
	startTime time.Time
	timeline  *perf.Timeline
}

// NewRecorder creates a Recorder with CPU, Memory and Thermal timeline
// then starts recording it.
func NewRecorder(ctx context.Context) (*Recorder, error) {
	sources := []perf.TimelineDatasource{
		perfSrc.NewCPUUsageSource("CPU"),
		perfSrc.NewThermalDataSource(),
		perfSrc.NewMemoryDataSource("RAM.Absolute", "RAM.Diff.Absolute", "RAM"),
	}
	timeline, err := perf.NewTimeline(ctx, sources, perf.Interval(checkInterval), perf.Prefix(metricPrefix))
	if err != nil {
		return nil, errors.Wrap(err, "failed to start perf.Timeline")
	}
	if err = timeline.Start(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to start perf.Timeline")
	}

	r := &Recorder{
		timeline: timeline,
	}
	if err := r.timeline.StartRecording(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to start recording timeline data")
	}
	r.startTime = time.Now()
	return r, nil
}

// Record stops the timeline recording and creates the reporting values
// from the currently stored data points then
// saves the results "results-chart.json" to the outDir.
func (r *Recorder) Record(ctx context.Context, outDir string) error {
	vs, err := r.timeline.StopRecording(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to stop timeline")
	}

	pv := perf.NewValues()
	timeElapsed := time.Since(r.startTime)
	pv.Set(perf.Metric{
		Name:      "Recorder.ElapsedTime",
		Unit:      "s",
		Direction: perf.SmallerIsBetter,
	}, float64(timeElapsed.Seconds()))
	pv.Merge(vs)

	return pv.Save(outDir)
}
