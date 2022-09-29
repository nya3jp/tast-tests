// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cpu

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// IdleConfig contains the config to wait for the CPU is idle.
type IdleConfig struct {
	// time to wait for CPU to become idle.
	Timeout time.Duration
	// percentage below which CPU is ideally considered idle, gradually
	// increased up to CPUUsagePercentMax.
	CPUUsagePercentBase float64
	// maximum percentage below which CPU is considered idle.
	CPUUsagePercentMax float64
	// number of steps to take while relaxing target CPU idle percentage
	// from CPUUsagePercentBase to CPUUsagePercentMax. Each step has a
	// duration of Timeout/Steps. Steps must be greater than 1.
	Steps int
}

// DefaultIdleConfig returns the default config to wait for the CPU is idle.
func DefaultIdleConfig() IdleConfig {
	return IdleConfig{
		Timeout:             2 * time.Minute,
		CPUUsagePercentBase: 5.0,
		CPUUsagePercentMax:  20.0,
		Steps:               5,
	}
}

// WaitUntilIdle waits until the CPU is idle, for a maximum of 120s. The CPU is
// considered idle if the average usage over all CPU cores is less than 5%.
// This percentage will be gradually increased to 20%, as older boards might
// have a hard time getting below 5%.
func WaitUntilIdle(ctx context.Context) error {
	return WaitUntilIdleWithConfig(ctx, DefaultIdleConfig())
}

// WaitUntilIdleWithConfig waits until the CPU is idle, based on provided configuration.
func WaitUntilIdleWithConfig(ctx context.Context, config IdleConfig) error {
	// Wait for the CPU to become idle. It's e.g. possible the board just booted
	// and is running various startup programs. Some slower platforms have a
	// hard time getting below 10% CPU usage, so we'll gradually increase the
	// CPU idle threshold.
	if config.Steps < 2 {
		return errors.Errorf("invalid Steps in config: got %d; want >= 2", config.Steps)
	}
	var err error
	startTime := time.Now()
	idleIncrease := (config.CPUUsagePercentMax - config.CPUUsagePercentBase) / (float64(config.Steps) - 1)
	testing.ContextLogf(ctx, "Waiting for idle CPU at most %v, threshold will be gradually relaxed (from %.1f%% to %.1f%%)",
		config.Timeout, config.CPUUsagePercentBase, config.CPUUsagePercentMax)
	for i := 0; i < config.Steps; i++ {
		idlePercent := config.CPUUsagePercentBase + (idleIncrease * float64(i))
		timeout := time.Duration(config.Timeout.Seconds()/float64(config.Steps)) * time.Second
		testing.ContextLogf(ctx, "Waiting up to %v for CPU usage to drop below %.1f%% (%d/%d)",
			timeout.Round(time.Second), idlePercent, i+1, config.Steps)
		var usage float64
		if usage, err = waitUntilIdleStep(ctx, timeout, idlePercent); err == nil {
			testing.ContextLogf(ctx, "Waiting for idle CPU took %v (usage: %.1f%%, threshold: %.1f%%)",
				time.Now().Sub(startTime).Round(time.Second), usage, idlePercent)
			return nil
		}
	}
	return err
}

// waitUntilIdleStep waits until the CPU is idle or the specified timeout has
// elapsed and returns CPU usage. The CPU is considered idle if the average CPU
// usage over all cores is less than maxUsage, which is a percentage in the
// range [0.0, 100.0].
func waitUntilIdleStep(ctx context.Context, timeout time.Duration, maxUsage float64) (usage float64, err error) {
	const measureDuration = time.Second
	err = testing.Poll(ctx, func(context.Context) error {
		var e error
		// testing.Poll shortens ctx so that its deadline matches timeout. Use the original ctx to
		// prevent the Sleep in cpu.MeasureUsage from always failing during the last poll iteration.
		usage, e = MeasureUsage(ctx, measureDuration)
		if e != nil {
			return testing.PollBreak(errors.Wrap(e, "failed measuring CPU usage"))
		}
		if usage >= maxUsage {
			return errors.Errorf("CPU not idle: got %.1f%%; want < %.1f%%", usage, maxUsage)
		}
		return nil
	}, &testing.PollOptions{Timeout: timeout})
	if err != nil {
		return usage, err
	}
	return usage, nil
}
