// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package cuj has utilities for CUJ-style UI performance tests.
package cuj

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path"
	"path/filepath"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
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

const checkInterval = 5 * time.Second

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

	// TestConn to pull the histogram. If nil, the histogram is fetched using
	// the TestConn in recorder.
	tconn *chrome.TestConn
}

// NewSmoothnessMetricConfig creates a new MetricConfig instance for collecting
// animation smoothness data for the given histogram name. The whole data of all
// smoothness metrics will be aggregated into the "AnimationSmoothness" entry at
// the end.
func NewSmoothnessMetricConfig(histogramName string) MetricConfig {
	return MetricConfig{histogramName: histogramName, unit: "percent", direction: perf.BiggerIsBetter, jankCriteria: []int64{50, 20}, group: groupSmoothness}
}

// NewSmoothnessMetricConfigWithTestConn works like NewSmoothnessMetricConfig
// but allows specifying a TestConn to pull histogram data.
func NewSmoothnessMetricConfigWithTestConn(histogramName string, tconn *chrome.TestConn) MetricConfig {
	conf := NewSmoothnessMetricConfig(histogramName)
	conf.tconn = tconn
	return conf
}

// NewLatencyMetricConfig creates a new MetricConfig instance for collecting
// input latency data for the given histogram name. The whole data of all input
// latency metrics will be aggregated into the "InputLatency" entry at the end.
func NewLatencyMetricConfig(histogramName string) MetricConfig {
	return MetricConfig{histogramName: histogramName, unit: "ms", direction: perf.SmallerIsBetter, jankCriteria: []int64{100, 250}, group: groupLatency}
}

// NewLatencyMetricConfigWithTestConn works like NewLatencyMetricConfig but
// allows specifying a TestConn to pull histogram data.
func NewLatencyMetricConfigWithTestConn(histogramName string, tconn *chrome.TestConn) MetricConfig {
	conf := NewLatencyMetricConfig(histogramName)
	conf.tconn = tconn
	return conf
}

// NewCustomMetricConfig creates a new MetricConfig for the given histogram
// name, unit, direction, and jankCriteria. The data are reported as-is but
// not aggregated with other histograms.
func NewCustomMetricConfig(histogramName, unit string, direction perf.Direction, jankCriteria []int64) MetricConfig {
	return MetricConfig{histogramName: histogramName, unit: unit, direction: direction, jankCriteria: jankCriteria, group: groupOther}
}

// NewCustomMetricConfigWithTestConn works like NewCustomMetricConfig but
// allows specifying a TestConn to pull histogram data.
func NewCustomMetricConfigWithTestConn(histogramName, unit string,
	direction perf.Direction, jankCriteria []int64, tconn *chrome.TestConn) MetricConfig {
	conf := NewCustomMetricConfig(histogramName, unit, direction, jankCriteria)
	conf.tconn = tconn
	return conf
}

type record struct {
	config     MetricConfig
	totalCount int64
	jankCounts [2]float64

	// The following fields can be outputted to json file as histogram raw data.

	// Sum is the sum of the all entries in the histogram.
	Sum int64 `json:"sum"`
	// Buckets contains ranges of reported values. It's the concatenated histogram buckets from multiple runs.
	Buckets []metrics.HistogramBucket `json:"buckets"`
}

