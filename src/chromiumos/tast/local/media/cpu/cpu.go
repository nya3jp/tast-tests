// Copyright 2018 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package cpu measures CPU usage.
package cpu

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"sync"
	"time"

	"golang.org/x/sys/unix"

	"chromiumos/tast/common/testexec"
	upstartcommon "chromiumos/tast/common/upstart"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/cpu"
	"chromiumos/tast/local/gtest"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

// ExitOption describes how to clean up the child process upon function exit.
type ExitOption int

const (
	// KillProcess option kills the child process when the function is done.
	KillProcess ExitOption = iota
	// WaitProcess option waits for the child process to finish.
	WaitProcess
)

// raplExec is the command used to measure power consumption, only supported on Intel platforms.
const raplExec = "/usr/bin/dump_intel_rapl_consumption"

// MeasureProcessUsage starts one or more gtest processes and measures CPU usage and power consumption asynchronously
// for the given duration. A map is returned containing CPU usage (percentage in [0-100] range) with key "cpu" and power
// consumption (Watts) with key "power" if supported.
func MeasureProcessUsage(ctx context.Context, duration time.Duration,
	exitOption ExitOption, ts ...*gtest.GTest) (measurements map[string]float64, retErr error) {
	const (
		stabilizeTime = 1 * time.Second // time to wait for CPU to stabilize after launching proc.
		cleanupTime   = 5 * time.Second // time reserved for cleanup after measuring.
	)

	for _, t := range ts {
		// Start the process asynchronously by calling the provided startup function.
		cmd, err := t.Start(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "failed to run binary")
		}

		// Clean up the process upon exiting the function.
		defer func() {
			// If the exit option is 'WaitProcess' wait for the process to terminate.
			if exitOption == WaitProcess {
				if err := cmd.Wait(); err != nil {
					retErr = err
					testing.ContextLog(ctx, "Failed waiting for the command to exit: ", retErr)
				}
				return
			}

			// If the exit option is 'KillProcess' we will send a 'SIGKILL' signal
			// to the process after collecting performance metrics.
			if err := cmd.Kill(); err != nil {
				retErr = err
				testing.ContextLog(ctx, "Failed to kill process: ", retErr)
				return
			}

			// After sending a 'SIGKILL' signal to the process we need to wait
			// for the process to terminate. If Wait() doesn't return any error,
			// we know the process already terminated before we explicitly killed
			// it and the measured performance metrics are invalid.
			err = cmd.Wait()
			if err == nil {
				retErr = errors.New("process did not run for entire measurement duration")
				testing.ContextLog(ctx, retErr)
				return
			}

			// Check whether the process was terminated with a 'SIGKILL' signal.
			ws, ok := testexec.GetWaitStatus(err)
			if !ok {
				retErr = errors.Wrap(err, "failed to get wait status")
				testing.ContextLog(ctx, retErr)
			} else if !ws.Signaled() || ws.Signal() != unix.SIGKILL {
				retErr = errors.Wrap(err, "process did not terminate with SIGKILL signal")
				testing.ContextLog(ctx, retErr)
			}
		}()
	}

	// Use a shorter context to leave time for cleanup upon failure.
	ctx, cancel := ctxutil.Shorten(ctx, cleanupTime)
	defer cancel()

	if err := testing.Sleep(ctx, stabilizeTime); err != nil {
		return nil, errors.Wrap(err, "failed waiting for CPU usage to stabilize")
	}

	testing.ContextLog(ctx, "Measuring CPU usage and power consumption for ", duration.Round(time.Second))
	return MeasureUsage(ctx, duration)
}

// SetUpBenchmark performs setup needed for running benchmarks. It disables CPU
// frequency scaling and thermal throttling. A deferred call to the returned
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

// MeasureUsage measures the average utilization across all CPUs and the
// average SoC 'pkg' power consumption during the specified duration. Measuring
// power consumption is currently not supported on all platforms. A map is
// returned containing CPU usage (percentage in [0-100] range) and power
// consumption (Watts) if supported.
func MeasureUsage(ctx context.Context, duration time.Duration) (map[string]float64, error) {
	var cpuUsage, powerConsumption float64
	var cpuErr, powerErr error
	var wg sync.WaitGroup

	// Start measuring CPU usage asynchronously.
	wg.Add(1)
	go func() {
		defer wg.Done()
		cpuUsage, cpuErr = cpu.MeasureUsage(ctx, duration)
	}()

	// Start measuring power consumption asynchronously. Power consumption
	// is currently only measured on Intel devices that support the
	// dump_intel_rapl_consumption command.
	if _, powerErr = os.Stat(raplExec); powerErr == nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if powerConsumption, powerErr = MeasurePowerConsumption(ctx, duration); powerErr != nil {
				testing.ContextLog(ctx, "Measuring power consumption failed: ", powerErr)
			}
		}()
	}

	wg.Wait()

	measurements := make(map[string]float64)
	if cpuErr == nil {
		measurements["cpu"] = cpuUsage
	}
	if powerErr == nil {
		measurements["power"] = powerConsumption
	}

	// Ignore powerErr as not all platforms support measuring power consumption.
	return measurements, cpuErr
}

