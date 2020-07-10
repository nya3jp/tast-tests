// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package cuj has utilities for CUJ-style UI performance tests.
package cuj

import (
	"context"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/local/load"
	"chromiumos/tast/local/power"
	"chromiumos/tast/testing"
)

type metricGroup string

const (
	groupSmoothness metricGroup = "AnimationSmoothness"
	groupLatency    metricGroup = "InputLatency"
	groupOther      metricGroup = ""
)

const checkInterval = 300 * time.Millisecond

// MetricConfig is the configuration for the recorder.
type MetricConfig struct {
	// The name of the histogram to be recorded.
	histogramName string

	// The unit of the histogram, like "percent" or "ms".
	unit string

	// The direction of the histogram.
	direction perf.Direction

	// The criteria to be considered jank, used to aggregated rate of janky
	// instances. This can be empty, in that case the defualt criteria will be
	// used.
	jankCriteria []int64

	// The group of the metrics. Metrics in the same group will be aggregated
	// nto one, except for groupOther.
	group metricGroup
}

// NewSmoothnessMetricConfig creates a new MetricConfig instance for collecting
// animation smoothness data for the given histogram name. The whole data of all
// smoothness metrics will be aggregated into the "AnimationSmoothness" entry at
// the end.
func NewSmoothnessMetricConfig(histogramName string) MetricConfig {
	return MetricConfig{histogramName: histogramName, unit: "percent", direction: perf.BiggerIsBetter, jankCriteria: []int64{50, 20}, group: groupSmoothness}
}

// NewLatencyMetricConfig creates a new MetricConfig instance for collecting
// input latency data for the given histogram name. The whole data of all input
// latency metrics will be aggregated into the "InputLatency" entry at the end.
func NewLatencyMetricConfig(histogramName string) MetricConfig {
	return MetricConfig{histogramName: histogramName, unit: "ms", direction: perf.SmallerIsBetter, jankCriteria: []int64{100, 250}, group: groupLatency}
}

// NewCustomMetricConfig creates a new MetricConfig for the given histogram
// name, unit, direction, and jankCriteria. The data are reported as-is but
// not aggregated with other histograms.
func NewCustomMetricConfig(histogramName, unit string, direction perf.Direction, jankCriteria []int64) MetricConfig {
	return MetricConfig{histogramName: histogramName, unit: unit, direction: direction, jankCriteria: jankCriteria, group: groupOther}
}

type record struct {
	config     MetricConfig
	totalCount int64
	jankCounts [2]float64
	sum        int64
}

// Recorder is a utility to measure various metrics for CUJ-style tests.
type Recorder struct {
	names   []string
	records map[string]*record

	memDiff    *memoryDiffDataSource
	timeline   *perf.Timeline
	loadValues []*perf.Values
}

func getJankCounts(hist *metrics.Histogram, direction perf.Direction, criteria int64) float64 {
	var count float64
	if direction == perf.BiggerIsBetter {
		for _, bucket := range hist.Buckets {
			if bucket.Max < criteria {
				count += float64(bucket.Count)
			} else if bucket.Min <= criteria {
				// Estimate the count with assuming uniform distribution.
				count += float64(bucket.Count) * float64(criteria-bucket.Min) / float64(bucket.Max-bucket.Min)
			}
		}
	} else {
		for _, bucket := range hist.Buckets {
			if bucket.Min > criteria {
				count += float64(bucket.Count)
			} else if bucket.Max > criteria {
				count += float64(bucket.Count) * float64(bucket.Max-criteria) / float64(bucket.Max-bucket.Min)
			}
		}
	}
	return count
}