// Recorder is a utility to measure various metrics for CUJ-style tests.
type Recorder struct {
	cr    *chrome.Chrome
	tconn *chrome.TestConn

	// Metrics names keyed by relevant chrome.TestConn pointer.
	names map[*chrome.TestConn][]string

	// Metric records keyed by metric name.
	records map[string]*record

	traceDir string

	// duration is the total running time of the recorder.
	duration time.Duration

	// Time when recording was started.
	// Defined only for the running recorder.
	startedAtTm time.Time

	// Running recorder has these metrics recorders initialized for each metric
	// Defined only for the running recorder.
	mr map[*chrome.TestConn]*metrics.Recorder

	// A function to clean up started recording.
	// Defined only for the running recorder.
	cleanup func(ctx context.Context) error

	timeline           *perf.Timeline
	gpuDataSource      *gpuDataSource
	frameDataTracker   *FrameDataTracker
	zramInfoTracker    *ZramInfoTracker
	batteryInfoTracker *BatteryInfoTracker
	memInfoTracker     *MemoryInfoTracker
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
func NewRecorder(ctx context.Context, cr *chrome.Chrome, a *arc.ARC, configs ...MetricConfig) (*Recorder, error) {
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to test API")
	}

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

	memInfoTracker := NewMemoryTracker(a)

	r := &Recorder{
		cr:                 cr,
		tconn:              tconn,
		names:              make(map[*chrome.TestConn][]string),
		records:            make(map[string]*record, len(configs)+2),
		timeline:           timeline,
		gpuDataSource:      gpuDS,
		frameDataTracker:   frameDataTracker,
		zramInfoTracker:    zramInfoTracker,
		batteryInfoTracker: batteryInfoTracker,
		memInfoTracker:     memInfoTracker,
	}
	for _, config := range configs {
		if config.histogramName == string(groupLatency) || config.histogramName == string(groupSmoothness) {
			return nil, errors.Errorf("invalid histogram name: %s", config.histogramName)
		}

		bTconn := tconn
		if config.tconn != nil {
			bTconn = config.tconn
		}

		r.names[bTconn] = append(r.names[bTconn], config.histogramName)
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

	if err := r.memInfoTracker.Start(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to start recording memory data")
	}

	success = true

	return r, nil
}

// EnableTracing enables tracing when the recorder running test scenario.
func (r *Recorder) EnableTracing(traceDir string) {
	r.traceDir = traceDir
}

// Close clears states for all trackers.
func (r *Recorder) Close(ctx context.Context) error {
	r.gpuDataSource.Close()
	return r.frameDataTracker.Close(ctx, r.tconn)
}

// StartRecording starts to record CUJ data.
//
// In:
// * context to initialize data recording (and tracing if needed).
//
// Out:
// * New context (with reduced timeout) that should be used to run the test
//   function.
// * Error
func (r *Recorder) StartRecording(ctx context.Context) (runCtx context.Context, e error) {
	if !r.startedAtTm.IsZero() {
		return nil, errors.New("start requested on the started recorder")
	}
	if r.mr != nil || r.cleanup != nil {
		return nil, errors.New("start requested but some paramerters are already initialized:" + fmt.Sprintf(" mr=%v, r.cleanup=%v", r.mr, r.cleanup))
	}

	const traceCleanupDuration = 2 * time.Second
	runCtx, cancelRunCtx := ctxutil.Shorten(ctx, traceCleanupDuration)
	cancel := func(ctx context.Context) error {
		cancelRunCtx()
		return nil
	}
	defer func(ctx context.Context) {
		// If this function finishes without errors, cleanup will happen in StopRecording
		if e == nil {
			return
		}
		if err := cancel(ctx); err != nil {
			// We cannot overwrite e here.
			testing.ContextLogf(ctx, "Failed to cleanup after StartRecording: %s", err)
		}
		r.cleanup = nil
		r.startedAtTm = time.Time{} // Reset to zero.
		r.mr = nil
	}(ctx)

	if r.traceDir != "" {
		if err := r.cr.StartTracing(ctx,
			[]string{"benchmark", "cc", "gpu", "input", "toplevel", "ui", "views", "viz", "memory-infra"},
			browser.DisableSystrace()); err != nil {
			testing.ContextLog(ctx, "Failed to start tracing: ", err)
			return nil, errors.Wrap(err, "failed to start tracing")
		}
		stopTracing := func(ctx context.Context) error {
			tr, err := r.cr.StopTracing(ctx)
			if err != nil {
				testing.ContextLog(ctx, "Failed to stop tracing: ", err)
				return errors.Wrap(err, "failed to stop tracing")
			}
			if tr == nil || len(tr.Packet) == 0 {
				testing.ContextLog(ctx, "No trace data is collected")
				return errors.New("no trace data is collected")
			}
			filename := "trace.data.gz"
			if err := chrome.SaveTraceToFile(ctx, tr, filepath.Join(r.traceDir, filename)); err != nil {
				testing.ContextLog(ctx, "Failed to save trace to file: ", err)
				return errors.Wrap(err, "failed to save trace to file")
			}
			return nil
		}
		cancel = func(ctx context.Context) error {
			err := stopTracing(ctx)
			cancelRunCtx()
			return err
		}
	}

	// Starts metrics record per browser test connection.
	r.mr = make(map[*chrome.TestConn]*metrics.Recorder)
	for tconn, names := range r.names {
		var err error
		r.mr[tconn], err = metrics.StartRecorder(ctx, tconn, names...)
		if err != nil {
			return nil, errors.Wrap(err, "failed to start metrics recorder")
		}
	}
	r.cleanup = cancel

	// Remember when recording started.
	r.startedAtTm = time.Now()

	return runCtx, nil
}

// StopRecording stops CUJ data recording.
//
// In:
// * context used to initialise recording (the one that was passed to the
//   StartRecording above).
// * shorted context returned from the StartRecording()
//
// Out:
// * Error
func (r *Recorder) StopRecording(ctx, runCtx context.Context) (e error) {
	if r.startedAtTm.IsZero() {
		return errors.New("Stop requested on the stopped recorder")
	}
	if r.mr == nil || r.cleanup == nil {
		return errors.New("Stop requested but recorder was not fully started: " + fmt.Sprintf(" mr=%v, r.cleanup=%v", r.mr, r.cleanup))
	}

	defer func(ctx context.Context) {
		err := r.cleanup(ctx)
		if err != nil {
			testing.ContextLogf(ctx, "Failed to stop recording: %s", err)
		}
		if e == nil && err != nil {
			e = errors.Wrap(err, "failed to cleanup after StopRecording")
		}
		r.cleanup = nil
	}(ctx)
	r.duration += time.Now().Sub(r.startedAtTm)
	r.startedAtTm = time.Time{} // Reset to zero.

	// Collects metrics per browser test connection.
	var hists []*metrics.Histogram
	for tconn, rr := range r.mr {
		h, err := rr.Histogram(runCtx, tconn)
		if err != nil {
			return errors.Wrap(err, "failed to collect metrics")
		}
		hists = append(hists, h...)
	}
	// Reset recorders and context.
	r.mr = nil

	for _, hist := range hists {
		if hist.TotalCount() == 0 {
			continue
		}
		record := r.records[hist.Name]
		record.totalCount += hist.TotalCount()
		record.Sum += hist.Sum
		jankCounts := []float64{
			getJankCounts(hist, record.config.direction, record.config.jankCriteria[0]),
			getJankCounts(hist, record.config.direction, record.config.jankCriteria[1]),
		}
		record.jankCounts[0] += jankCounts[0]
		record.jankCounts[1] += jankCounts[1]

		// Concatenate buckets.
		record.Buckets = append(record.Buckets, hist.Buckets...)

		if totalRecord, ok := r.records[string(record.config.group)]; ok {
			totalRecord.totalCount += hist.TotalCount()
			totalRecord.Sum += hist.Sum
			totalRecord.jankCounts[0] += jankCounts[0]
			totalRecord.jankCounts[1] += jankCounts[1]
		}
	}
	return nil
}

// Run conducts the test scenario f, and collects the related metrics for the
// test scenario, and updates the internal data.
//
// This function should be kept to the bare minimum, all relevant changes
// should go into StartRecording()/StopRecording() to allow tests with
// different runners to accommodate them.
//
// This function also serves as an example for test developers on how to
// incorporate CUJ data recording into other tests.
func (r *Recorder) Run(ctx context.Context, f func(ctx context.Context) error) (e error) {
	runCtx, err := r.StartRecording(ctx)
	if err != nil {
		return err
	}
	defer func(ctx, runCtx context.Context) {
		err := r.StopRecording(ctx, runCtx)
		if e == nil && err != nil {
			e = err
		} else if err != nil {
			testing.ContextLogf(ctx, "Failed to stop recording: %s", err)
		}
	}(ctx, runCtx)
	if err := f(runCtx); err != nil {
		return err
	}
	return nil
}

// RunUntil calls Run repeatedly until the total duration of metrics collection (including
// any that was already done before this call to RunUntil) meets or exceeds a given minimum.
func (r *Recorder) RunUntil(ctx context.Context, f func(ctx context.Context) error, minimumTotalDuration time.Duration) (e error) {
	for r.duration < minimumTotalDuration {
		if err := r.Run(ctx, f); err != nil {
			return err
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

	vs, err := r.timeline.StopRecording(ctx)
	if err != nil {
		testing.ContextLog(ctx, "Failed to stop timeline: ", err)
		if stopErr == nil {
			stopErr = errors.Wrap(err, "failed to stop timeline")
		}
	}

	if err := r.memInfoTracker.Stop(ctx); err != nil {
		testing.ContextLog(ctx, "Failed to stop MemInfoTracker: ", err)
		if stopErr == nil {
			stopErr = errors.Wrap(err, "failed to stop MemInfoTracker")
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

	var crasUnderruns float64
	for name, record := range r.records {
		if record.totalCount == 0 {
			continue
		}
		if name == "Cras.UnderrunsPerDevice" {
			crasUnderruns = float64(record.Sum)
			// We are not interested in reporting Cras.UnderrunsPerDevice but will use this value
			// to derive UnderrunsPerDevicePerMinute. Continue the loop.
			continue
		}
		pv.Set(perf.Metric{
			Name:      name,
			Unit:      record.config.unit,
			Variant:   "average",
			Direction: record.config.direction,
		}, float64(record.Sum)/float64(record.totalCount))
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

	// Derive Cras.UnderrunsPerDevicePerMinute. Ideally, the audio playing time and number of CRAS audio device
	// should be captured. For now use the recorder running duration and assume there is only one device.
	pv.Set(perf.Metric{
		Name:      "Media.Cras.UnderrunsPerDevicePerMinute",
		Unit:      "count",
		Direction: perf.SmallerIsBetter,
	}, crasUnderruns/r.duration.Minutes())

	displayInfo.Record(pv)
	r.frameDataTracker.Record(pv)
	r.zramInfoTracker.Record(pv)
	r.batteryInfoTracker.Record(pv)
	r.memInfoTracker.Record(pv)

	return nil
}

// SaveHistograms saves histogram raw data to a given directory in a
// file named "recorder_histograms.json" by marshal the recorders.
func (r *Recorder) SaveHistograms(outDir string) error {
	filePath := path.Join(outDir, "recorder_histograms.json")
	j, err := json.MarshalIndent(r.records, "", "  ")
	if err != nil {
		return err
	}
	return ioutil.WriteFile(filePath, j, 0644)
}
