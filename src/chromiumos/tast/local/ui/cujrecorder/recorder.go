// Copyright 2019 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package cujrecorder has utilities for CUJ-style UI performance tests.
package cujrecorder

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/local/chrome/uiauto"
	perfSrc "chromiumos/tast/local/perf"
	"chromiumos/tast/local/power"
	"chromiumos/tast/local/power/setup"
	"chromiumos/tast/local/tracing"
	"chromiumos/tast/testing"
)

type metricGroup string

const (
	deprecatedGroupSmoothness metricGroup = "AnimationSmoothness"
	deprecatedGroupLatency    metricGroup = "InputLatency"
)

const (
	tpsMetricPrefix   = "TPS."
	powerMetricPrefix = "Power."
)

const checkInterval = 5 * time.Second

// SystemTraceConfigFile is a perfetto tracing config.
const SystemTraceConfigFile = "perfetto/system_trace_config.pbtxt"

// MetricConfig is the configuration for the recorder.
type MetricConfig struct {
	// The name of the histogram to be recorded.
	histogramName string

	// The unit of the histogram, like "percent" or "ms".
	unit string

	// The direction of the histogram.
	direction perf.Direction
}

// NewSmoothnessMetricConfig creates a new MetricConfig instance for collecting
// animation smoothness data for the given histogram name. The whole data of all
// smoothness metrics will be aggregated into the "AnimationSmoothness" entry at
// the end.
func NewSmoothnessMetricConfig(histogramName string) MetricConfig {
	return MetricConfig{histogramName: histogramName, unit: "percent", direction: perf.BiggerIsBetter}
}

// NewLatencyMetricConfig creates a new MetricConfig instance for collecting
// input latency data for the given histogram name. The whole data of all input
// latency metrics will be aggregated into the "InputLatency" entry at the end.
func NewLatencyMetricConfig(histogramName string) MetricConfig {
	return MetricConfig{histogramName: histogramName, unit: "ms", direction: perf.SmallerIsBetter}
}

// NewCustomMetricConfig creates a new MetricConfig for the given histogram
// name, unit, and direction. The data are reported as-is but
// not aggregated with other histograms.
func NewCustomMetricConfig(histogramName, unit string, direction perf.Direction) MetricConfig {
	return MetricConfig{histogramName: histogramName, unit: unit, direction: direction}
}

type record struct {
	config     MetricConfig
	totalCount int64

	// Sum is the sum of the all entries in the histogram.
	Sum int64 `json:"sum"`

	// Buckets contains ranges of reported values. It's the concatenated histogram buckets from multiple runs.
	Buckets []metrics.HistogramBucket `json:"buckets"`
}

// combine combines another record into an existing one.
func (rec *record) combine(newRec *record) error {
	if rec.config != newRec.config {
		return errors.New("records with different config cannot be combined")
	}
	rec.totalCount += newRec.totalCount
	rec.Sum += newRec.Sum
	rec.Buckets = append(rec.Buckets, newRec.Buckets...)
	return nil
}

// saveMetric records the metric into the perf values.
func (rec *record) saveMetric(pv *perf.Values, name string) {
	if rec.totalCount == 0 {
		return
	}
	pv.Set(perf.Metric{
		Name:      name,
		Unit:      rec.config.unit,
		Variant:   "average",
		Direction: rec.config.direction,
	}, float64(rec.Sum)/float64(rec.totalCount))
}

