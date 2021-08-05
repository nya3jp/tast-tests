// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package gpucuj tests GPU CUJ tests on lacros Chrome and Chrome OS Chrome.
package gpucuj

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"

	"android.googlesource.com/platform/external/perfetto/protos/perfetto/trace"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/cdputil"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/local/lacros"
	"chromiumos/tast/local/power"
	"chromiumos/tast/lsbrelease"
	"chromiumos/tast/testing"
)

type metricInfo struct {
	unit      string
	direction perf.Direction
	uma       bool
}

var metricMap = map[string]metricInfo{
	"Graphics.Smoothness.Checkerboarding.TouchScroll": {
		unit:      "percent",
		direction: perf.SmallerIsBetter,
		uma:       true,
	},
	"Graphics.Smoothness.Checkerboarding.WheelScroll": {
		unit:      "percent",
		direction: perf.SmallerIsBetter,
		uma:       true,
	},
	"Graphics.Smoothness.PercentDroppedFrames.AllAnimations": {
		unit:      "percent",
		direction: perf.SmallerIsBetter,
		uma:       true,
	},
	"Graphics.Smoothness.PercentDroppedFrames.AllInteractions": {
		unit:      "percent",
		direction: perf.SmallerIsBetter,
		uma:       true,
	},
	"Graphics.Smoothness.PercentDroppedFrames.AllSequences": {
		unit:      "percent",
		direction: perf.SmallerIsBetter,
		uma:       true,
	},
	"Graphics.Smoothness.PercentDroppedFrames.CompositorThread.CompositorAnimation": {
		unit:      "percent",
		direction: perf.SmallerIsBetter,
		uma:       true,
	},
	"Graphics.Smoothness.PercentDroppedFrames.CompositorThread.MainThreadAnimation": {
		unit:      "percent",
		direction: perf.SmallerIsBetter,
		uma:       true,
	},
	"Graphics.Smoothness.PercentDroppedFrames.CompositorThread.PinchZoom": {
		unit:      "percent",
		direction: perf.SmallerIsBetter,
		uma:       true,
	},
	"Graphics.Smoothness.PercentDroppedFrames.CompositorThread.RAF": {
		unit:      "percent",
		direction: perf.SmallerIsBetter,
		uma:       true,
	},
	"Graphics.Smoothness.PercentDroppedFrames.CompositorThread.ScrollbarScroll": {
		unit:      "percent",
		direction: perf.SmallerIsBetter,
		uma:       true,
	},
	"Graphics.Smoothness.PercentDroppedFrames.CompositorThread.TouchScroll": {
		unit:      "percent",
		direction: perf.SmallerIsBetter,
		uma:       true,
	},
	"Graphics.Smoothness.PercentDroppedFrames.CompositorThread.Video": {
		unit:      "percent",
		direction: perf.SmallerIsBetter,
		uma:       true,
	},
	"Graphics.Smoothness.PercentDroppedFrames.CompositorThread.WheelScroll": {
		unit:      "percent",
		direction: perf.SmallerIsBetter,
		uma:       true,
	},
	"Graphics.Smoothness.PercentDroppedFrames.MainThread.CanvasAnimation": {
		unit:      "percent",
		direction: perf.SmallerIsBetter,
		uma:       true,
	},
	"Graphics.Smoothness.PercentDroppedFrames.MainThread.CompositorAnimation": {
		unit:      "percent",
		direction: perf.SmallerIsBetter,
		uma:       true,
	},
	"Graphics.Smoothness.PercentDroppedFrames.MainThread.JSAnimation": {
		unit:      "percent",
		direction: perf.SmallerIsBetter,
		uma:       true,
	},
	"Graphics.Smoothness.PercentDroppedFrames.MainThread.MainThreadAnimation": {
		unit:      "percent",
		direction: perf.SmallerIsBetter,
		uma:       true,
	},
	"Graphics.Smoothness.PercentDroppedFrames.MainThread.PinchZoom": {
		unit:      "percent",
		direction: perf.SmallerIsBetter,
		uma:       true,
	},
	"Graphics.Smoothness.PercentDroppedFrames.MainThread.RAF": {
		unit:      "percent",
		direction: perf.SmallerIsBetter,
		uma:       true,
	},
	"Graphics.Smoothness.PercentDroppedFrames.MainThread.ScrollbarScroll": {
		unit:      "percent",
		direction: perf.SmallerIsBetter,
		uma:       true,
	},
	"Graphics.Smoothness.PercentDroppedFrames.MainThread.TouchScroll": {
		unit:      "percent",
		direction: perf.SmallerIsBetter,
		uma:       true,
	},
	"Graphics.Smoothness.PercentDroppedFrames.MainThread.Video": {
		unit:      "percent",
		direction: perf.SmallerIsBetter,
		uma:       true,
	},
	"Graphics.Smoothness.PercentDroppedFrames.MainThread.WheelScroll": {
		unit:      "percent",
		direction: perf.SmallerIsBetter,
		uma:       true,
	},
	"Graphics.Smoothness.PercentDroppedFrames.SlowerThread.PinchZoom": {
		unit:      "percent",
		direction: perf.SmallerIsBetter,
		uma:       true,
	},
	"Compositing.Display.DrawToSwapUs": {
		unit:      "us",
		direction: perf.SmallerIsBetter,
		uma:       true,
	},
	"total_power": {
		unit:      "joules",
		direction: perf.SmallerIsBetter,
		uma:       false,
	},
	"gpu_power": {
		unit:      "joules",
		direction: perf.SmallerIsBetter,
		uma:       false,
	},
	"nongpu_power": {
		unit:      "joules",
		direction: perf.SmallerIsBetter,
		uma:       false,
	},
	"cpu_power": {
		unit:      "joules",
		direction: perf.SmallerIsBetter,
		uma:       false,
	},
	"dram_power": {
		unit:      "joules",
		direction: perf.SmallerIsBetter,
		uma:       false,
	},
	"trace_percent_dropped": {
		unit:      "percent",
		direction: perf.SmallerIsBetter,
		uma:       false,
	},
	"trace_fps": {
		unit:      "count",
		direction: perf.BiggerIsBetter,
		uma:       false,
	},
	"trace_num_frames": {
		unit:      "count",
		direction: perf.BiggerIsBetter,
		uma:       false,
	},
}

