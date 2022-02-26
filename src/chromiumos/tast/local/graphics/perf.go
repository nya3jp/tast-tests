// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package graphics contains graphics-related utility functions for local tests.
package graphics

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/testing"
)

// collectGPUPerformanceCounters gathers the use time for each of a given set of
// Performance Monitoring Units (PMUs), if available, providing them in a map
// indexed by the name of the associated Command Streamer (CS): RCS for Renderer/3D,
// VCS for the Fixed-Function Video (decoding and encoding), and VECS for Video
// Enhancement CS. The resulting map also provides an accurate hardware elapsed
// time counter, and the amount of time the GPU spent in sleep mode (RC6 mode).
// If the hardware/kernel doesn't provide PMU event monitoring, the returned
// counters will be nil.
func collectGPUPerformanceCounters(ctx context.Context, interval time.Duration) (counters map[string]time.Duration, megaPeriods int64, err error) {
	var perfCounters = []struct {
		filePath   string
		eventName  string
		outputName string
	}{
		// "actual-frequency" is NOT a frequency, but an accumulation of cycles.
		{"/sys/devices/i915/events/actual-frequency", "i915/actual-frequency/", "megaperiod"},
		{"/sys/devices/i915/events/rcs0-busy", "i915/rcs0-busy/", "rcs"},
		{"/sys/devices/i915/events/vcs0-busy", "i915/vcs0-busy/", "vcs"},
		{"/sys/devices/i915/events/vecs0-busy", "i915/vecs0-busy/", "vecs"},
		{"/sys/devices/i915/events/rc6-residency", "i915/rc6-residency/", "rc6"},
	}

	var eventsToCollect []string
	for _, perfCounter := range perfCounters {
		if _, err := os.Stat(perfCounter.filePath); err == nil {
			eventsToCollect = append(eventsToCollect, perfCounter.eventName)
		}
	}

	if len(eventsToCollect) == 0 {
		return nil, 0, nil
	}

	// Run the command e.g. `perf stat -e i915/vcs0-busy/ -- sleep 2`
	cmd := testexec.CommandContext(ctx,
		"/usr/bin/perf", "stat", "-e", strings.Join(eventsToCollect, ","), "--", "sleep",
		strconv.FormatInt(int64(interval/time.Second), 10))
	_, stderr, err := cmd.SeparatedOutput()
	if err != nil {
		return nil, 0, errors.Wrap(err, "error while measuring perf counters")
	}
	perfOutput := string(stderr)

	// A sample multiple counter output perfOutput could be e.g.:
	// Performance counter stats for 'system wide':
	//
	//             8215 M    i915/actual-frequency/
	//       2001181274 ns   i915/rc6-residency/
	//      17188646693 ns   i915/rcs0-busy/
	//      11937916640 ns   i915/vcs0-busy/
	//      12894570939 ns   i915/vecs0-busy/
	//
	//      25.001367738 seconds time elapsed
	counters = make(map[string]time.Duration)

	regexps := make(map[string]*regexp.Regexp)
	for _, perfCounter := range perfCounters {
		regexps[perfCounter.outputName] = regexp.MustCompile(`([0-9]+ ns)\s*` + regexp.QuoteMeta(perfCounter.eventName))
	}
	// Add and extra regexp for the overall time elapsed.
	regexps["total"] = regexp.MustCompile("([0-9]+[.][0-9]+ s)econds time elapsed")

	perfLines := strings.Split(perfOutput, "\n")
	for _, line := range perfLines {
		for name, r := range regexps {
			submatch := r.FindStringSubmatch(line)
			if submatch == nil {
				continue
			}
			// ParseDuration() cannot parse whitespaces in the input string.
			counters[name], err = time.ParseDuration(strings.Replace(string(submatch[1]), " ", "", -1))
			if err != nil {
				dir, ok := testing.ContextOutDir(ctx)
				if !ok {
					return nil, 0, errors.New("failed to retrieve output directory")
				}
				const outFile = "perf_stat_output.log"
				if err := ioutil.WriteFile(filepath.Join(dir, outFile), stderr, 0644); err != nil {
					testing.ContextLogf(ctx, "Failed to dump perf output to %s: %v", outFile, err)
				}
				return nil, 0, errors.Wrapf(err, "error parsing perf output, see %s if present", outFile)
			}
		}
	}

	// Run and extra regexp pass for the GPU actual-frequency. This information is
	// provided as a number-of-accumulated mega cycles over the total time, see
	// https://patchwork.freedesktop.org/patch/339667/.
	regexpCycles := regexp.MustCompile(`([0-9]+)\s*M\s*` + "i915/actual-frequency/")
	for _, line := range perfLines {
		submatch := regexpCycles.FindStringSubmatch(line)
		if submatch == nil {
			continue
		}
		if megaPeriods, err = strconv.ParseInt(submatch[1], 10, 64); err != nil {
			return nil, 0, errors.Wrapf(err, "error parsing perf output (%s)", perfOutput)
		}
		break
	}

	return counters, megaPeriods, nil
}