// Recorder is a utility to measure various metrics for CUJ-style tests.
type Recorder struct {
	cr    *chrome.Chrome
	tconn *chrome.TestConn

	// Metrics names keyed by relevant browser type.
	names map[browser.Type][]string

	// Keep one TestConn instance for each browser type.
	tconns map[browser.Type]*chrome.TestConn

	// Metric records keyed by relevant browser type.
	// Its value is a map keyed by metric name.
	records map[browser.Type]map[string]*record

	traceDir        string
	perfettoCfgPath string

	// duration is the total running time of the recorder.
	duration time.Duration

	// Total number of times that the test has been successfully run.
	testCyclesCount int64

	// Time when recording was started.
	// Defined only for the running recorder.
	startedAtTm time.Time

	// Running recorder has these metrics recorders initialized for each metric
	// Defined only for the running recorder.
	mr map[browser.Type]*metrics.Recorder

	// A function to clean up started recording.
	// Defined only for the running recorder.
	cleanup func(ctx context.Context) error

	// powerSetupCleanup cleans up power setup.
	powerSetupCleanup setup.CleanupCallback

	// batteryDischarge is true if battery discharge was successfully induced.
	batteryDischarge bool

	// screenRecorderStart starts the screen recorder, if present.
	screenRecorderStart action.Action

	// screenRecorderCleanup cleans up the screen recorder, if present.
	screenRecorderCleanup func(ctx context.Context)

	screenshotRecorder uiauto.ScreenshotRecorder

	tpsTimeline        *perf.Timeline
	powerTimeline      *perf.Timeline
	gpuDataSource      *perfSrc.GPUDataSource
	frameDataTracker   *perfSrc.FrameDataTracker
	zramInfoTracker    *perfSrc.ZramInfoTracker
	batteryInfoTracker *perfSrc.BatteryInfoTracker
	memInfoTracker     *perfSrc.MemoryInfoTracker
	loginEventRecorder *perfSrc.LoginEventRecorder
}

// RecorderOptions contains options to control the recorder setup.
// The options are determined based on the test needs.
type RecorderOptions struct {
	// DischargeThreshold is the battery discharge threshold.
	// If not set, defaultDischargeThreshold will be used.
	DischargeThreshold *float64
	// FailOnDischargeErr, if set, will cause test to fail on battery discharge error.
	// NoBatteryError is not considered as dischage error, though.
	FailOnDischargeErr   bool
	DoNotChangeWifi      bool
	DoNotChangePowerd    bool
	DoNotChangeDPTF      bool
	DoNotChangeAudio     bool
	DoNotChangeBluetooth bool
}

var performanceCUJDischargeThreshold = 55.0

// NewPerformanceCUJOptions indicates the power test settings for performance CUJs run by partners.
func NewPerformanceCUJOptions() RecorderOptions {
	return RecorderOptions{
		DischargeThreshold: &performanceCUJDischargeThreshold,
		FailOnDischargeErr: true,
		DoNotChangeWifi:    true,
		DoNotChangePowerd:  true,
		DoNotChangeDPTF:    true,
		DoNotChangeAudio:   true,
	}
}

// AddCollectedMetrics adds |configs| to the collected metrics for browser |bt| using the |tconn|
// as test connection.
func (r *Recorder) AddCollectedMetrics(tconn *chrome.TestConn, bt browser.Type, configs ...MetricConfig) error {
	if tconn == nil {
		return errors.New("tconn must never be nil")
	}
	if !r.startedAtTm.IsZero() {
		return errors.New("canont modify list of collected metrics after recodding was started")
	}

	// Keep the tconn so it can be used to collect metrics from the browser.
	r.tconns[bt] = tconn

	for _, config := range configs {
		if config.histogramName == string(deprecatedGroupLatency) || config.histogramName == string(deprecatedGroupSmoothness) {
			return errors.Errorf("invalid histogram name: %s", config.histogramName)
		}
		r.names[bt] = append(r.names[bt], config.histogramName)
		if _, ok := r.records[bt]; !ok {
			r.records[bt] = make(map[string]*record)
		}
		r.records[bt][config.histogramName] = &record{config: config}
	}
	return nil
}