// These are the default categories for 'UI Rendering' in chrome://tracing plus 'exo' and 'wayland'.
// var tracingCategories = []string{"benchmark", "cc", "exo", "gpu", "input", "toplevel", "ui", "views", "viz", "wayland"}

type statType string

const (
	meanStat  = "mean"
	valueStat = "value"
)

type statBucketKey struct {
	metric string
	stat   statType
	crt    lacros.ChromeType
}

type metricsRecorder struct {
	buckets   map[statBucketKey][]float64
	metricMap map[string]metricInfo
}

func (m *metricsRecorder) record(ctx context.Context, invoc *testInvocation, minfo metricInfo, key statBucketKey, value float64) error {
	name := fmt.Sprintf("%s.%s.%s.%s", invoc.page.name, key.metric, string(key.stat), string(key.crt))
	testing.ContextLog(ctx, name, ": ", value, " ", minfo.unit)

	invoc.pv.Set(perf.Metric{
		Name:      name,
		Unit:      minfo.unit,
		Direction: minfo.direction,
	}, value)
	m.buckets[key] = append(m.buckets[key], value)
	m.metricMap[key.metric] = minfo
	return nil
}

func (m *metricsRecorder) recordHistogram(ctx context.Context, invoc *testInvocation, h *metrics.Histogram) error {
	// Ignore empty histograms. It's hard to define what the mean should be in this case.
	if h.TotalCount() == 0 {
		return nil
	}

	mean, err := h.Mean()
	if err != nil {
		return errors.Wrapf(err, "failed to get mean for histogram: %s", h.Name)
	}

	key := statBucketKey{
		metric: h.Name,
		stat:   meanStat,
		crt:    invoc.crt,
	}

	minfo, ok := metricMap[key.metric]
	if !ok {
		return errors.Errorf("failed to lookup metric info: %s", key.metric)
	}

	testing.ContextLog(ctx, h)

	return m.record(ctx, invoc, minfo, key, mean)
}

func (m *metricsRecorder) recordValue(ctx context.Context, invoc *testInvocation, name string, value float64) error {
	key := statBucketKey{
		metric: name,
		stat:   valueStat,
		crt:    invoc.crt,
	}

	minfo, ok := metricMap[key.metric]
	if !ok {
		return errors.Errorf("failed to lookup metric info: %s", key.metric)
	}

	return m.record(ctx, invoc, minfo, key, value)
}

