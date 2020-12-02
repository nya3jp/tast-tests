// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package cuj has utilities for CUJ-style UI performance tests.
package cuj

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"path"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/testing"
)

type metricGroup string

const (
	groupSmoothness metricGroup = "AnimationSmoothness"
	groupLatency    metricGroup = "InputLatency"
	groupOther      metricGroup = ""
)

const metricPrefix = "TPS."

const checkInterval = time.Second

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
	tconn *chrome.TestConn

	names      []string
	records    map[string]*record
	histograms map[string]*metrics.Histogram

	timeline           *perf.Timeline
	gpuDataSource      *gpuDataSource
	frameDataTracker   *FrameDataTracker
	zramInfoTracker    *ZramInfoTracker
	batteryInfoTracker *BatteryInfoTracker
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
	gpuDS := newGPUDataSource(tconn)
	sources := []perf.TimelineDatasource{
		NewCPUUsageSource("CPU"),
		newThermalDataSource(ctx),
		gpuDS,
		newMemoryDataSource("RAM.Absolute", "RAM.Diff.Absolute", "RAM"),
	}
	timeline, err := perf.NewTimeline(ctx, sources, perf.Interval(checkInterval), perf.Prefix(metricPrefix))
	if err != nil {
		return nil, errors.Wrap(err, "failed to start perf.Timeline")
	}
	if err = timeline.Start(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to start perf.Timeline")
	}

	frameDataTracker, err := NewFrameDataTracker(metricPrefix)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create FrameDataTracker")
	}

	zramInfoTracker, err := NewZramInfoTracker(metricPrefix)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create ZramInfoTracker")
	}

	batteryInfoTracker, err := NewBatteryInfoTracker(ctx, metricPrefix)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create BatteryInfoTracker")
	}

	r := &Recorder{
		tconn:              tconn,
		names:              make([]string, 0, len(configs)),
		records:            make(map[string]*record, len(configs)+2),
		histograms:         make(map[string]*metrics.Histogram),
		timeline:           timeline,
		gpuDataSource:      gpuDS,
		frameDataTracker:   frameDataTracker,
		zramInfoTracker:    zramInfoTracker,
		batteryInfoTracker: batteryInfoTracker,
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

	success := false

	if err := r.frameDataTracker.Start(ctx, tconn); err != nil {
		return nil, errors.Wrap(err, "failed to start FrameDataTracker")
	}
	defer func() {
		if success {
			return
		}
		if err := r.frameDataTracker.Stop(ctx, tconn); err != nil {
			testing.ContextLog(ctx, "Failed to stop frame data tracker: ", err)
		}
	}()

	if err := r.zramInfoTracker.Start(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to start ZramInfoTracker")
	}

	if err := r.batteryInfoTracker.Start(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to start BatteryInfoTracker")
	}

	if err := r.timeline.StartRecording(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to start recording timeline data")
	}
	success = true

	return r, nil
}

// Close clears states for all trackers.
func (r *Recorder) Close(ctx context.Context) error {
	r.gpuDataSource.Stop()
	return r.frameDataTracker.Close(ctx, r.tconn)
}

// Run conducts the test scenario f, and collects the related metrics for the
// test scenario, and updates the internal data.
func (r *Recorder) Run(ctx context.Context, f func(ctx context.Context) error) error {
	hists, err := metrics.Run(ctx, r.tconn, f, r.names...)
	if err != nil {
		return err
	}

	for _, hist := range hists {
		if hist.TotalCount() == 0 {
			continue
		}

		// Concat to exsting histograms.
		if _, ok := r.histograms[hist.Name]; !ok {
			r.histograms[hist.Name] = hist
		} else {
			if err := r.histograms[hist.Name].Concat(hist); err != nil {
				return err
			}
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
func (r *Recorder) Record(ctx context.Context, pv *perf.Values) error {
	// We want to conduct all of Stop tasks even when some of them fails.  Return
	// an error when one of them has failed.
	var stopErr error
	if err := r.frameDataTracker.Stop(ctx, r.tconn); err != nil {
		testing.ContextLog(ctx, "Failed to stop FrameDataTracker: ", err)
		stopErr = errors.Wrap(err, "failed to stop FrameDataTracker")
	}

	if err := r.zramInfoTracker.Stop(ctx); err != nil {
		testing.ContextLog(ctx, "Failed to stop ZramInfoTracker: ", err)
		if stopErr == nil {
			stopErr = errors.Wrap(err, "failed to stop ZramInfoTracker")
		}
	}

	if err := r.batteryInfoTracker.Stop(ctx); err != nil {
		testing.ContextLog(ctx, "Failed to stop BatteryInfoTracker: ", err)
		if stopErr == nil {
			stopErr = errors.Wrap(err, "failed to stop BatteryInfoTracker")
		}
	}

	vs, err := r.timeline.StopRecording()
	if err != nil {
		testing.ContextLog(ctx, "Failed to stop timeline: ", err)
		if stopErr == nil {
			stopErr = errors.Wrap(err, "failed to stop timeline")
		}
	}
	if stopErr != nil {
		return stopErr
	}
	pv.Merge(vs)

	displayInfo, err := NewDisplayInfo(ctx, r.tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get display info")
	}

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

	displayInfo.Record(pv)
	r.frameDataTracker.Record(pv)
	r.zramInfoTracker.Record(pv)
	r.batteryInfoTracker.Record(pv)

	return nil
}

// SaveHistograms saves histogram raw data to a given directory.
func (r *Recorder) SaveHistograms(outDir string) error {
	// Save to file "recorder_histograms.json"
	filePath := path.Join(outDir, "recorder_histograms.json")
	j, err := json.MarshalIndent(r.histograms, "", "  ")
	if err != nil {
		return err
	}
	return ioutil.WriteFile(filePath, j, 0644)
}
