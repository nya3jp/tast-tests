// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package graphics contains graphics-related utility functions for local tests.
package graphics

import (
	"bytes"
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
	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// collectPerformanceCounters gathers the use time for each of a given set of
// Performance Monitoring Units (PMUs), if available, providing them in a map
// indexed by the name of the associated Command Streamer (CS): RCS for Renderer/3D,
// VCS for the Fixed-Function Video (decoding and encoding), and VECS for Video
// Enhancement CS. The resulting map also provides an accurate hardware elapsed
// time counter.
// If the hardware/kernel doesn't provide PMU event monitoring, the returned
// counters will be nil.
func collectPerformanceCounters(ctx context.Context, interval time.Duration) (counters map[string]time.Duration, megaPeriods int64, err error) {
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
	}

	var eventsToCollect []string
	for _, perfCounter := range perfCounters {
		if _, err := os.Stat(perfCounter.filePath); err == nil {
			eventsToCollect = append(eventsToCollect, perfCounter.eventName)
		} else {
			testing.ContextLogf(ctx, "Could not find %s perf event file (error: %v)", perfCounter.filePath, err)
		}
	}

	if len(eventsToCollect) == 0 {
		return nil, 0, nil
	}

	// Run the command e.g. `perf stat -e i915/vcs0-busy/ -- sleep 2`
	cmd := testexec.CommandContext(ctx,
		"/usr/bin/perf", "stat", "-e", strings.Join(eventsToCollect, ","), "--", "sleep",
		strconv.FormatInt(int64(interval/time.Second), 10))
	var perfOutput bytes.Buffer
	cmd.Stderr = &perfOutput
	if err := cmd.Run(); err != nil {
		return nil, 0, errors.Wrap(err, "error while measuring perf counters")
	}

	// A sample multiple counter output perfOutput could be e.g.:
	// Performance counter stats for 'system wide':
	//
	//             8215 M    i915/actual-frequency/
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

	for _, line := range strings.Split(perfOutput.String(), "\n") {
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
				if err := ioutil.WriteFile(filepath.Join(dir, outFile), perfOutput.Bytes(), 0644); err != nil {
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
	for _, line := range strings.Split(perfOutput.String(), "\n") {
		submatch := regexpCycles.FindStringSubmatch(line)
		if submatch == nil {
			continue
		}
		if megaPeriods, err = strconv.ParseInt(submatch[1], 10, 64); err != nil {
			return nil, 0, errors.Wrapf(err, "error parsing perf output (%s)", perfOutput.String())
		}
		break
	}

	return counters, megaPeriods, nil
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
	counters, megaPeriods, err := collectPerformanceCounters(ctx, t)
	if err != nil {
		return errors.Wrap(err, "error collecting graphics performance counters")
	}
	if counters == nil {
		return nil
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

	return nil
}
