// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package cpu measures CPU usage.
package cpu

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"syscall"
	"time"

	"github.com/shirou/gopsutil/cpu"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

// StartProcFunc starts a process and returns corresponding testexec.Cmd.
// The caller is responsible for calling Wait.
type StartProcFunc func() (*testexec.Cmd, error)

// MeasureProcessCPU starts a process by calling the provided runCmdAsync and
// measures CPU usage for the given duration. After measuring the process is
// killed. The average usage over all CPU cores is returned as a percentage.
func MeasureProcessCPU(ctx context.Context, runCmdAsync StartProcFunc, duration time.Duration) (float64, error) {
	const (
		stabilize   = 1 * time.Second // time to wait for CPU to stabilize after launching proc.
		cleanupTime = 5 * time.Second // time reserved for cleanup after measuring.
	)

	// Start the process asynchronous by calling the provided startup function.
	cmd, err := runCmdAsync()
	if err != nil {
		return 0.0, errors.Wrap(err, "failed to run binary")
	}

	// Kill and clean up the process upon exiting the function.
	defer func() {
		if err := cmd.Kill(); err != nil {
			testing.ContextLog(ctx, "Failed to kill process: ", err)
		}
		// After killing the process we still need to wait for all resources to get released.
		if err := cmd.Wait(); err != nil {
			ws, ok := testexec.GetWaitStatus(err)
			if !ok || !ws.Signaled() || ws.Signal() != syscall.SIGKILL {
				testing.ContextLog(ctx, "Failed waiting for the command to exit: ", err)
			}
		}
	}()

	// Use a shorter context to leave time for cleanup upon failure.
	ctx, cancel := ctxutil.Shorten(ctx, cleanupTime)
	defer cancel()

	if err := testing.Sleep(ctx, stabilize); err != nil {
		return 0.0, errors.Wrap(err, "failed waiting for CPU usage to stabilize")
	}

	testing.ContextLog(ctx, "Measuring CPU usage for ", duration.Round(time.Second))
	cpuUsage, err := MeasureUsage(ctx, duration)
	if err != nil {
		return 0.0, errors.Wrap(err, "failed to measure CPU usage on running command")
	}

	return cpuUsage, nil
}

// SetUpBenchmark performs setup needed for running benchmarks. It disables CPU frequency scaling
// and thermal throttling, and waits for the CPU to become idle. A deferred call to the returned
// cleanUp function should be scheduled by the caller if err is non-nil.
func SetUpBenchmark(ctx context.Context) (cleanUp func(ctx context.Context), err error) {
	const cleanupTime = 10 * time.Second // time reserved for cleanup on error.

	var restoreScaling func(ctx context.Context) error
	var restoreThrottling func(ctx context.Context) error
	cleanUp = func(ctx context.Context) {
		if restoreScaling != nil {
			if err = restoreScaling(ctx); err != nil {
				testing.ContextLog(ctx, "Failed to restore CPU frequency scaling to original values: ", err)
			}
		}
		if restoreThrottling != nil {
			if err = restoreThrottling(ctx); err != nil {
				testing.ContextLog(ctx, "Failed to restore CPU thermal throttling to original values: ", err)
			}
		}
	}

	// Run the cleanUp function automatically if we encounter an error.
	doCleanup := cleanUp
	defer func() {
		if doCleanup != nil {
			doCleanup(ctx)
		}
	}()

	// Run all non-cleanup operations with a shorter context. This ensures
	// thermal throttling and CPU frequency scaling get re-enabled, even when
	// test execution exceeds the maximum time allowed.
	ctx, cancel := ctxutil.Shorten(ctx, cleanupTime)
	defer cancel()

	// CPU frequency scaling and thermal throttling might influence our test results.
	if restoreScaling, err = disableCPUFrequencyScaling(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to disable CPU frequency scaling")
	}
	if restoreThrottling, err = disableThermalThrottling(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to disable thermal throttling")
	}

	// Disarm running the cleanUp function now that we expect the caller to do it.
	doCleanup = nil
	return cleanUp, nil
}

// WaitUntilIdle waits until the CPU is idle, for a maximum of 60s. The CPU is
// considered idle if the average usage over all CPU cores is less than 5%.
// This percentage will be gradually increased to 20%, as older boards might
// have a hard time getting below 5%.
func WaitUntilIdle(ctx context.Context) error {
	const (
		// time to wait for CPU to become idle.
		waitIdleCPUTimeout = 60 * time.Second
		// percentage below which CPU is ideally considered idle, gradually
		// increased up to idleCPUUsagePercentMax.
		idleCPUUsagePercentBase = 5.0
		// maximum percentage below which CPU is considered idle.
		idleCPUUsagePercentMax = 20.0
		// times we wait for CPU to become idle, idle percentage is increased each time.
		idleCPUSteps = 5
	)

	// Wait for the CPU to become idle. It's e.g. possible the board just booted
	// and is running various startup programs. Some slower platforms have a
	// hard time getting below 10% CPU usage, so we'll gradually increase the
	// CPU idle threshold.
	var err error
	idleIncrease := (idleCPUUsagePercentMax - idleCPUUsagePercentBase) / (idleCPUSteps - 1)
	for i := 0; i < idleCPUSteps; i++ {
		idlePercent := idleCPUUsagePercentBase + (idleIncrease * float64(i))
		if err = waitUntilIdleStep(ctx, waitIdleCPUTimeout/idleCPUSteps, idlePercent); err == nil {
			return nil
		}
	}
	return err
}

