// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package cpu provides utility functions for CPU.
package cpu

import (
	"context"
	"io/ioutil"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// WaitForIdle waits until CPU is idle, or timeout is elapsed.
// CPU is evaluated as idle if the CPU usage (percent) is less than utilization.
func WaitForIdle(ctx context.Context, timeout time.Duration, utilization float64) error {
	sleepTime := 1 * time.Second
	fracActiveTime := 1.0
	timePassed := 0 * time.Second
	testing.ContextLogf(ctx, "Starting to wait up to %ds for idle CPU", timeout/time.Second)
	for fracActiveTime >= utilization {
		usageBegin, err := GetUsage(ctx)
		if err != nil {
			return err
		}
		time.Sleep(sleepTime)
		usageEnd, err := GetUsage(ctx)
		if err != nil {
			return err
		}
		fracActiveTime = ComputeActiveTime(ctx, usageBegin, usageEnd)
		timePassed += sleepTime
		testing.ContextLogf(ctx, "After waiting %ds CPU utilization is %.3f", timePassed/time.Second, fracActiveTime)
		if timePassed > timeout {
			// TODO(hiroh): Perhaps, we should not fail the test if this happens frequently.
			return errors.New("CPU did not become idle")
		}

		sleepTime *= 2
		if sleepTime > 16 {
			sleepTime = 16
		}
	}
	testing.ContextLogf(ctx, "Wait for idle CPU took %ds (utilization = %.3f)", timePassed/time.Second, fracActiveTime)
	return nil
}

// GetUsage returns machine's CPU usage. This function uses /proc/stat to identify CPU usage.
// The return value is a dictionary with values for all columns in /proc/stat.
func GetUsage(ctx context.Context) (map[string]int64, error) {
	content, err := ioutil.ReadFile("/proc/stat")
	if err != nil {
		return nil, err
	}
	// The first line in /proc/stat is "cpu" line.
	cpuUsageStr := strings.Trim(strings.Split(string(content), "\n")[0], " ")[len("cpu  "):]
	cpuUsageValues := strings.Split(cpuUsageStr, " ")
	columns := [...]string{"user", "nice", "system", "idle", "iowait", "irq", "softirq", "steal", "guest", "guest_nice"}
	d := make(map[string]int64)
	for i, col := range columns {
		if i >= len(cpuUsageValues) {
			testing.ContextLogf(ctx, "CPU usage string doesn't have %d values: %s", len(columns), cpuUsageStr)
			d[col] = 0
			continue
		}

		val, err := strconv.ParseInt(cpuUsageValues[i], 10, 64)
		if err != nil {
			testing.ContextLogf(ctx, "Failed to convert to int64: %d %v %v", i, cpuUsageValues, err)
			d[col] = 0
		} else {
			d[col] = val
		}
	}
	return d, nil
}

// ComputeActiveTime computes the fraction of CPU time spent non-idling between cpuUsageBegin and cpuUsageEnd.
func ComputeActiveTime(ctx context.Context, cpuUsageBegin, cpuUsageEnd map[string]int64) float64 {
	idleCols := []string{"idle", "iowait"}
	var timeActiveBegin, timeActiveEnd int64 = 0, 0
	var totalTimeBegin, totalTimeEnd int64 = 0, 0
	for k, v := range cpuUsageBegin {
		totalTimeBegin += v
		if k == idleCols[0] || k == idleCols[1] {
			timeActiveBegin += v
		}
	}
	for k, v := range cpuUsageEnd {
		totalTimeEnd += v
		if k == idleCols[0] || k == idleCols[1] {
			timeActiveEnd += v
		}
	}

	// Avoid bogus division which has been observed on Tegra.
	if totalTimeEnd <= totalTimeBegin {
		testing.ContextLogf(ctx, "ComputeActiveCpuTime() observed bogus data")
		// We pretend to be busy, this will force a longer wait for idle CPU.
		return 1.0
	}
	return float64(timeActiveEnd-timeActiveBegin) / float64(totalTimeEnd-totalTimeBegin)
}
