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
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/chromeproc"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/local/power"
	"chromiumos/tast/local/power/setup"
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

// collectAMDBusyCounter gathers AMD GPU utilization stats.
// AMD does not use the command line tool perf to report GPU utilization, but it
// provides a sysfs file that can be read with the GPU utilization as a
// percent, see the kernel amdgpu_pm.c file. This function reads the values
// during interval and returns it in the counters' "rcs" and "total", imitating
// what perf and collectGPUPerformanceCounters() would do.
// TODO(b/181352867): Remove this method when AMD implements perf counters.
func collectAMDBusyCounter(ctx context.Context, interval time.Duration) (counters map[string]time.Duration, megaPeriods int64, err error) {
	// Check if context deadline allows collecting data for the given interval.
	deadLine, ok := ctx.Deadline()
	if ok {
		contextInteval := deadLine.Sub(time.Now())
		if contextInteval < interval {
			return nil, 0, errors.Errorf("context interval %v is less than the collecting interval %v", contextInteval, interval)
		}
	}

	const amdBusyGPUFile = "/sys/class/drm/card0/device/gpu_busy_percent"
	if _, err = os.Stat(amdBusyGPUFile); err != nil {
		return nil, 0, nil
	}

	accuBusy := int64(0)
	const samplePeriod = 10 * time.Millisecond
	numSamples := int(interval / samplePeriod)
	actualSamples := 0
	for i := 0; i < numSamples; i++ {
		// Check if enough time is left for the next sampling cycle before reaching the ctx deadline.
		deadLine, ok := ctx.Deadline()
		if ok && deadLine.Sub(time.Now()) <= samplePeriod {
			testing.ContextLog(ctx, "Complete AMD gpu counter collecting because context deadline is about to reach")
			break
		}
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
		actualSamples++
	}

	counters = make(map[string]time.Duration)
	// Divide accuBusy by hundred to remove the percentage.
	counters["rcs"] = time.Duration(float64(accuBusy) / 100.0 * float64(samplePeriod))
	counters["total"] = time.Duration(actualSamples * int(samplePeriod))
	return counters, 0, nil
}

// collectMaliPerformanceCounters gathers the amount of GPU cycles the GPU
// was active. Source code for the mali_stats program this function wraps can
// be found in "platform/drm-tests/mali_stats.c". Mali_stats automatically
// take into account up/down clocking of the GPU by dividing by its maximum
// frequency.
func collectMaliPerformanceCounters(ctx context.Context, interval time.Duration) (counters map[string]time.Duration, megaPeriods int64, err error) {
	const maliFile = "/dev/mali0"
	if _, err = os.Stat(maliFile); err != nil {
		// This isn't a Mali GPU, so return nil with no error.
		return nil, 0, nil
	}

	accuBusy := float64(0.0)
	const samplePeriod = time.Second
	numSamples := int(interval / samplePeriod)
	for i := 0; i < numSamples; i++ {
		maliStatsCmd := exec.Command("mali_stats", "-u", "10000")
		var out bytes.Buffer
		maliStatsCmd.Stdout = &out

		err := maliStatsCmd.Run()
		if err != nil {
			return nil, 0, errors.Wrap(err, "error running mali_stats")
		}

		percentUsage, err := strconv.ParseFloat(out.String()[:strings.Index(out.String(), "%")], 64)

		if err != nil {
			return nil, 0, errors.Wrap(err, "error parsing mali_stats output")
		}

		if percentUsage > 100.0 || percentUsage < 0.0 {
			percentUsage = 100.0
		}

		accuBusy += percentUsage
	}

	counters = make(map[string]time.Duration)
	counters["rcs"] = time.Duration(accuBusy / 100.0 * float64(time.Second))
	counters["total"] = time.Duration(float64(numSamples) * float64(time.Second))

	return counters, 0, nil
}

// parseTransStatFile parses the content of a given transStatFileName which must
// be a "trans_stat" sysfs file ([1], see also the body of the function). If
// successful, freqs will contain the frequencies and the sampled elapsed
// times in every frequency, otherwise err will be filled in.
// [1]https://www.kernel.org/doc/Documentation/ABI/testing/sysfs-class-devfreq
func parseTransStatFile(transStatFileName string) (freqs map[float64]time.Duration, err error) {
	out, err := ioutil.ReadFile(transStatFileName)
	if err != nil {
		return nil, errors.Wrap(err, "problem reading DevFreq trans_stat file")
	}
	// Sample out:
	//      From  :   To
	//            : 253500000 299000000 396500000 455000000 494000000 598000000   time(ms)
	//   253500000:         0         0         0         0         0         1  15005129
	//   299000000:         0         0         0         0         0         0         0
	//   396500000:         0         0         0         0         0         0         0
	//   455000000:         0         0         0         0         0         0         0
	//   494000000:         0         0         0         0         0         1       304
	// * 598000000:         0         0         0         0         1         0     10972
	// Total transition : 3
	//
	// (Note that the frequencies can come in any order). We need to extract the
	// first columns' numbers (frequencies in Hertz) and the last column
	// (time in each frequency level in ms).
	freqs = make(map[float64]time.Duration)

	re := regexp.MustCompile("([0-9]+):.* ([0-9]+)$")
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		submatch := re.FindStringSubmatch(line)
		if submatch == nil || len(submatch) != 3 {
			continue
		}

		t, err := time.ParseDuration(string(submatch[2]) + "ms")
		if err != nil {
			return nil, errors.Wrap(err, "problem parsing string")
		}

		var f float64
		if f, err = strconv.ParseFloat(submatch[1], 64); err != nil {
			return nil, errors.Wrap(err, "problem converting float value")
		}

		freqs[f] = t
	}
	return freqs, nil
}