// waitUntilIdleStep waits until the CPU is idle or the specified timeout has
// elapsed. The CPU is considered idle if the average CPU usage over all cores
// is less than maxUsage, which is a percentage in the range [0.0, 100.0].
func waitUntilIdleStep(ctx context.Context, timeout time.Duration, maxUsage float64) error {
	const sleepTime = time.Second
	startTime := time.Now()
	testing.ContextLogf(ctx, "Waiting up to %v for CPU usage to drop below %.1f%%", timeout.Round(time.Second), maxUsage)
	err := testing.Poll(ctx, func(ctx context.Context) error {
		usage, err := MeasureUsage(ctx, sleepTime)
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed measuring CPU usage"))
		}
		if usage >= maxUsage {
			return errors.Errorf("CPU not idle: got %.1f%%; want < %.1f%%", usage, maxUsage)
		}
		return nil
	}, &testing.PollOptions{Timeout: timeout})
	if err != nil {
		return err
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

// cpuConfigEntry holds a single CPU config entry. If ignoreErrors is true
// failure to apply the config will result in a warning, rather than an error.
// This is needed as on some platforms we might not have the right permissions
// to disable frequency scaling.
type cpuConfigEntry struct {
	value        string
	ignoreErrors bool
}

// disableCPUFrequencyScaling disables frequency scaling. All CPU cores will be
// set to always run at their maximum frequency. A function is returned so the
// caller can restore the original CPU frequency scaling configuration.
// Depending on the platform different mechanisms are present:
//  - Most platforms use the scaling_governor to control CPU frequency scaling.
//  - Some platforms (e.g. Dru) use a different CPU frequency scaling governor.
//  - Some Intel-based platforms (e.g. Eve and Nocturne) ignore the values set
//    in the scaling_governor, and instead use the intel_pstate application to
//    control CPU frequency scaling.
func disableCPUFrequencyScaling(ctx context.Context) (func(ctx context.Context) error, error) {
	optimizedConfig := make(map[string]cpuConfigEntry)
	for glob, config := range map[string]cpuConfigEntry{
		// crbug.com/977925: Disabled hyperthreading cores are listed but
		// writing config for these disabled cores results in 'invalid argument'.
		// TODO(dstaessens): Skip disabled CPU cores when setting scaling_governor.
		"/sys/devices/system/cpu/cpu[0-9]*/cpufreq/scaling_governor": {"performance", true},
		"/sys/class/devfreq/devfreq[0-9]*/governor":                  {"performance", true},
		// crbug.com/938729: BIOS settings might prevent us from overwriting intel_pstate/no_turbo.
		"/sys/devices/system/cpu/intel_pstate/no_turbo":     {"1", true},
		"/sys/devices/system/cpu/intel_pstate/min_perf_pct": {"100", false},
		"/sys/devices/system/cpu/intel_pstate/max_perf_pct": {"100", false},
	} {
		paths, err := filepath.Glob(glob)
		if err != nil {
			return nil, err
		}
		for _, path := range paths {
			optimizedConfig[path] = config
		}
	}

	origConfig, err := applyConfig(ctx, optimizedConfig)
	undo := func(ctx context.Context) error {
		_, err := applyConfig(ctx, origConfig)
		return err
	}
	if err != nil {
		undo(ctx)
		return nil, err
	}
	return undo, nil
}

// applyConfig applies the specified frequency scaling configuration. A map of
// path-value pairs needs to be provided. A map of the original path-value pairs
// is returned to allow restoring the original config. If ignoreErrors is true
// for a config entry we won't return an error upon failure, but will only show
// a warning. The provided context will only be used for logging, so the config
// will even be applied upon timeout.
func applyConfig(ctx context.Context, cpuConfig map[string]cpuConfigEntry) (map[string]cpuConfigEntry, error) {
	origConfig := make(map[string]cpuConfigEntry)
	for path, config := range cpuConfig {
		origValue, err := ioutil.ReadFile(path)
		if err != nil {
			if !config.ignoreErrors {
				return origConfig, err
			}
			testing.ContextLogf(ctx, "Failed to read %v: %v", path, err)
			continue
		}
		if err = ioutil.WriteFile(path, []byte(config.value), 0644); err != nil {
			if !config.ignoreErrors {
				return origConfig, err
			}
			testing.ContextLogf(ctx, "Failed to write to %v: %v", path, err)
			continue
		}
		origConfig[path] = cpuConfigEntry{string(origValue), false}
	}
	return origConfig, nil
}

// disableThermalThrottling disables thermal throttling, as it might interfere
// with test execution. A function is returned that restores the original
// settings, so the caller can re-enable thermal throttling after testing.
func disableThermalThrottling(ctx context.Context) (func(context.Context) error, error) {
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
