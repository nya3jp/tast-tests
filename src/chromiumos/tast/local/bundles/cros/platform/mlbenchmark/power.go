// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package mlbenchmark contains functionality used by the ml_benchmark tast
// test. This is all implementation that developers don't need to get confused
// by when writing additional scenarios, so keeping it out of the way here.
package mlbenchmark

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/power"
	"chromiumos/tast/testing"
)

// GetReadMomentaryPowerW returns a lambda that returns current momentary power consumption in W.
func GetReadMomentaryPowerW(ctx context.Context) (func() float64, error) {
	batteryPaths, err := power.ListSysfsBatteryPaths(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get battery paths")
	}

	lambda := func() float64 {
		result, err := power.ReadSystemPower(batteryPaths)
		if err != nil {
			// TODO(mblsha): Should we mark the test as failed using |s.Error| here?
			testing.ContextLog(ctx, "Failed to read battery momentary power: ", err)
			return 0
		}
		return result
	}

	return lambda, nil
}

// SamplePower samples momentary power from battery discharge to first calculate
// total power used in Wh, and then converts to Joules.
func SamplePower(ctx context.Context, momentaryPowerW func() float64, sampleInterval time.Duration, quit chan struct{}, result chan float64) {
	if sampleInterval.Seconds() <= 0 {
		result <- 0
		return
	}

	ticker := time.NewTicker(sampleInterval)

	lastTime := time.Now()
	totalPowerWh := 0.0

	updateTotalPower := func(t time.Time) {
		duration := t.Sub(lastTime)
		totalPowerWh += momentaryPowerW() * duration.Seconds() / 3600
		lastTime = t
	}

	for {
		select {
		case <-quit:
			updateTotalPower(time.Now())
			// 1Wh == 3600 Joules.
			result <- totalPowerWh * 3600
			return
		case t := <-ticker.C:
			updateTotalPower(t)
		}
	}
}