// collectDevFreqCounters gathers GPU frequency and idle state stats. For that
// it reads the contents of the appropriate "trans_stat" sysfs file (see [1]),
// before and after the given interval. Said file contains information on how
// much time the GPU was operating at a certain frequency, with time not logged
// being in "idle" state. This "idle" count is returned in the counters' "rc6"
// entry, with the average frequency in megaPeriods, both imitating what
// collectGPUPerformanceCounters() does.
// [1] https://www.kernel.org/doc/Documentation/ABI/testing/sysfs-class-devfreq
func collectDevFreqCounters(ctx context.Context, interval time.Duration) (counters map[string]time.Duration, megaPeriods int64, err error) {
	files, err := ioutil.ReadDir("/sys/class/devfreq/")
	if os.IsNotExist(err) {
		// If the kernel doesn't provide devfreq it's not an error.
		return nil, 0, nil
	}
	if err != nil {
		return nil, 0, errors.Wrap(err, "couldn't read DevFreq folder")
	}

	transStatFileName := ""
	r := regexp.MustCompile(".*(gpu|mali)$")
	for _, file := range files {
		if match := r.FindStringSubmatch(file.Name()); match == nil {
			continue
		}

		if _, err = os.Stat("/sys/class/devfreq/" + file.Name() + "/trans_stat"); err != nil {
			return nil, 0, errors.Wrap(err, "couldn't find DevFreq trans_stat file")
		}
		transStatFileName = "/sys/class/devfreq/" + file.Name() + "/trans_stat"
	}
	if transStatFileName == "" {
		return nil, 0, nil
	}
	testing.ContextLogf(ctx, "GPU Frequency meas: using %s", transStatFileName)

	var freqsBefore map[float64]time.Duration
	if freqsBefore, err = parseTransStatFile(transStatFileName); err != nil {
		return nil, 0, errors.Wrap(err, "error parsing trans_stat file")
	}

	if err := testing.Sleep(ctx, interval); err != nil {
		return nil, 0, errors.Wrap(err, "error sleeping")
	}

	var freqsAfter map[float64]time.Duration
	if freqsAfter, err = parseTransStatFile(transStatFileName); err != nil {
		return nil, 0, errors.Wrap(err, "error parsing trans_stat file")
	}

	if len(freqsBefore) != len(freqsAfter) {
		return nil, 0, errors.New("different number of frequencies read before/after")
	}

	accuPeriods := 0.0
	var accuTime time.Duration
	for f := range freqsBefore {
		accuTime += freqsAfter[f] - freqsBefore[f]
		accuPeriods += f * (freqsAfter[f] - freqsBefore[f]).Seconds()
	}

	counters = make(map[string]time.Duration)
	counters["total"] = interval
	counters["rc6"] = interval - accuTime
	return counters, int64(accuPeriods / 1e6), nil
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

	type gpuTask struct {
		name        string
		counters    map[string]time.Duration
		megaPeriods int64
		err         error
		run         func(ctx context.Context, interval time.Duration) (map[string]time.Duration, int64, error)
	}

	// tasks defines the performance collecting tasks to run.
	// The tasks will be run in parallel and the result entries merged.
	tasks := []gpuTask{
		{name: "GPU", run: collectGPUPerformanceCounters},
		{name: "AMD", run: collectAMDBusyCounter},
		{name: "Mali", run: collectMaliPerformanceCounters},
		{name: "DevFreq", run: collectDevFreqCounters},
	}

	var wg sync.WaitGroup
	for i := range tasks {
		wg.Add(1)
		go func(task *gpuTask) {
			defer wg.Done()
			// Store returned values of measurement function into the tasks slice.
			task.counters, task.megaPeriods, task.err = task.run(ctx, t)
		}(&tasks[i])
	}
	wg.Wait()
	counters := make(map[string]time.Duration)
	var megaPeriods int64
	for _, task := range tasks {
		if task.err != nil {
			// Return at the first error.
			return errors.Wrapf(task.err, "error collecting %s performance counters", task.name)
		}
		// Merge any counters present. Several tasks could report values.
		if task.counters != nil {
			for k, v := range task.counters {
				counters[k] = v
			}
		}
		// Only one task should report megaPeriods, because only one counts the
		// average GPU frequency.
		if task.megaPeriods != 0 {
			megaPeriods = task.megaPeriods
		}
	}
	if len(counters) == 0 {
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

// MeasureSystemPowerConsumption samples the battery power consumption every so
// often during an interval t using sysfs [1], and reports its average over
// that time. To provide accurate readings, the battery needs to be configured
// to discharge, if that fails (perhaps due to low battery charge), this
// function returns nil (i.e.no error).
// [1] https://www.kernel.org/doc/Documentation/ABI/testing/sysfs-class-power
func MeasureSystemPowerConsumption(ctx context.Context, c *chrome.TestConn, t time.Duration, p *perf.Values) error {

	batteryDischarge := setup.TryBatteryDischarge(true, setup.DefaultDischargeThreshold)

	cleanup, err := setup.PowerTest(ctx, c, setup.PowerTestOptions{
		Battery:    batteryDischarge,
		NightLight: setup.DisableNightLight})
	if err != nil {
		// This is not really an error: sometimes powerd is down or lost and setting
		// up the power test fails. Just don't provide any metric.
		testing.ContextLog(ctx, "Skipping measurement, something went wrong during test set up: ", err)
		return nil
	}
	defer cleanup(ctx)

	// We don't use power.SysfsBatteryMetrics because we want to reject zero
	// readings below.
	battery, err := power.SysfsBatteryPath(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to find a battery")
	}

	accuPower := float64(0)
	numReadings := float64(0)
	const samplePeriod = 100 * time.Millisecond
	numSamples := int(t / samplePeriod)
	for i := 0; i < numSamples; i++ {
		// Check whether enough time is left for the next sampling cycle before reaching the ctx deadline.
		if deadLine, ok := ctx.Deadline(); ok && deadLine.Sub(time.Now()) <= samplePeriod {
			testing.ContextLog(ctx, "Finishing system power consumption measurement because context deadline is about to reach")
			break
		}

		if err := testing.Sleep(ctx, samplePeriod); err != nil {
			return errors.Wrap(err, "error sleeping")
		}

		power, err := power.ReadSystemPower(battery)
		if err != nil {
			return err
		}
		if power == 0.0 {
			continue
		}
		accuPower += power
		numReadings++
	}

	testing.ContextLogf(ctx, "Average system power consumption: %fW", accuPower/numReadings)
	reportMetric("system_power", "W", accuPower/numReadings, perf.SmallerIsBetter, p)

	return nil
}

// MeasureFdCount counts the average and peak number of open FDs by the GPU
// process(es) during playback. Polls every 1 seconds up until the duration
// given.
func MeasureFdCount(ctx context.Context, duration time.Duration, p *perf.Values) error {
	testing.ContextLog(ctx, "Measuring open file descriptors for ", duration)
	processes, err := chromeproc.GetGPUProcesses()
	if err != nil {
		return errors.Wrap(err, "failed to get gpu process")
	}
	if len(processes) == 0 {
		return errors.New("no processes found")
	}

	var peakFds, totalFds, iterations int32
	_ = testing.Poll(ctx, func(ctx context.Context) error {
		var fdCount int32
		for _, process := range processes {
			numFds, err := process.NumFDsWithContext(ctx)
			if err != nil {
				return errors.Wrap(err, "failed to get fds for process")
			}
			fdCount += numFds
		}

		if fdCount > peakFds {
			peakFds = fdCount
		}
		// TODO(b/215719663) Consider adding safeguards or switching to rolling
		// average to make sure |totalFds| doesn't hit integer overflow.
		totalFds += fdCount
		iterations++
		// Always return an error. We let the timeout handle the duration for which
		// the open FDs are checked.
		return errors.New("Still polling the open FDs")
	}, &testing.PollOptions{Timeout: duration, Interval: time.Second})

	reportMetric("peakOpenFds", "count", float64(peakFds), perf.SmallerIsBetter, p)
	reportMetric("averageOpenFds", "count", float64(totalFds)/float64(iterations), perf.SmallerIsBetter, p)
	return nil
}

// MeasureDRAMBandwidth measures average DRAM bandwidth consumption in bytes
// per second over the given duration.
func MeasureDRAMBandwidth(ctx context.Context, duration time.Duration, p *perf.Values) error {
	testing.ContextLog(ctx, "Measuring DRAM bandwidth usage for ", duration)

	mtkDramToolCmd := exec.Command("mtk_dram_tool", "-l", strconv.FormatInt(duration.Milliseconds(), 10))
	var out bytes.Buffer
	mtkDramToolCmd.Stdout = &out

	err := mtkDramToolCmd.Run()
	if err != nil {
		if strings.Contains(out.String(), "Error! Incompatible device!") {
			testing.ContextLog(ctx, "mtk_dram_tool not supported on this platform")
			return nil
		}

		return errors.Wrap(err, "failed to run mtk_dram_tool")
	}

	dramUsage, err := strconv.ParseInt(out.String()[:strings.Index(out.String(), ".")], 10, 64)
	if err != nil {
		return errors.Wrap(err, "failed to parse mtk_dram_tool output")
	}

	reportMetric("dramUsage", "bytesPerSecond", float64(dramUsage), perf.SmallerIsBetter, p)
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
