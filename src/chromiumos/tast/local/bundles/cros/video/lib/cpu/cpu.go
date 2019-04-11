// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package cpu measures CPU usage.
package cpu

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"time"

	"github.com/shirou/gopsutil/cpu"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

// SetUpBenchmark performs setup needed for running benchmarks. It enables
// verbose logging, disables CPU frequency scaling and thermal throttling, and
// waits for the CPU to become idle. The returned shortCtx should be used to
// perform testing, to leave time for cleanup operations. A deferred call to the
// returned undo function should be scheduled by the caller if err is non-nil.
func SetUpBenchmark(ctx context.Context) (shortCtx context.Context, undo func(), err error) {
	const (
		waitIdleCPUTimeout  = 30 * time.Second // time to wait for CPU to be idle.
		idleCPUUsagePercent = 10.0             // percent below which CPU is idle.
		cleanupTime         = 10 * time.Second // time reserved for cleanup after running test.
	)

	var restoreScaling func() error
	var cancel func()
	var restoreThrottling func(ctx context.Context) error
	undo = func() {
		cancel()
		if restoreScaling != nil {
			restoreScaling()
		}
		if restoreThrottling != nil {
			restoreThrottling(ctx)
		}
	}

	// Run the undo function automatically if we encounter an error.
	cleanup := undo
	defer func() {
		if cleanup != nil {
			cleanup()
		}
	}()

	// Run all non-cleanup operations with a shorter context. This ensures
	// thermal throttling and CPU frequency scaling get re-enabled, even when
	// test execution exceeds the maximum time allowed.
	shortCtx, cancel = ctxutil.Shorten(ctx, cleanupTime)

	// CPU frequency scaling and thermal throttling might influence our test results.
	if restoreScaling, err = DisableCPUFrequencyScaling(); err != nil {
		return shortCtx, nil, errors.Wrap(err, "failed to disable CPU frequency scaling")
	}
	if restoreThrottling, err = DisableThermalThrottling(shortCtx); err != nil {
		return shortCtx, nil, errors.Wrap(err, "failed to disable thermal throttling")
	}

	if err = WaitForIdle(shortCtx, waitIdleCPUTimeout, idleCPUUsagePercent); err != nil {
		return shortCtx, nil, errors.Wrap(err, "failed waiting for CPU to become idle")
	}

	// Disarm running the undo function now that we expect the caller to do it.
	cleanup = nil
	return shortCtx, undo, nil
}

// WaitForIdle waits until CPU is idle, or timeout is elapsed.
// CPU is evaluated as idle if the CPU usage is less than maxUsage, a percentage in the range [0.0, 100.0].
func WaitForIdle(ctx context.Context, timeout time.Duration, maxUsage float64) error {
	const sleepTime = time.Second
	startTime := time.Now()
	testing.ContextLogf(ctx, "Waiting up to %v for CPU usage to drop below %.1f%%", timeout.Round(time.Second), maxUsage)
	err := testing.Poll(ctx, func(ctx context.Context) error {
		usage, err := MeasureUsage(ctx, sleepTime)
		if err != nil {
			return err
		}
		if usage >= maxUsage {
			return errors.Errorf("CPU usage is %.1f%%; want less than %.1f%%", usage, maxUsage)
		}
		return nil
	}, &testing.PollOptions{Timeout: timeout})
	if err != nil {
		return errors.Wrap(err, "CPU didn't become idle")
	}
	testing.ContextLog(ctx, "Wait for idle CPU took ", time.Now().Sub(startTime).Round(time.Second))
	return nil
}

// MeasureUsage measures utilization across all CPUs during duration.
// Returns a percentage in the range [0.0, 100.0].
func MeasureUsage(ctx context.Context, duration time.Duration) (float64, error) {
	// Get the total time the CPU spent in different states (read from
	// /proc/stat on linux machines).
	statBegin, err := getStat()
	if err != nil {
		return 0, err
	}

	if err := testing.Sleep(ctx, duration); err != nil {
		return 0, err
	}

	// Get the total time the CPU spent in different states again. By looking at
	// the difference with the values we got earlier, we can calculate the time
	// the processor was idle. The gopsutil library also has a function that
	// does this directly, but unfortunately we can't use it as that function
	// doesn't abort when the timeout in ctx is exceeded.
	statEnd, err := getStat()
	if err != nil {
		return 0, err
	}

	totalTimeBegin := statBegin.Total()
	activeTimeBegin := totalTimeBegin - (statBegin.Idle + statBegin.Iowait)
	totalTimeEnd := statEnd.Total()
	activeTimeEnd := totalTimeEnd - (statEnd.Idle + statEnd.Iowait)

	if totalTimeEnd <= totalTimeBegin {
		return 0.0, errors.Errorf("total time went from %f to %f", totalTimeBegin, totalTimeEnd)
	}

	return (activeTimeEnd - activeTimeBegin) / (totalTimeEnd - totalTimeBegin) * 100.0, nil
}