// NewRecorder creates a Recorder based on the configs. It also aggregates the
// metrics of each category (animation smoothness and input latency) and creates
// the aggregated reports.
func NewRecorder(ctx context.Context, tconn *chrome.TestConn, configs ...MetricConfig) (*Recorder, error) {
	memDiff := newMemoryDiffDataSource("Memory.Diff")
	displayInfo := NewDisplayInfoSource("Display", tconn)
	sources := []perf.TimelineDatasource{
		load.NewCPUUsageSource("CPU", false),
		load.NewMemoryUsageSource("Memory"),
		power.NewSysfsThermalMetrics(),
		memDiff,
		displayInfo,
	}
	timeline, err := perf.NewTimeline(ctx, sources, perf.Interval(checkInterval), perf.Prefix("TPS."))
	if err != nil {
		return nil, errors.Wrap(err, "failed to start perf.Timeline")
	}
	if err = timeline.Start(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to start perf.Timeline")
	}

	r := &Recorder{
		names:    make([]string, 0, len(configs)),
		records:  make(map[string]*record, len(configs)+2),
		memDiff:  memDiff,
		timeline: timeline,
	}
	for _, config := range configs {
		if config.histogramName == string(groupLatency) || config.histogramName == string(groupSmoothness) {
			return nil, errors.Errorf("invalid histogram name: %s", config.histogramName)
		}
		r.names = append(r.names, config.histogramName)
		r.records[config.histogramName] = &record{config: config}
	}
	r.records[string(groupLatency)] = &record{config: MetricConfig{
		histogramName: string(groupLatency),
		unit:          "ms",
		direction:     perf.SmallerIsBetter,
	}}
	r.records[string(groupSmoothness)] = &record{config: MetricConfig{
		histogramName: string(groupSmoothness),
		unit:          "percent",
		direction:     perf.BiggerIsBetter,
	}}

	return r, nil
}

// Run conducts the test scenario f, and collects the related metrics for the
// test scenario, and updates the internal data.
func (r *Recorder) Run(ctx context.Context, tconn *chrome.TestConn, f func() error) error {
	if err := r.memDiff.PrepareBaseline(ctx, diffWait); err != nil {
		return errors.Wrap(err, "failed to prepare baseline for memory diff calcuation")
	}
	if err := r.timeline.StartRecording(ctx); err != nil {
		return errors.Wrap(err, "failed to start recording timeline data")
	}
	defer func() {
		vs, err := r.timeline.StopRecording()
		if err != nil {
			testing.ContextLog(ctx, "Failed to stop timeline: ", err)
			return
		}
		r.loadValues = append(r.loadValues, vs)
	}()
	hists, err := metrics.Run(ctx, tconn, f, r.names...)
	if err != nil {
		return err
	}
	for _, hist := range hists {
		if hist.TotalCount() == 0 {
			continue
		}
		record := r.records[hist.Name]
		record.totalCount += hist.TotalCount()
		record.sum += hist.Sum
		jankCounts := []float64{
			getJankCounts(hist, record.config.direction, record.config.jankCriteria[0]),
			getJankCounts(hist, record.config.direction, record.config.jankCriteria[1]),
		}
		record.jankCounts[0] += jankCounts[0]
		record.jankCounts[1] += jankCounts[1]

		if totalRecord, ok := r.records[string(record.config.group)]; ok {
			totalRecord.totalCount += hist.TotalCount()
			totalRecord.sum += hist.Sum
			totalRecord.jankCounts[0] += jankCounts[0]
			totalRecord.jankCounts[1] += jankCounts[1]
		}
	}
	return nil
}

// Record creates the reporting values from the currently stored data points and
// sets the values into pv.
func (r *Recorder) Record(pv *perf.Values) error {
	pv.Merge(r.loadValues...)

	for name, record := range r.records {
		if record.totalCount == 0 {
			continue
		}
		pv.Set(perf.Metric{
			Name:      name,
			Unit:      record.config.unit,
			Variant:   "average",
			Direction: record.config.direction,
		}, float64(record.sum)/float64(record.totalCount))
		pv.Set(perf.Metric{
			Name:      name,
			Unit:      "percent",
			Variant:   "jank_rate",
			Direction: perf.SmallerIsBetter,
		}, record.jankCounts[0]/float64(record.totalCount)*100)
		pv.Set(perf.Metric{
			Name:      name,
			Unit:      "percent",
			Variant:   "very_jank_rate",
			Direction: perf.SmallerIsBetter,
		}, record.jankCounts[1]/float64(record.totalCount)*100)
	}
	return nil
}