func (m *metricsRecorder) recordMetric(ctx context.Context, invoc *testInvocation, metric perf.Metric, value float64) error {
	key := statBucketKey{
		metric: metric.Name,
		stat:   valueStat,
		crt:    invoc.crt,
	}

	minfo := metricInfo{
		unit:      metric.Unit,
		direction: metric.Direction,
		uma:       false,
	}

	return m.record(ctx, invoc, minfo, key, value)
}

func (m *metricsRecorder) computeStatistics(ctx context.Context, pv *perf.Values) error {
	// Collect means and standard deviations for each bucket. Each bucket contains results from several different pages.
	// We define the population as the set of all pages (another option would be to define the population as the
	// metric itself). For histograms (meanStat), we take a single sample which contains the means for each page.
	// For single values (valueStat), we take as single sample which just consists of those values.
	// We estimate the following quantities:
	// page_mean:
	//   Meaning: The mean for all pages. (e.g. mean of histogram means)
	//   Estimator: sample mean
	// page_stddev:
	//   Meaning: Variance over all pages. (e.g. variance of histogram means)
	//   Estimator: unbiased sample variance
	// N.B. we report standard deviation not variance so even though we use Bessel's correction the standard deviation
	// is still biased.
	// TODO: Consider extending this to also provide data where the population is the metric itself.
	//   e.g. metric_stddev, metric_mean - statistics on the metric overall not per-page.
	var logs []string
	for k, bucket := range m.buckets {
		minfo, ok := m.metricMap[k.metric]
		if !ok {
			return errors.Errorf("failed to lookup metric info: %s", k.metric)
		}

		var sum float64
		for _, value := range bucket {
			sum += value
		}
		n := float64(len(bucket))
		mean := sum / n
		var variance float64
		for _, value := range bucket {
			variance += (value - mean) * (value - mean)
		}
		variance /= float64(len(bucket) - 1) // Bessel's correction.
		stddev := math.Sqrt(variance)

		m := perf.Metric{
			Name:      fmt.Sprintf("all.%s.%s.%s", k.metric, "page_mean", string(k.crt)),
			Unit:      minfo.unit,
			Direction: minfo.direction,
		}
		s := perf.Metric{
			Name:      fmt.Sprintf("all.%s.%s.%s", k.metric, "page_stddev", string(k.crt)),
			Unit:      minfo.unit,
			Direction: perf.SmallerIsBetter, // In general, it's better if standard deviation is less.
		}
		logs = append(logs, fmt.Sprint(m.Name, ": ", mean, " ", m.Unit), fmt.Sprint(s.Name, ": ", stddev, " ", s.Unit))
		pv.Set(m, mean)

		// Standard deviation can be NaN if there weren't enough points to properly calculate it,
		// including Bessel's correction. Don't report it in this case.
		if !math.IsNaN(stddev) && !math.IsInf(stddev, 0) {
			pv.Set(s, stddev)
		}
	}

	// Print logs in order.
	sort.Strings(logs)
	for _, log := range logs {
		testing.ContextLog(ctx, log)
	}
	return nil
}

type traceable interface {
	StartTracing(ctx context.Context, categories []string, opts ...cdputil.TraceOption) error
	StopTracing(ctx context.Context) (*trace.Trace, error)
}

func computeTraceStats(tr *trace.Trace) (*results, error) {
	fs, err := newFrameStats(tr)
	if err != nil {
		return nil, err
	}
	res, err := fs.computeProportionDroppedFrames()
	if err != nil {
		return nil, err
	}
	return res, nil
}

// shouldOutputTraceStats determines if we should output custom metrics based on traces.
// TODO(crbug.com/1140906): We only output custom metrics based on traces on eve.
// This is because it was taking too long on other boards, and we only need the
// custom metrics on a single board to check if noise in other metrics is
// a metric problem or an inherent issue. This code is temporary and will be
// removed in the future. It is not recommended to look at lsb-release to
// change test behaviour.
func shouldOutputTraceStats() (bool, error) {
	k, err := lsbrelease.Load()
	if err != nil {
		return false, err
	}

	board, ok := k[lsbrelease.Board]
	if !ok {
		return false, errors.New("could not look up board in lsb-release")
	}

	return board == "eve", nil
}

