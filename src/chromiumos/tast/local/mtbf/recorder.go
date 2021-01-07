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
	"chromiumos/tast/testing"
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

// NewRecorder creates a Recorder with CPU Memory and Thermal timeline
// than start recording it.
func NewRecorder(ctx context.Context) (*Recorder, error) {
	sources := []perf.TimelineDatasource{
		perfSrc.NewCPUUsageSource("CPU"),
		perfSrc.NewThermalDataSource(ctx),
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

// Record stop the timeline recording and creates the reporting values
// from the currently stored data points than
// save the results "results-chart.json" to the outDir.
func (r *Recorder) Record(ctx context.Context, outDir string) error {
	// We want to conduct all of Stop tasks even when some of them fails.
	// Return an error when one of them has failed.
	var stopErr error

	vs, err := r.timeline.StopRecording(ctx)
	if err != nil {
		testing.ContextLog(ctx, "Failed to stop timeline: ", err)
		if stopErr == nil {
			stopErr = errors.Wrap(err, "failed to stop timeline")
		}
	}
	if stopErr != nil {
		return stopErr
	}

	pv := perf.NewValues()
	timeElapsed := time.Since(r.startTime)
	pv.Set(perf.Metric{
		Name:      "Recorder.ElapsedTime",
		Unit:      "s",
		Direction: perf.SmallerIsBetter,
	}, float64(timeElapsed.Seconds()))
	pv.Merge(vs)
	if err := pv.Save(outDir); err != nil {
		return err
	}

	return nil
}