// AMD does not use the command line tool perf to report GPU utilization, but it
// provides a sysfs file that can be read with the GPU utilization as a
// percent, see the kernel amdgpu_pm.c file. This function reads the values
// during interval and returns it in the counters' "rcs" and "total", imitating
// what perf and collectGPUPerformanceCounters() would do.
// TODO(b/181352867): Remove this method when AMD implements perf counters.
func collectAMDBusyCounter(ctx context.Context, interval time.Duration) (counters map[string]time.Duration, megaPeriods int64, err error) {

	const amdBusyGPUFile = "/sys/class/drm/card0/device/gpu_busy_percent"
	if _, err = os.Stat(amdBusyGPUFile); err != nil {
		return nil, 0, nil
	}

	accuBusy := int64(0)
	const samplePeriod = 10 * time.Millisecond
	numSamples := int(interval / samplePeriod)
	for i := 0; i < numSamples; i++ {
		if err := testing.Sleep(ctx, samplePeriod); err != nil {
			return nil, 0, errors.Wrap(err, "error sleeping")
		}

		v, err := ioutil.ReadFile(amdBusyGPUFile)
		if err != nil {
			return nil, 0, errors.Wrapf(err, "error reading from %s", amdBusyGPUFile)
		}
		busy, err := strconv.ParseInt(strings.TrimSuffix(string(v), "\n"), 10, 64)
		if err != nil {
			return nil, 0, errors.Wrapf(err, "error converting %s", string(v))
		}
		accuBusy += busy
	}

	counters = make(map[string]time.Duration)
	// Divide accuBusy by hundred to remove the percentage.
	counters["rcs"] = time.Duration(float64(accuBusy) / 100.0 * float64(time.Second))
	counters["total"] = time.Duration(numSamples * int(time.Second))
	return counters, 0, nil
}

// collectPackagePerformanceCounters gathers the amount of cycles the Package
// was in each of a given C-States, providing also the total amount of cycles
// elapsed. If the hardware/kernel doesn't provide this type of event monitoring
// or if the reference TSC (Time Stamp Counter) is not available, the returned
// counter map will be nil, but this is not considered an error.
func collectPackagePerformanceCounters(ctx context.Context, interval time.Duration) (counters map[string]int64, err error) {
	var perfCounters = []struct {
		filePath   string
		eventName  string
		outputName string
		necessary  bool
	}{
		{"/sys/devices/cstate_pkg/events/c1-residency", "cstate_pkg/c1-residency/", "c1", false},
		{"/sys/devices/cstate_pkg/events/c2-residency", "cstate_pkg/c2-residency/", "c2", false},
		{"/sys/devices/cstate_pkg/events/c3-residency", "cstate_pkg/c3-residency/", "c3", false},
		{"/sys/devices/cstate_pkg/events/c4-residency", "cstate_pkg/c4-residency/", "c4", false},
		{"/sys/devices/cstate_pkg/events/c5-residency", "cstate_pkg/c5-residency/", "c5", false},
		{"/sys/devices/cstate_pkg/events/c6-residency", "cstate_pkg/c6-residency/", "c6", false},
		{"/sys/devices/cstate_pkg/events/c7-residency", "cstate_pkg/c7-residency/", "c7", false},
		{"/sys/devices/cstate_pkg/events/c8-residency", "cstate_pkg/c8-residency/", "c8", false},
		{"/sys/devices/cstate_pkg/events/c9-residency", "cstate_pkg/c9-residency/", "c9", false},
		{"/sys/devices/cstate_pkg/events/c10-residency", "cstate_pkg/c10-residency/", "c10", false},
		// TSC (Time StampCounter) is necessary to give dimension to all others.
		{"/sys/devices/msr/events/tsc", "msr/tsc/", "tsc", true},
	}

	var eventsToCollect []string
	for _, perfCounter := range perfCounters {
		if _, err := os.Stat(perfCounter.filePath); err == nil {
			eventsToCollect = append(eventsToCollect, perfCounter.eventName)
		} else if perfCounter.necessary {
			return nil, nil
		}
	}
	// The TSC event will be present.
	if len(eventsToCollect) <= 1 {
		return nil, nil
	}

	// Run the command e.g. `perf stat -C 1 -e msr/tsc/,cstate_pkg/c2-residency/ -- sleep 2`
	// Limit the collection to the first core ("-C 0") otherwise the results would
	// add the TSC counters in each CPU.
	cmd := testexec.CommandContext(ctx,
		"/usr/bin/perf", "stat", "-C", "0", "-e", strings.Join(eventsToCollect, ","), "--", "sleep",
		strconv.FormatInt(int64(interval/time.Second), 10))
	_, stderr, err := cmd.SeparatedOutput()
	if err != nil {
		return nil, errors.Wrap(err, "error while measuring perf counters")
	}
	perfOutput := string(stderr)

	// A sample multiple counter output perfOutput could be e.g.:
	// Performance counter stats for 'CPU(s) 0':
	//
	//        1017226860      cstate_pkg/c2-residency/
	//         596249316      cstate_pkg/c3-residency/
	//           7153458      cstate_pkg/c6-residency/
	//         870845664      cstate_pkg/c7-residency/
	//        3182648118      cstate_pkg/c8-residency/
	//                 0      cstate_pkg/c9-residency/
	//                 0      cstate_pkg/c10-residency/
	//        5994345394      msr/tsc/
	//
	//       2.001372100 seconds time elapsed
	counters = make(map[string]int64)

	regexps := make(map[string]*regexp.Regexp)
	for _, perfCounter := range perfCounters {
		regexps[perfCounter.outputName] = regexp.MustCompile(`([0-9]+)\s*` + regexp.QuoteMeta(perfCounter.eventName))
	}

	perfLines := strings.Split(perfOutput, "\n")
	for _, line := range perfLines {
		for name, r := range regexps {
			submatch := r.FindStringSubmatch(line)
			if submatch == nil {
				continue
			}
			counters[name], err = strconv.ParseInt(submatch[1], 10, 64)
			if err != nil {
				return nil, errors.Wrap(err, "error parsing perf output")
			}
		}
	}
	return counters, nil
}