func runHistogram(ctx context.Context, tconn *chrome.TestConn, traceConfigPath string, tracer traceable,
	invoc *testInvocation, perfFn func(ctx context.Context) error) error {
	if s, err := os.Stat(invoc.traceDir); err != nil || !s.IsDir() {
		return errors.Wrap(err, "given trace directory does not appear to be a directory")
	}

	var keys []string
	for k, v := range metricMap {
		if v.uma {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)

	thermal := power.NewSysfsThermalMetrics()
	thermal.Setup(ctx, "") // No prefix, we use our own naming scheme.

	rapl, err := power.NewRAPLSnapshot()
	if err != nil {
		return errors.Wrap(err, "failed to get RAPL snapshot")
	}

	histograms, err := metrics.Run(ctx, tconn, perfFn, keys...)
	if err != nil {
		return errors.Wrap(err, "failed to get histograms")
	}

	// Collect temperature first in case it decreases after the test finishes.
	temps, err := thermal.SnapshotValues(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get temperature data")
	}

	// `rapl` could be nil when not supported.
	var raplv *power.RAPLValues
	if rapl != nil {
		rd, err := rapl.DiffWithCurrentRAPL()
		if err != nil {
			return errors.Wrap(err, "failed to compute RAPL diffs")
		}
		raplv = rd
	}

	cmd := testexec.CommandContext(ctx, "perfetto", "-c", traceConfigPath, "--txt", "-o", "-")
	out, err := cmd.Output()
	if err != nil {
		return errors.Wrap(err, "failed to run the tracing session")
	}
	tr := &trace.Trace{}
	if err := tr.XXX_Unmarshal(out); err != nil {
		return errors.Wrap(err, "failed to unmarshal trace result")
	}

	filename := fmt.Sprintf("%s-%s-trace.data.gz", string(invoc.crt), invoc.page.name)
	filename = filepath.Join(invoc.traceDir, filename)
	if err := chrome.SaveTraceToFile(ctx, tr, filename); err != nil {
		return err
	}

	shouldOutput, err := shouldOutputTraceStats()
	if err != nil {
		return err
	}

	if shouldOutput {
		res, err := computeTraceStats(tr)
		if err != nil {
			return err
		}
		if err := invoc.metrics.recordValue(ctx, invoc, "trace_percent_dropped", res.proportionDropped*100.0); err != nil {
			return err
		}
		if err := invoc.metrics.recordValue(ctx, invoc, "trace_fps", res.fps); err != nil {
			return err
		}
		if err := invoc.metrics.recordValue(ctx, invoc, "trace_num_frames", float64(res.numFrames)); err != nil {
			return err
		}
	}

	// Store metrics in the form: Scenario.PageSet.UMA metric name.statistic.{chromeos, lacros}.
	// For example, maximized.Compositing.Display.DrawToSwapUs.mean.chromeos. In crosbolt, for each
	// scenario (e.g. three-dot menu), we can then easily compare between chromeos and lacros
	// for the same metric, in the same scenario.
	for _, h := range histograms {
		if err := invoc.metrics.recordHistogram(ctx, invoc, h); err != nil {
			return err
		}
	}

	for metric, value := range temps {
		if err := invoc.metrics.recordMetric(ctx, invoc, metric, value); err != nil {
			return err
		}
	}

	if raplv != nil {
		nongpuPower := raplv.Total() - raplv.Uncore()
		if err := invoc.metrics.recordValue(ctx, invoc, "total_power", raplv.Total()); err != nil {
			return err
		}
		if err := invoc.metrics.recordValue(ctx, invoc, "nongpu_power", nongpuPower); err != nil {
			return err
		}
		if err := invoc.metrics.recordValue(ctx, invoc, "cpu_power", raplv.Core()); err != nil {
			return err
		}
		if err := invoc.metrics.recordValue(ctx, invoc, "dram_power", raplv.DRAM()); err != nil {
			return err
		}
		if err := invoc.metrics.recordValue(ctx, invoc, "gpu_power", raplv.Uncore()); err != nil {
			return err
		}
	}
	return nil
}