// AddCommonMetrics adds MetricConfigs defined by CommonMetrics to the collected metrics.
// tconn will be used as the test connection for Ash and AnyChrome common metrics. bTconn
// will be used for the Browser common metrics. If bTconn is different than tconn, then
// another set of AnyChrome common metrics will be collected with bTconn used as the test
// connection.
func (r *Recorder) AddCommonMetrics(tconn, bTconn *chrome.TestConn) error {
	var bt browser.Type
	if tconn == bTconn {
		bt = browser.TypeAsh
	} else {
		bt = browser.TypeLacros
	}
	if err := r.AddCollectedMetrics(tconn, browser.TypeAsh, CUJAshCommonMetricConfigs()...); err != nil {
		return errors.Wrap(err, "failed to add Ash common metrics")
	}
	if err := r.AddCollectedMetrics(bTconn, bt, CUJBrowserCommonMetricConfigs()...); err != nil {
		return errors.Wrapf(err, "failed to add Browser(%s) common metrics", bt)
	}
	if err := r.AddCollectedMetrics(tconn, browser.TypeAsh, CUJAnyChromeCommonMetricConfigs()...); err != nil {
		return errors.Wrap(err, "failed to add Ash AnyChrome common metrics")
	}
	if tconn != bTconn {
		if err := r.AddCollectedMetrics(bTconn, browser.TypeLacros, CUJLacrosCommonMetricConfigs()...); err != nil {
			return errors.Wrap(err, "failed to add Lacros common metrics")
		}
		if err := r.AddCollectedMetrics(bTconn, browser.TypeLacros, CUJAnyChromeCommonMetricConfigs()...); err != nil {
			return errors.Wrap(err, "failed to add Lacros AnyChrome common metrics")
		}
	}
	return nil
}

// AddScreenRecorder creates a screen recorder that will record the
// device during the execution of recorder.Run. The screen recording is
// saved with the |testName| as the filename prefix.
func (r *Recorder) AddScreenRecorder(ctx context.Context, tconn *chrome.TestConn, testName string) error {
	dir, ok := testing.ContextOutDir(ctx)
	if !ok || dir == "" {
		return errors.New("failed to get the out directory to save the screen recording")
	}

	screenRecorder, err := uiauto.NewScreenRecorder(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to create ScreenRecorder")
	}

	r.screenRecorderStart = func(ctx context.Context) error {
		return screenRecorder.Start(ctx, tconn)
	}
	r.screenRecorderCleanup = func(ctx context.Context) {
		uiauto.ScreenRecorderStopSaveRelease(ctx, screenRecorder, filepath.Join(dir, fmt.Sprintf("%s-record.webm", testName)))
	}
	return nil
}

// AddScreenshotRecorder creates a screenshot recorder that takes
// screenshots at every |interval| while executing recorder.Run.
// |maxImages| is the maximum number of images the screenshot recorder
// is allowed to take.
func (r *Recorder) AddScreenshotRecorder(ctx context.Context, interval time.Duration, maxImages int) error {
	var err error
	r.screenshotRecorder, err = uiauto.NewScreenshotRecorder(ctx, interval, maxImages)
	if err != nil {
		return errors.Wrap(err, "failed to setup new screenshot recorder")
	}
	return nil
}

