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
// CPU is evaluated as idle if the CPU usage is less than maxUsage, a percentage range in the range [0.0, 100.0].
func WaitForIdle(ctx context.Context, timeout time.Duration, maxUsage float64) error {
	const sleepTime = time.Second
	var timePassed time.Duration
	testing.ContextLogf(ctx, "Waiting up to %v for CPU usage to drop below %.1f%%", timeout.Round(time.Second), maxUsage)
	err := testing.Poll(ctx, func(ctx context.Context) error {
		usage, err := MeasureUsage(ctx, sleepTime)
		if err != nil {
			return err
		}
		testing.ContextLogf(ctx, "CPU usage is %.1f%%", usage)
		timePassed += sleepTime
		if usage >= maxUsage {
			return errors.Errorf("CPU usage is %.1f%%; want less than %.1f%%", usage, maxUsage)
		}
		return nil
	}, &testing.PollOptions{Timeout: timeout})
	if err != nil {
		return errors.Wrap(err, "CPU didn't become idle")
	}
	testing.ContextLogf(ctx, "Wait for idle CPU took %v", timePassed.Round(time.Second))
	return nil
}

// MeasureUsage measures CPU usage from utilization stats during duration.
func MeasureUsage(ctx context.Context, duration time.Duration) (float64, error) {
	statBegin, err := getStat()
	if err != nil {
		return 0, err
	}

	select {
	case <-time.After(duration):
	case <-ctx.Done():
		return 0, err
	}

	statEnd, err := getStat()
	if err != nil {
		return 0, err
	}

	return computeUsage(statBegin, statEnd)
}

// getStat returns utilization stats across all CPUs as reported by /proc/stat.
func getStat() (*psutilcpu.TimesStat, error) {
	times, err := psutilcpu.Times(false)
	if err != nil {
		return nil, err
	}
	return &times[0], nil
}

// computeUsage computes the percentage of CPU time spent non-idling between statBegin and statEnd.
// The returned value is percentage in the range [0.0, 100.0].
func computeUsage(statBegin, statEnd *psutilcpu.TimesStat) (float64, error) {
	totalTimeBegin := statBegin.Total()
	activeTimeBegin := totalTimeBegin - (statBegin.Idle + statBegin.Iowait)
	totalTimeEnd := statEnd.Total()
	activeTimeEnd := totalTimeEnd - (statEnd.Idle + statEnd.Iowait)

	if totalTimeEnd <= totalTimeBegin {
		return 100.0, errors.New("ComputeUsage() observed bogus data")
	}
	return float64(activeTimeEnd-activeTimeBegin) / float64(totalTimeEnd-totalTimeBegin) * 100.0, nil
}