// MeasurePowerConsumption measures power consumption during the specified
// duration and returns the average power consumption (in Watts). The power
// consumption is acquired by reading the RAPL 'pkg' entry, which gives a
// measure of the total SoC power consumption.
func MeasurePowerConsumption(ctx context.Context, duration time.Duration) (float64, error) {
	cmd := testexec.CommandContext(ctx, raplExec, "--interval_ms="+
		strconv.FormatInt(int64(duration/time.Millisecond), 10))
	powerConsumptionOutput, err := cmd.CombinedOutput()
	if err != nil {
		return 0.0, err
	}

	var powerConsumptionRegex = regexp.MustCompile(`(\d+\.\d+)`)
	match := powerConsumptionRegex.FindAllString(string(powerConsumptionOutput), 1)
	if len(match) != 1 {
		return 0.0, errors.Errorf("failed to parse output of %s", raplExec)
	}
	powerConsumption, err := strconv.ParseFloat(match[0], 64)
	if err != nil {
		return 0.0, err
	}

	return powerConsumption, nil
}

// cpuConfigEntry holds a single CPU config entry. If ignoreErrors is true
// failure to apply the config will result in a warning, rather than an error.
// This is needed as on some platforms we might not have the right permissions
// to disable frequency scaling.
type cpuConfigEntry struct {
	path         string
	value        string
	ignoreErrors bool
}

// disableCPUFrequencyScaling disables frequency scaling. All CPU cores will be
// set to always run at their maximum frequency. A function is returned so the
// caller can restore the original CPU frequency scaling configuration.
// Depending on the platform different mechanisms are present:
//   - Some Intel-based platforms (e.g. Eve and Nocturne) ignore the values set
//     in the scaling_governor, and instead use the intel_pstate application to
//     control CPU frequency scaling.
//   - Most platforms use the scaling_governor to control CPU frequency scaling.
//   - Some platforms (e.g. Dru) use a different CPU frequency scaling governor.
func disableCPUFrequencyScaling(ctx context.Context) (func(ctx context.Context) error, error) {
	configPatterns := []cpuConfigEntry{
		// crbug.com/938729: BIOS settings might prevent us from overwriting intel_pstate/no_turbo.
		{"/sys/devices/system/cpu/intel_pstate/no_turbo", "1", true},
		// Fix the intel_pstate percentage to 100 if possible. We raise the
		// maximum value before the minimum value as the min cannot exceed the
		// max. To restore them, the order must be inverted. Note that we set
		// and save the original values for these values because changing
		// scaling_governor to "performance" can change these values as well.
		{"/sys/devices/system/cpu/intel_pstate/max_perf_pct", "100", false},
		{"/sys/devices/system/cpu/intel_pstate/min_perf_pct", "100", false},
		// crbug.com/977925: Disabled hyperthreading cores are listed but
		// writing config for these disabled cores results in 'invalid argument'.
		// TODO(dstaessens): Skip disabled CPU cores when setting scaling_governor.
		{"/sys/devices/system/cpu/cpu[0-9]*/cpufreq/scaling_governor", "performance", true},
		{"/sys/class/devfreq/devfreq[0-9]*/governor", "performance", true},
	}

	var optimizedConfig []cpuConfigEntry
	// Expands patterns in configPatterns and pack actual configs into
	// optimizedConfig.
	for _, config := range configPatterns {
		paths, err := filepath.Glob(config.path)
		if err != nil {
			return nil, err
		}
		for _, path := range paths {
			optimizedConfig = append(optimizedConfig, cpuConfigEntry{
				path,
				config.value,
				config.ignoreErrors,
			})
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

// applyConfig applies the specified frequency scaling configuration. A slice of
// cpuConfigEntry needs to be provided and will be processed in order. A slice
// of the original cpuConfigEntry values that were successfully processed is
// returned in reverse order so the caller can restore the original config by
// passing the slice to this function as is. If ignoreErrors is true for a
// config entry we won't return an error upon failure, but will only show a
// warning. The provided context will only be used for logging, so the config
// will even be applied upon timeout.
func applyConfig(ctx context.Context, cpuConfig []cpuConfigEntry) ([]cpuConfigEntry, error) {
	var origConfig []cpuConfigEntry
	for _, config := range cpuConfig {
		origValue, err := ioutil.ReadFile(config.path)
		if err != nil {
			if !config.ignoreErrors {
				return origConfig, err
			}
			testing.ContextLogf(ctx, "Failed to read %v: %v", config.path, err)
			continue
		}
		if err = ioutil.WriteFile(config.path, []byte(config.value), 0644); err != nil {
			if !config.ignoreErrors {
				return origConfig, err
			}
			testing.ContextLogf(ctx, "Failed to write to %v: %v", config.path, err)
			continue
		}
		// Inserts a new entry at the front of origConfig.
		e := cpuConfigEntry{config.path, string(origValue), false}
		origConfig = append([]cpuConfigEntry{e}, origConfig...)
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
	} else if state != upstartcommon.RunningState {
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
