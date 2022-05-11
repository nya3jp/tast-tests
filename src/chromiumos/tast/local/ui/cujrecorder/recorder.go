// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package cujrecorder has utilities for CUJ-style UI performance tests.
package cujrecorder

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
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/metrics"
	perfSrc "chromiumos/tast/local/perf"
	"chromiumos/tast/local/power"
	"chromiumos/tast/local/power/setup"
	"chromiumos/tast/testing"
)

type metricGroup string

const (
	groupSmoothness metricGroup = "AnimationSmoothness"
	groupLatency    metricGroup = "InputLatency"
	groupOther      metricGroup = ""
)

const (
	tpsMetricPrefix   = "TPS."
	powerMetricPrefix = "Power."
)

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
	// instances.
	jankCriteria []int64

	// The group of the metrics. Metrics in the same group will be aggregated
	// nto one, except for groupOther.
	group metricGroup

	// TestConn to pull the histogram. If nil, the histogram is fetched using
	// the TestConn in recorder.
	// TODO(b/230676548): Deprecated.
	tconn *chrome.TestConn
}

// NewSmoothnessMetricConfig creates a new MetricConfig instance for collecting
// animation smoothness data for the given histogram name. The whole data of all
// smoothness metrics will be aggregated into the "AnimationSmoothness" entry at
// the end.
func NewSmoothnessMetricConfig(histogramName string) MetricConfig {
	return MetricConfig{histogramName: histogramName, unit: "percent", direction: perf.BiggerIsBetter, jankCriteria: []int64{50, 20}, group: groupSmoothness}
}