func reportMetric(name, unit string, value float64, direction perf.Direction, p *perf.Values) {
	p.Set(perf.Metric{
		Name:      name,
		Unit:      unit,
		Direction: direction,
	}, value)
}

func parseAndReportCounter(ctx context.Context, counters map[string]time.Duration, counterName string, p *perf.Values) {
	if counter, ok := counters[counterName]; ok && counter.Seconds() != 0 {
		usage := 100 * counter.Seconds() / counters["total"].Seconds()
		testing.ContextLogf(ctx, "%s: %f%%", counterName, usage)
		reportMetric(fmt.Sprintf("%s_usage", counterName), "percent", usage, perf.SmallerIsBetter, p)
	}
}

// MeasureGPUCounters measures GPU usage for a period of time t into p.
func MeasureGPUCounters(ctx context.Context, t time.Duration, p *perf.Values) error {
	testing.ContextLog(ctx, "Measuring GPU usage for ", t)
	counters, megaPeriods, err := collectGPUPerformanceCounters(ctx, t)
	if err != nil {
		return errors.Wrap(err, "error collecting graphics performance counters")
	}
	if counters == nil {
		// Give a chance to AMD-specific counter readings.
		counters, megaPeriods, err = collectAMDBusyCounter(ctx, t)
		if counters == nil {
			return nil
		}
	}
	if counters["total"].Milliseconds() == 0 {
		return errors.New("total elapsed time counter is zero")
	}

	if megaPeriods != 0 {
		frequencyMHz := float64(megaPeriods) / counters["total"].Seconds()
		testing.ContextLogf(ctx, "Average frequency: %fMHz", frequencyMHz)
		reportMetric("frequency", "MHz", frequencyMHz, perf.SmallerIsBetter, p)
	}
	parseAndReportCounter(ctx, counters, "rcs", p)
	parseAndReportCounter(ctx, counters, "vcs", p)
	parseAndReportCounter(ctx, counters, "vecs", p)
	parseAndReportCounter(ctx, counters, "rc6", p)

	return nil
}

// MeasurePackageCStateCounters measures the Package C-State residencies for a
// period of time t into p. Package C-States counters report how many cycles the
// package was in a given state, with a larger index corresponding to deeper
// sleep states. The total elapsed cycles is available under the first CPU's TSC
// (Time Stamp Counter) register. The "active " state, which would be c0, is the
// remaining cycles. See e.g. https://en.wikichip.org/wiki/acpi/c-states.
func MeasurePackageCStateCounters(ctx context.Context, t time.Duration, p *perf.Values) error {
	testing.ContextLog(ctx, "Measuring Package C-State residency for ", t)
	counters, err := collectPackagePerformanceCounters(ctx, t)
	if err != nil {
		return errors.Wrap(err, "error collecting C-State performance counters")
	}
	if counters == nil {
		return nil
	}
	// "tsc" (Time Stamp Counter register) is a special entry because is the
	// amount of cycles elapsed and gives a reference for all the C-States
	// counters. We don't report it.
	var accu int64
	for name, value := range counters {
		if name == "tsc" {
			continue
		}
		accu += value

		cStatePercent := 100 * float64(value) / float64(counters["tsc"])
		testing.ContextLogf(ctx, "%s: %f%%", name, cStatePercent)
		reportMetric(name, "percent", cStatePercent, perf.BiggerIsBetter, p)
	}
	// The amount of cycles not in any sleep state is the active state, "c0".
	c0Percent := 100 * float64(counters["tsc"]-accu) / float64(counters["tsc"])
	testing.ContextLogf(ctx, "c0: %f%%", c0Percent)
	reportMetric("c0", "percent", c0Percent, perf.SmallerIsBetter, p)

	return nil
}