// NewRecorderWithTestConn creates a Recorder. It also aggregates the metrics of each
// category (animation smoothness and input latency) and creates the aggregated
// reports.
func NewRecorderWithTestConn(ctx context.Context, tconn *chrome.TestConn, cr *chrome.Chrome, bTconn *chrome.TestConn, a *arc.ARC, options RecorderOptions) (*Recorder, error) {
	if tconn == nil {
		return nil, errors.New("tconn must never be nil")
	}
	// Keep one TestConn for each browser.
	tconns := make(map[browser.Type]*chrome.TestConn)
	tconns[browser.TypeAsh] = tconn
	if bTconn != nil && *tconn != *bTconn {
		tconns[browser.TypeLacros] = bTconn
	}
	r := &Recorder{cr: cr, tconn: tconn, tconns: tconns}

	powerTestOptions := setup.PowerTestOptions{
		// The default for the following options is to disable these setting.
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
	var dischargeThreshold = setup.DefaultDischargeThreshold
	if options.DischargeThreshold != nil {
		dischargeThreshold = *options.DischargeThreshold
	}
	// Create batteryDischarge with both discharge and ignoreErr set to true.
	batteryDischarge := setup.NewBatteryDischarge(true, true, dischargeThreshold)

	var err error
	r.powerSetupCleanup, err = setup.PowerTest(ctx, r.tconn, powerTestOptions, batteryDischarge)
	batteryDischargeErr := batteryDischarge.Err()
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
	// Check options.FailOnDischargeErr after the deferred function is set.
	if batteryDischargeErr != nil && options.FailOnDischargeErr &&
		!errors.Is(batteryDischargeErr, power.ErrNoBattery) {
		return nil, errors.Wrap(batteryDischargeErr, "battery discharge failed")
	}

	r.gpuDataSource = perfSrc.NewGPUDataSource(r.tconns)
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

	r.names = make(map[browser.Type][]string)
	r.records = make(map[browser.Type]map[string]*record)

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

// NewRecorder creates a Recorder based on the configs. It also aggregates the
// metrics of each category (animation smoothness and input latency) and creates
// the aggregated reports.
func NewRecorder(ctx context.Context, cr *chrome.Chrome, bTconn *chrome.TestConn, a *arc.ARC, options RecorderOptions) (*Recorder, error) {
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "creating test API connection failed")
	}
	return NewRecorderWithTestConn(ctx, tconn, cr, bTconn, a, options)
}

