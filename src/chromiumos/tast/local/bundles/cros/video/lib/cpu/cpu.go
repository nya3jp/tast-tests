// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package cpu measures CPU usage.
package cpu

import (
	//"bytes"
	"context"
	"io/ioutil"
	"regexp"
	"strings"
	"time"

	"github.com/shirou/gopsutil/cpu"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
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
	testing.ContextLogf(ctx, "Wait for idle CPU took %v", time.Now().Sub(startTime).Round(time.Second))
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
	// doesn't check whether the timeout in ctx is exceeded.
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

// DisableCPUFrequencyScaling disables frequency scaling. All CPU cores are set
// to 'performance' mode, to always run on their maximum frequency. The original
// frequency scaling modes are returned.
func DisableCPUFrequencyScaling(ctx context.Context) ([]string, error) {
	origCPUFreqScalingModes, err := getCPUFrequencyScalingModes(ctx)
	if err != nil {
		return nil, err
	}
	modes := make([]string, len(origCPUFreqScalingModes))
	for i := 0; i < len(modes); i++ {
		modes[i] = "performance"
	}
	err = SetCPUFrequencyScalingModes(ctx, modes)
	return origCPUFreqScalingModes, err
}

// SetCPUFrequencyScalingModes sets the frequency scaling modes to the specified
// values.
func SetCPUFrequencyScalingModes(ctx context.Context, modes []string) error {
	paths, _ := getCPUPaths(ctx)
	if len(modes) != len(paths) {
		return errors.New("Wrong number of CPU frequency scaling modes provided")
	}
	for i, path := range paths {
		if err := ioutil.WriteFile(path+"/cpufreq/scaling_governor", []byte(modes[i]), 0644); err != nil {
			return err
		}
	}
	return nil
}

// getCPUFrequencyScalingModes gets a list containing the current CPU frequency
// scaling modes.
func getCPUFrequencyScalingModes(ctx context.Context) ([]string, error) {
	paths, _ := getCPUPaths(ctx)
	states := []string{}
	for _, path := range paths {
		state, err := ioutil.ReadFile(path + "/cpufreq/scaling_governor")
		if err != nil {
			return nil, err
		}
		states = append(states, string(state))
	}
	return states, nil
}

// getCPUPaths gets a list of paths corresponding to the cpu cores on the system.
func getCPUPaths(ctx context.Context) ([]string, error) {
	const exec = "/sys/devices/system/cpu/"
	cmd := testexec.CommandContext(ctx, "sudo", "-u", "chronos", "ls", exec)
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	// Filter out the list of cpu cores (cpu0, cpu1,...).
	paths := []string{}
	for _, line := range strings.Split(string(output), "\n") {
		match, _ := regexp.MatchString(`^cpu\d+$`, line)
		if match {
			paths = append(paths, exec+line)
		}
	}
	return paths, nil
}