// getStat returns utilization stats across all CPUs as reported by /proc/stat.
func getStat() (*cpu.TimesStat, error) {
	times, err := cpu.Times(false)
	if err != nil {
		return nil, err
	}
	return &times[0], nil
}

// DisableCPUFrequencyScaling disables frequency scaling. All CPU cores will be
// set to always run at their maximum frequency. A function is returned so the
// caller can restore the original CPU frequency scaling configuration.
// Depending on the platform different mechanisms are present:
//  - Most platforms use the scaling_governor to control CPU frequency scaling.
//  - Some platforms (e.g. Dru) use a different CPU frequency scaling governor.
//  - Some Intel-based platforms (e.g. Eve and Nocturne) ignore the values set
//    in the scaling_governor, and instead use the intel_pstate application to
//    control CPU frequency scaling.
func DisableCPUFrequencyScaling() (func() error, error) {
	origConfig := make(map[string]string)
	optimizedConfig := make(map[string]string)
	for glob, value := range map[string]string{
		"/sys/devices/system/cpu/cpu[0-9]*/cpufreq/scaling_governor": "performance",
		"/sys/class/devfreq/devfreq[0-9]*/governor":                  "performance",
		"/sys/devices/system/cpu/intel_pstate/no_turbo":              "1",
		"/sys/devices/system/cpu/intel_pstate/min_perf_pct":          "100",
		"/sys/devices/system/cpu/intel_pstate/max_perf_pct":          "100",
	} {
		paths, err := filepath.Glob(glob)
		if err != nil {
			return nil, err
		}
		for _, path := range paths {
			origValue, err := ioutil.ReadFile(path)
			if err != nil {
				return nil, err
			}
			origConfig[path] = string(origValue)
			optimizedConfig[path] = value
		}
	}

	undo := func() error { return applyConfig(origConfig) }
	if err := applyConfig(optimizedConfig); err != nil {
		undo()
		return nil, err
	}
	return undo, nil
}

// applyConfig applies the specified frequency scaling configuration. A map of
// path-value pairs need to be provided.
func applyConfig(config map[string]string) error {
	for path, value := range config {
		if err := ioutil.WriteFile(path, []byte(value), 0644); err != nil {
			return err
		}
	}
	return nil
}

// DisableThermalThrottling disables thermal throttling, as it might interfere
// with test execution. A function is returned that restores the original
// settings, so the caller can re-enable thermal throttling after testing.
func DisableThermalThrottling(ctx context.Context) (func(context.Context) error, error) {
	job := getThermalThrottlingJob(ctx)
	if job == "" {
		return func(ctx context.Context) error { return nil }, nil
	}

	_, state, _, err := upstart.JobStatus(ctx, job)
	if err != nil {
		return nil, err
	} else if state != upstart.RunningState {
		return func(ctx context.Context) error { return nil }, nil
	}

	if err := upstart.StopJob(ctx, job); err != nil {
		return nil, err
	}

	undo := func(ctx context.Context) error { return upstart.EnsureJobRunning(ctx, job) }
	return undo, nil
}

// getThermalThrottlingJob tries to determine the name of the thermal throttling
// job used by the current platform.
func getThermalThrottlingJob(ctx context.Context) string {
	// List of possible thermal throttling jobs that should be disabled:
	// - dptf for intel >= baytrail
	// - temp_metrics for link
	// - thermal for daisy, snow, pit,...
	for _, job := range []string{"dptf", "temp_metrics", "thermal"} {
		if upstart.JobExists(ctx, job) {
			return job
		}
	}
	return ""
}