// DeprecatedNewSmoothnessMetricConfigWithTestConn (use non-"WithTestConn"
// version) works like NewSmoothnessMetricConfig but allows specifying a
// TestConn to pull histogram data.
// TODO(b/230676548): Deprecated.
func DeprecatedNewSmoothnessMetricConfigWithTestConn(histogramName string, tconn *chrome.TestConn) MetricConfig {
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

// DeprecatedNewLatencyMetricConfigWithTestConn (use non-"WithTestConn"
// version) works like NewLatencyMetricConfig but allows specifying a TestConn
// to pull histogram data.
// TODO(b/230676548): Deprecated.
func DeprecatedNewLatencyMetricConfigWithTestConn(histogramName string, tconn *chrome.TestConn) MetricConfig {
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

// DeprecatedNewCustomMetricConfigWithTestConn (use non-"WithTestConn" version)
// works like NewCustomMetricConfig but allows specifying a TestConn to pull
// histogram data.
// TODO(b/230676548): Deprecated.
func DeprecatedNewCustomMetricConfigWithTestConn(histogramName, unit string,
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
	cs    ash.ConnSource
	tconn *chrome.TestConn

	// Metrics names keyed by relevant chrome.TestConn pointer.
	names map[*chrome.TestConn][]string

	// Metric records keyed by metric name.
	records map[string]*record

	traceDir string

	// duration is the total running time of the recorder.
	duration time.Duration

	// Total number of times that the test has been successfully run.
	testCyclesCount int64

	// Time when recording was started.
	// Defined only for the running recorder.
	startedAtTm time.Time

	// Running recorder has these metrics recorders initialized for each metric
	// Defined only for the running recorder.
	mr map[*chrome.TestConn]*metrics.Recorder

	// A function to clean up started recording.
	// Defined only for the running recorder.
	cleanup func(ctx context.Context) error

	// powerSetupCleanup cleans up power setup.
	powerSetupCleanup setup.CleanupCallback

	// batteryDischarge is true if battery discharge was successfully induced.
	batteryDischarge bool

	tpsTimeline        *perf.Timeline
	powerTimeline      *perf.Timeline
	gpuDataSource      *perfSrc.GPUDataSource
	frameDataTracker   *perfSrc.FrameDataTracker
	zramInfoTracker    *perfSrc.ZramInfoTracker
	batteryInfoTracker *perfSrc.BatteryInfoTracker
	memInfoTracker     *perfSrc.MemoryInfoTracker
	loginEventRecorder *perfSrc.LoginEventRecorder
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

// RecorderOptions indicates whether the services should not be changed.
// The following options are allowed the status of which is determined based on the test.
type RecorderOptions struct {
	DoNotChangeWifi      bool
	DoNotChangePowerd    bool
	DoNotChangeDPTF      bool
	DoNotChangeAudio     bool
	DoNotChangeBluetooth bool
}

// NewPerformanceCUJOptions indicates the power test settings for performance CUJs run by partners.
func NewPerformanceCUJOptions() RecorderOptions {
	return RecorderOptions{
		DoNotChangeWifi:   true,
		DoNotChangePowerd: true,
		DoNotChangeDPTF:   true,
		DoNotChangeAudio:  true,
	}
}

// addCollectedMetrics is a special version of AddCollectedMetrics that allows
// to use per-metric test connection. This is needed until MetricConfig.tconn
// is deprecated per b/230676548).
//
// tconn handling is different from the public AddCollectedMetrics() method.
// If tconn is nil, config's tconn is used.
func (r *Recorder) addCollectedMetrics(tconn *chrome.TestConn, configs ...MetricConfig) error {
	if !r.startedAtTm.IsZero() {
		return errors.New("canont modify list of collected metrics after recodding was started")
	}
	for _, config := range configs {
		if config.histogramName == string(groupLatency) || config.histogramName == string(groupSmoothness) {
			return errors.Errorf("invalid histogram name: %s", config.histogramName)
		}

		bTconn := tconn
		if bTconn == nil {
			// TODO(b/230676548): Remove this.
			bTconn = config.tconn
		}
		if bTconn == nil {
			// TODO(b/230676548): Remove this.
			bTconn = r.tconn
		}

		r.names[bTconn] = append(r.names[bTconn], config.histogramName)
		r.records[config.histogramName] = &record{config: config}
	}
	return nil
}

// AddCollectedMetrics adds |configs| to the collected metrics using the |tconn|
// as test connection. Note: MetricConfig.tconn is ignored (b/230676548)!
func (r *Recorder) AddCollectedMetrics(tconn *chrome.TestConn, configs ...MetricConfig) error {
	if tconn == nil {
		return errors.New("tconn must never be nil")
	}
	return r.addCollectedMetrics(tconn, configs...)
}

// NewRecorder creates a Recorder. It also aggregates the metrics of each
// category (animation smoothness and input latency) and creates the aggregated
// reports.
//
// TODO(b/230676548): |configs| is deprecated, use
// recorder.AddCollectedMetrics() instead.
func NewRecorder(ctx context.Context, cr *chrome.Chrome, cs ash.ConnSource, a *arc.ARC, options RecorderOptions, configs ...MetricConfig) (*Recorder, error) {
	r := &Recorder{cr: cr, cs: cs}

	var err error
	r.tconn, err = cr.TestAPIConn(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to test API")
	}

	var batteryDischargeErr error
	powerTestOptions := setup.PowerTestOptions{
		// The default for the following options is to disable these setting.
		Battery:    setup.TryBatteryDischarge(&batteryDischargeErr),
		Wifi:       setup.DisableWifiInterfaces,
		NightLight: setup.DisableNightLight,
		Powerd:     setup.DisablePowerd,
		DPTF:       setup.DisableDPTF,
		Audio:      setup.Mute,
		Bluetooth:  setup.DisableBluetoothInterfaces,
	}
	// Check recorder options and don't change them when required.
	if options.DoNotChangeWifi {
		powerTestOptions.Wifi = setup.DoNotChangeWifiInterfaces
	}
	if options.DoNotChangePowerd {
		powerTestOptions.Powerd = setup.DoNotChangePowerd
	}
	if options.DoNotChangeDPTF {
		powerTestOptions.DPTF = setup.DoNotChangeDPTF
	}
	if options.DoNotChangeAudio {
		powerTestOptions.Audio = setup.DoNotChangeAudio
	}
	if options.DoNotChangeBluetooth {
		powerTestOptions.Bluetooth = setup.DoNotChangeBluetooth
	}

	r.powerSetupCleanup, err = setup.PowerTest(ctx, r.tconn, powerTestOptions)
	if batteryDischargeErr != nil {
		testing.ContextLog(ctx, "Failed to induce battery discharge: ", batteryDischargeErr)
	} else {
		r.batteryDischarge = true
	}
	if err != nil {
		return nil, errors.Wrap(err, "power setup failed")
	}
	success := false
	defer func(ctx context.Context) {
		if success {
			return
		}
		if err := r.powerSetupCleanup(ctx); err != nil {
			testing.ContextLog(ctx, "Failed to clean up power setup: ", err)
		}
	}(ctx)

	r.gpuDataSource = perfSrc.NewGPUDataSource(r.tconn)
	r.tpsTimeline, err = perf.NewTimeline(ctx, []perf.TimelineDatasource{
		perfSrc.NewCPUUsageSource("CPU"),
		perfSrc.NewThermalDataSource(),
		r.gpuDataSource,
		perfSrc.NewMemoryDataSource("RAM.Absolute", "RAM.Diff.Absolute", "RAM"),
	}, perf.Interval(checkInterval), perf.Prefix(tpsMetricPrefix))
	if err != nil {
		return nil, errors.Wrap(err, "failed to create TPS timeline")
	}

	r.powerTimeline, err = perf.NewTimeline(ctx, power.TestMetrics(), perf.Interval(checkInterval), perf.Prefix(powerMetricPrefix))
	if err != nil {
		return nil, errors.Wrap(err, "failed to create power timeline")
	}

	if err := r.tpsTimeline.Start(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to start TPS timeline")
	}

	if err := r.powerTimeline.Start(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to start power timeline")
	}

	r.frameDataTracker, err = perfSrc.NewFrameDataTracker(tpsMetricPrefix)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create FrameDataTracker")
	}

	r.zramInfoTracker, err = perfSrc.NewZramInfoTracker(tpsMetricPrefix)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create ZramInfoTracker")
	}

	r.batteryInfoTracker, err = perfSrc.NewBatteryInfoTracker(ctx, tpsMetricPrefix)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create BatteryInfoTracker")
	}

	r.memInfoTracker = perfSrc.NewMemoryTracker(a)

	r.loginEventRecorder = perfSrc.NewLoginEventRecorder(tpsMetricPrefix)

	r.names = make(map[*chrome.TestConn][]string)
	r.records = make(map[string]*record, len(configs)+2)
	r.addCollectedMetrics(nil, configs...)
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

	if err := r.frameDataTracker.Start(ctx, r.tconn); err != nil {
		return nil, errors.Wrap(err, "failed to start FrameDataTracker")
	}
	defer func(ctx context.Context) {
		if success {
			return
		}
		if err := r.frameDataTracker.Stop(ctx, r.tconn); err != nil {
			testing.ContextLog(ctx, "Failed to stop frame data tracker: ", err)
		}
	}(ctx)

	if err := r.zramInfoTracker.Start(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to start ZramInfoTracker")
	}

	if err := r.batteryInfoTracker.Start(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to start BatteryInfoTracker")
	}

	if err := r.tpsTimeline.StartRecording(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to start recording TPS timeline data")
	}

	if err := r.powerTimeline.StartRecording(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to start recording power timeline data")
	}

	if err := r.memInfoTracker.Start(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to start recording memory data")
	}
	// loginEventRecorder.Prepare() may not be needed because we usually start
	// Chrome with --keep-login-events-for-testing flag that will start
	// LoginEventRecorder data collection automatically. But we do it here
	// just in case Chrome was started with different parameters.
	if err := r.loginEventRecorder.Prepare(ctx, r.tconn); err != nil {
		return nil, errors.Wrap(err, "failed to start recording login event data")
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
	var firstErr error
	if err := r.powerSetupCleanup(ctx); err != nil {
		firstErr = errors.Wrap(err, "failed to clean up power setup")
	}
	r.gpuDataSource.Close()
	if err := r.frameDataTracker.Close(ctx, r.tconn); firstErr == nil && err != nil {
		firstErr = errors.Wrap(err, "failed to close frame data tracker")
	}
	return firstErr
}

// startRecording starts to record CUJ data.
//
// In:
// * context to initialize data recording (and tracing if needed).
//
// Out:
// * New context (with reduced timeout) that should be used to run the test
//   function.
// * Error
func (r *Recorder) startRecording(ctx context.Context) (runCtx context.Context, e error) {
	if !r.startedAtTm.IsZero() {
		return nil, errors.New("start requested on the started recorder")
	}
	if r.mr != nil || r.cleanup != nil {
		return nil, errors.New("start requested but some paramerters are already initialized:" + fmt.Sprintf(" mr=%v, r.cleanup=%p", r.mr, r.cleanup))
	}

	const traceCleanupDuration = 2 * time.Second
	runCtx, cancelRunCtx := ctxutil.Shorten(ctx, traceCleanupDuration)
	cancel := func(ctx context.Context) error {
		cancelRunCtx()
		return nil
	}
	defer func(ctx context.Context) {
		// If this function finishes without errors, cleanup will happen in stopRecording
		if e == nil {
			return
		}
		if err := cancel(ctx); err != nil {
			// We cannot overwrite e here.
			testing.ContextLogf(ctx, "Failed to cleanup after startRecording: %s", err)
		}
		r.cleanup = nil
		r.startedAtTm = time.Time{} // Reset to zero.
		r.mr = nil
	}(ctx)

	if r.traceDir != "" {
		if err := r.cs.StartTracing(ctx,
			[]string{"benchmark", "cc", "gpu", "input", "toplevel", "ui", "views", "viz", "memory-infra"},
			browser.DisableSystrace()); err != nil {
			testing.ContextLog(ctx, "Failed to start tracing: ", err)
			return nil, errors.Wrap(err, "failed to start tracing")
		}
		stopTracing := func(ctx context.Context) error {
			tr, err := r.cs.StopTracing(ctx)
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

// stopRecording stops CUJ data recording.
//
// In:
// * context used to initialise recording (the one that was passed to the
//   startRecording above).
// * shorted context returned from the startRecording()
//
// Out:
// * Error
func (r *Recorder) stopRecording(ctx, runCtx context.Context) (e error) {
	if r.startedAtTm.IsZero() {
		return errors.New("Stop requested on the stopped recorder")
	}
	if r.mr == nil || r.cleanup == nil {
		return errors.New("Stop requested but recorder was not fully started: " + fmt.Sprintf(" mr=%v, r.cleanup=%p", r.mr, r.cleanup))
	}

	defer func(ctx context.Context) {
		err := r.cleanup(ctx)
		if err != nil {
			testing.ContextLogf(ctx, "Failed to stop recording: %s", err)
		}
		if e == nil && err != nil {
			e = errors.Wrap(err, "failed to cleanup after stopRecording")
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
		connName := "lacros-Chrome"
		// Check if the tconn uses the same underlying session connection with r.tconn.
		// We compare the value of the two pointers.
		if *tconn == *r.tconn {
			connName = "ash-Chrome"
		}
		testing.ContextLogf(ctx, "The following metrics are collected from %q: %v", connName, histsWithSamples(h))
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
// should go into startRecording()/stopRecording() to allow tests with
// different runners to accommodate them.
//
// This function also serves as an example for test developers on how to
// incorporate CUJ data recording into other tests.
func (r *Recorder) Run(ctx context.Context, f func(ctx context.Context) error) (e error) {
	runCtx, err := r.startRecording(ctx)
	if err != nil {
		return err
	}
	defer func(ctx, runCtx context.Context) {
		err := r.stopRecording(ctx, runCtx)
		if e == nil && err != nil {
			e = err
		} else if err != nil {
			testing.ContextLogf(ctx, "Failed to stop recording: %s", err)
		}
	}(ctx, runCtx)
	if err := f(runCtx); err != nil {
		return err
	}
	r.testCyclesCount++
	return nil
}

// RunFor conducts the test scenario f repeatedly for a given minimum
// duration. It may exceed that duration to complete the last call to f.
func (r *Recorder) RunFor(ctx context.Context, f func(ctx context.Context) error, minimumDuration time.Duration) error {
	return r.Run(ctx, func(ctx context.Context) error {
		for end := time.Now().Add(minimumDuration); time.Now().Before(end); {
			if err := f(ctx); err != nil {
				return err
			}
			r.testCyclesCount++
		}

		// Decrement test cycles to prevent double counting, since
		// Run() increments cycles count independently
		r.testCyclesCount--

		return nil
	})
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

	tpsData, err := r.tpsTimeline.StopRecording(ctx)
	if err != nil {
		testing.ContextLog(ctx, "Failed to stop TPS timeline: ", err)
		if stopErr == nil {
			stopErr = errors.Wrap(err, "failed to stop TPS timeline")
		}
	}

	powerData, err := r.powerTimeline.StopRecording(ctx)
	if err != nil {
		testing.ContextLog(ctx, "Failed to stop power timeline: ", err)
		if stopErr == nil {
			stopErr = errors.Wrap(err, "failed to stop power timeline")
		}
	}

	if err := r.memInfoTracker.Stop(ctx); err != nil {
		testing.ContextLog(ctx, "Failed to stop MemInfoTracker: ", err)
		if stopErr == nil {
			stopErr = errors.Wrap(err, "failed to stop MemInfoTracker")
		}
	}
	if err := r.loginEventRecorder.FetchLoginEvents(ctx, r.tconn); err != nil {
		testing.ContextLog(ctx, "Failed to fetch login events date: ", err)
		if stopErr == nil {
			stopErr = errors.Wrap(err, "failed to fetch login events")
		}
	}

	if stopErr != nil {
		return stopErr
	}
	pv.Merge(tpsData)
	pv.Merge(powerData)

	displayInfo, err := perfSrc.NewDisplayInfo(ctx, r.tconn)
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

	var batteryDischargeReport float64
	if r.batteryDischarge {
		batteryDischargeReport = 1
	}
	pv.Set(perf.Metric{
		Name:      powerMetricPrefix + "MetricsCollectedWithBatteryDischarge",
		Unit:      "unitless",
		Direction: perf.BiggerIsBetter,
	}, batteryDischargeReport)

	pv.Set(perf.Metric{
		Name:      "TestMetrics.TestCyclesCount",
		Unit:      "count",
		Direction: perf.SmallerIsBetter,
	}, float64(r.testCyclesCount))

	pv.Set(perf.Metric{
		Name: "TestMetrics.TotalTestRunTime",
		Unit: "s",
		// Longer runtime correlates to better performance data, so bigger is better
		Direction: perf.BiggerIsBetter,
	}, r.duration.Seconds())

	displayInfo.Record(pv)
	r.frameDataTracker.Record(pv)
	r.zramInfoTracker.Record(pv)
	r.batteryInfoTracker.Record(pv)
	r.memInfoTracker.Record(pv)
	r.loginEventRecorder.Record(ctx, pv)

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

// histsWithSamples returns the names of the histograms that have at least one sample.
func histsWithSamples(hists []*metrics.Histogram) []string {
	var histNames []string
	for _, hist := range hists {
		if hist.TotalCount() > 0 {
			histNames = append(histNames, hist.Name)
		}
	}
	return histNames
}
