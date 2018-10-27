// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package cpu measures CPU usage.
package cpu

import (
	"context"
	"time"

	psutilcpu "github.com/shirou/gopsutil/cpu"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// WaitForIdle waits until CPU is idle, or timeout is elapsed.
// CPU is evaluated as idle if the CPU usage is less than utilization, a percentage range in the range [0.0, 100.0].
func WaitForIdle(ctx context.Context, timeout time.Duration, utilization float64) error {
	const sleepTime = time.Second
	var timePassed time.Duration
	testing.ContextLogf(ctx, "Starting to wait up to %v for idle CPU", timeout.Round(time.Second))
	for true {
		statBegin, err := GetStat(ctx)
		if err != nil {
			return err
		}
		testing.Poll(ctx, func(ctx context.Context) error {
			time.Sleep(sleepTime)
			return nil
		}, nil)
		statEnd, err := GetStat(ctx)
		if err != nil {
			return err
		}
		cpuUsage := ComputeCPUUsage(ctx, statBegin, statEnd)
		timePassed += sleepTime
		testing.ContextLogf(ctx, "After waiting %v CPU utilization is %.3f%", timePassed.Round(time.Second), cpuUsage)
		if cpuUsage < utilization {
			break
		}
		if timePassed > timeout {
			// TODO(hiroh): Perhaps, we should not fail the test if this happens frequently.
			return errors.New("CPU did not become idle")
		}
	}
	testing.ContextLogf(ctx, "Wait for idle CPU took %v", timePassed.Round(time.Second))
	return nil
}

// GetStat returns CPU stat using gopsutil package, which is the result of /proc/stat/.
func GetStat(ctx context.Context) (*psutilcpu.TimesStat, error) {
	times, err := psutilcpu.Times(false)
	if err != nil {
		return nil, err
	}
	return &times[0], nil
}

// ComputeCPUUsage computes the percentage of CPU time spent non-idling between cpuUsageBegin and cpuUsageEnd.
func ComputeCPUUsage(ctx context.Context, statBegin, statEnd *psutilcpu.TimesStat) float64 {
	activeTimeBegin := activeTime(statBegin)
	totalTimeBegin := statBegin.Total()
	activeTimeEnd := activeTime(statEnd)
	totalTimeEnd := statEnd.Total()

	// Avoid bogus division which has been observed on Tegra.
	if totalTimeEnd <= totalTimeBegin {
		testing.ContextLogf(ctx, "ComputeCPUUsage() observed bogus data")
		// We pretend to be busy, this will force a longer wait for idle CPU.
		return 100.0
	}
	return float64(activeTimeEnd-activeTimeBegin) / float64(totalTimeEnd-totalTimeBegin) * 100.0
}

// activeTime calculates the CPU active time from st.
func activeTime(st *psutilcpu.TimesStat) float64 {
	return st.Total() - (st.Idle + st.Iowait)
}
