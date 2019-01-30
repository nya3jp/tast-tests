// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package cpu measures CPU usage.
package cpu

import (
	"context"
	"io/ioutil"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/shirou/gopsutil/cpu"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/bintest"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

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

	select {
	case <-time.After(duration):
	case <-ctx.Done():
		return 0, ctx.Err()
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

// MeasureUsageOnRunningBinary runs the whole procedure for measuring CPU usage while running binary
// test. It will disable CPU frequency scaling and thermal throttling first, wait until CPU idle,
// then run the binary at the same time measure CPU usage after stabilization duration.
// killAfterMeasure specifies whether to kill the binary after getting the measurement.
func MeasureUsageOnRunningBinary(ctx context.Context, testExec string, args []string, outDir string,
	stabilizationDuration, measurementDuration time.Duration, killAfterMeasure bool) (float64, error) {
	const (
		// waitIdleCPUTimeout is the maximum time to wait for CPU to become idle.
		waitIdleCPUTimeout = 30 * time.Second
		// idleCPUUsagePercent is the average usage below which CPU is considered idle.
		idleCPUUsagePercent = 10.0
	)

	// CPU frequency scaling and thermal throttling might influence our test results.
	restoreCPUFrequencyScaling, err := DisableCPUFrequencyScaling()
	if err != nil {
		return 0, errors.Wrap(err, "failed to disable CPU frequency scaling")
	}
	defer restoreCPUFrequencyScaling()

	restoreThermalThrottling, err := DisableThermalThrottling(ctx)
	if err != nil {
		return 0, errors.Wrap(err, "failed to disable thermal throttling")
	}
	defer restoreThermalThrottling(ctx)

	if err := WaitForIdle(ctx, waitIdleCPUTimeout, idleCPUUsagePercent); err != nil {
		return 0, errors.Wrap(err, "failed waiting for CPU to become idle")
	}

	cmd, err := bintest.RunAsync(ctx, testExec, args, []string{} /* env */, outDir)
	if err != nil {
		return 0, errors.Wrapf(err, "failed to run %v", testExec)
	}

	testing.ContextLogf(ctx, "Sleeping %v to wait for CPU usage to stabilize", stabilizationDuration.Round(time.Second))
	select {
	case <-ctx.Done():
		return 0, errors.Wrap(err, "failed waiting for CPU usage to stabilize")
	case <-time.After(stabilizationDuration):
	}

	cpuUsage, err := MeasureUsage(ctx, measurementDuration)
	if err != nil {
		return 0, errors.Wrap(err, "failed to measure CPU usage")
	}

	testing.ContextLogf(ctx, "Measured CPU usage: %.4f%%", cpuUsage)

	if killAfterMeasure {
		// We kill the process after we got our measurements. After killing we still need to wait for all resources to get released.
		if err := cmd.Kill(); err != nil {
			return 0, errors.Wrapf(err, "failed to kill %v", testExec)
		}
	}

	if err := cmd.Wait(); err != nil {
		ws := err.(*exec.ExitError).Sys().(syscall.WaitStatus)
		if !ws.Signaled() || ws.Signal() != syscall.SIGKILL {
			return 0, errors.Wrapf(err, "failed to wait %v to be finished", testExec)
		}
	}

	return cpuUsage, nil
}