// EnableTracing enables system tracing when the recorder is running a test scenario.
func (r *Recorder) EnableTracing(traceDir, perfettoCfgPath string) {
	r.traceDir = traceDir
	r.perfettoCfgPath = perfettoCfgPath
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
// function.
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

	if r.screenRecorderStart != nil {
		if err := r.screenRecorderStart(ctx); err != nil {
			return nil, errors.Wrap(err, "failed to start screen recorder")
		}
	}

	if r.screenshotRecorder != nil {
		if err := r.screenshotRecorder.Start(ctx); err != nil {
			return nil, errors.Wrap(err, "failed to start screenshot recorder")
		}
	}

	if r.traceDir != "" && r.perfettoCfgPath != "" {
		sess, err := tracing.StartSession(ctx, r.perfettoCfgPath)
		testing.ContextLog(ctx, "Starting system tracing session")
		if err != nil {
			return nil, errors.Wrap(err, "failed to start tracing")
		}
		stopTracing := func(ctx context.Context) error {
			if err := sess.Stop(); err != nil {
				return errors.Wrap(err, "failed to stop tracing")
			}
			testing.ContextLog(ctx, "Stopping system tracing session")

			data, err := ioutil.ReadAll(sess.TraceResultFile)
			if err != nil {
				return errors.Wrap(err, "failed to read from the temp file of trace result")
			}

			filename := "trace.data.gz"
			file, err := os.OpenFile(filepath.Join(r.traceDir, filename), os.O_CREATE|os.O_RDWR, 0644)
			if err != nil {
				return errors.Wrap(err, "could not open file")
			}
			defer func() {
				if err := file.Close(); err != nil {
					testing.ContextLog(ctx, "Failed to close file: ", err)
				}
			}()

			writer := gzip.NewWriter(file)
			defer func() {
				if err := writer.Close(); err != nil {
					testing.ContextLog(ctx, "Failed to close gzip writer: ", err)
				}
			}()

			if _, err := writer.Write(data); err != nil {
				return errors.Wrap(err, "could not write the data")
			}

			if err := writer.Flush(); err != nil {
				return errors.Wrap(err, "could not flush the gzip writer")
			}

			// The temporary file of trace data is no longer needed when returned.
			sess.RemoveTraceResultFile()

			return nil
		}
		cancel = func(ctx context.Context) error {
			err := stopTracing(ctx)
			cancelRunCtx()
			return err
		}
	}

	// Start metrics recording per browser.
	r.mr = make(map[browser.Type]*metrics.Recorder)
	for bt, names := range r.names {
		var err error
		r.mr[bt], err = metrics.StartRecorder(ctx, r.tconns[bt], names...)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to start metrics recorder for browser %v", bt)
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
// startRecording above).
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

	if r.screenRecorderCleanup != nil {
		r.screenRecorderCleanup(ctx)
	}

	if r.screenshotRecorder != nil {
		if err := r.screenshotRecorder.Stop(ctx); err != nil {
			testing.ContextLog(ctx, "Failed to stop screenshot recorder: ", err)
		}
	}

	// Collects metrics per browser type.
	tHists := make(map[browser.Type][]*metrics.Histogram)
	for bt, rr := range r.mr {
		tconn := r.tconns[bt]
		h, err := rr.Histogram(runCtx, tconn)
		if err != nil {
			return errors.Wrapf(err, "failed to collect metrics from browser %v", bt)
		}
		testing.ContextLogf(ctx, "The following metrics are collected from %q: %v", bt+"-Chrome", histsWithSamples(h))
		tHists[bt] = append(tHists[bt], h...)
	}
	// Reset recorders and context.
	r.mr = nil

	for bt, hists := range tHists {
		for _, hist := range hists {
			if hist.TotalCount() == 0 {
				continue
			}
			// Combine histogram result to the record.
			if err := r.records[bt][hist.Name].combine(&record{config: r.records[bt][hist.Name].config,
				totalCount: hist.TotalCount(), Sum: hist.Sum, Buckets: hist.Buckets}); err != nil {
				return err
			}
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

	allRecords := make(map[string]*record) // Combined records from all browsers.

	// Record records by browser.
	for bt, records := range r.records {
		for name, rec := range records {
			if rec.totalCount == 0 {
				continue
			}
			// Append metric name with browser type as the new metric name, for example:
			// - EventLatency.TotalLatency_ash-Chrome,
			// - PageLoad.InteractiveTiming.InputDelay3_lacros-Chrome
			rec.saveMetric(pv, fmt.Sprintf("%s_%s-Chrome", name, bt))
			// Combine the record.
			if _, ok := allRecords[name]; !ok {
				allRecords[name] = &record{config: rec.config}
			}
			if err := allRecords[name].combine(rec); err != nil {
				return err
			}
		}
	}
	var crasUnderruns float64
	// Record combined records from all tconns.
	for name, rec := range allRecords {
		if name == "Cras.UnderrunsPerDevice" {
			crasUnderruns = float64(rec.Sum)
			// We are not interested in reporting Cras.UnderrunsPerDevice but will use this value
			// to derive UnderrunsPerDevicePerMinute. Continue the loop.
			continue
		}
		// Metric name recorded is the original histogram name. For example:
		// - EventLatency.TotalLatency
		// - PageLoad.InteractiveTiming.InputDelay3
		rec.saveMetric(pv, name)
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
	saveJSONFile := func(fileName string, records map[string]*record) error {
		filePath := path.Join(outDir, fileName+".json")
		j, err := json.MarshalIndent(records, "", "  ")
		if err != nil {
			return errors.Wrapf(err, "failed to marshall data for %s json file: %v", fileName, records)
		}
		if err := ioutil.WriteFile(filePath, j, 0644); err != nil {
			return errors.Wrapf(err, "failed to write %s json file", fileName)
		}
		return nil
	}

	const histogramFileName = "recorder_histograms"
	allRecords := make(map[string]*record) // Combined records from all browsers.

	for bt, records := range r.records {
		// File for browser based histogram will be appended with the browser name.
		// For example:
		//   - recorder_histograms_ash-Chrome.json
		//   - recorder_histograms_lacros-Chrome.json
		fileName := fmt.Sprintf("%s_%s-Chrome", histogramFileName, bt)
		if err := saveJSONFile(fileName, records); err != nil {
			return err
		}
		for name, rec := range records {
			if _, ok := allRecords[name]; !ok {
				allRecords[name] = &record{config: rec.config}
			}
			// Combine the record.
			if err := allRecords[name].combine(rec); err != nil {
				return err
			}
		}
	}
	return saveJSONFile(histogramFileName, allRecords)
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