// UpdatePerfMetricFromHistogram takes a snapshot of histogramName and
// calculates the average difference with initHistogram. The result is then
// logged to perfValues with metricName.
func UpdatePerfMetricFromHistogram(ctx context.Context, tconn *chrome.TestConn, histogramName string, initHistogram *metrics.Histogram, perfValues *perf.Values, metricName string) error {
	laterHistogram, err := metrics.GetHistogram(ctx, tconn, histogramName)
	if err != nil {
		return errors.Wrap(err, "failed to get later histogram")
	}
	histogramDiff, err := laterHistogram.Diff(initHistogram)
	if err != nil {
		return errors.Wrap(err, "failed diffing histograms")
	}
	// Some devices don't have hardware decode acceleration, so the histogram diff
	// will be empty, this is not an error condition.
	if len(histogramDiff.Buckets) > 0 {
		decodeMetric := perf.Metric{
			Name:      metricName,
			Unit:      "ms",
			Direction: perf.SmallerIsBetter,
			Multiple:  true,
		}

		numHistogramSamples := float64(histogramDiff.TotalCount())
		var average float64
		// Walk the buckets of histogramDiff, append the central value of the
		// histogram bucket as many times as bucket entries to perfValues, and
		// calculate the average on the fly for debug printout purposes. This average
		// is a discrete approximation to the statistical average of the samples
		// underlying the histogramDiff histograms.
		for _, bucket := range histogramDiff.Buckets {
			bucketMidpoint := float64(bucket.Max+bucket.Min) / 2.0
			for i := 0; i < int(bucket.Count); i++ {
				perfValues.Append(decodeMetric, bucketMidpoint)
			}
			average += bucketMidpoint * float64(bucket.Count) / numHistogramSamples
		}
		testing.ContextLog(ctx, histogramName, ": histogram:", histogramDiff.String(), "; average: ", average)
	}
	return nil
}

// MeasureCPUUsageAndPower measures CPU usage and power consumption (if
// supported) for measurement time into p. If the optional stabilization
// duration is specified, the test will sleep for such amount of time before
// measuring.
//
// Optionally, clients of this method might like to call cpu.SetUpBenchmark()
// and cpu.WaitUntilIdle() before starting the actual test logic, to set up and
// wait for the CPU usage to stabilize to a low level. Example:
//
//  import "chromiumos/tast/local/media/cpu"
//
//  cleanUpBenchmark, err := cpu.SetUpBenchmark(ctx)
//  if err != nil {
//    return errors.Wrap(err, "failed to set up CPU benchmark")
//  }
//  defer cleanUpBenchmark(ctx)
//
//  if err := cpu.WaitUntilIdle(ctx); err != nil {
//    return errors.Wrap(err, "failed waiting for CPU to become idle")
//  }
func MeasureCPUUsageAndPower(ctx context.Context, stabilization, measurement time.Duration, p *perf.Values) error {
	if stabilization != 0 {
		testing.ContextLogf(ctx, "Sleeping %v to wait for CPU usage to stabilize", stabilization)
		if err := testing.Sleep(ctx, stabilization); err != nil {
			return err
		}
	}

	testing.ContextLog(ctx, "Measuring CPU usage and Power for ", measurement)
	measurements, err := cpu.MeasureUsage(ctx, measurement)
	if err != nil {
		return errors.Wrap(err, "failed to measure CPU usage and power consumption")
	}

	cpuUsage := measurements["cpu"]
	testing.ContextLogf(ctx, "CPU usage: %f%%", cpuUsage)
	p.Set(perf.Metric{
		Name:      "cpu_usage",
		Unit:      "percent",
		Direction: perf.SmallerIsBetter,
	}, cpuUsage)

	if power, ok := measurements["power"]; ok {
		testing.ContextLogf(ctx, "Avg pkg power usage: %fW", power)
		p.Set(perf.Metric{
			Name:      "pkg_power_usage",
			Unit:      "W",
			Direction: perf.SmallerIsBetter,
		}, power)
	}
	return nil
}
