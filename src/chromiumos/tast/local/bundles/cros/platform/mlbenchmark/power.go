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
)

// SamplingResult returns the total amount of power consumed or an error.
type SamplingResult struct {
	Value float64
	Err   error
}

type momentaryPowerFunc = func() (float64, error)

// GetReadMomentaryPowerW returns a lambda that returns current momentary power consumption in W.
func GetReadMomentaryPowerW(ctx context.Context) (momentaryPowerFunc, error) {
	batteryPaths, err := power.ListSysfsBatteryPaths(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get battery paths")
	}

	lambda := func() (float64, error) {
		result, err := power.ReadSystemPower(batteryPaths)
		if err != nil {
			return 0, errors.Wrap(err, "failed to read battery momentary power")
		}
		return result, nil
	}

	return lambda, nil
}

// SamplePower samples momentary power from battery discharge to first calculate
// total power used in Wh, and then converts to Joules.
func SamplePower(ctx context.Context, momentaryPowerW momentaryPowerFunc, sampleInterval time.Duration, quit chan struct{}, result chan SamplingResult) {
	if sampleInterval.Seconds() <= 0 {
		result <- SamplingResult{
			Value: 0,
			Err:   nil,
		}
		return
	}

	ticker := time.NewTicker(sampleInterval)

	lastTime := time.Now()
	totalPowerWh := 0.0
	var err error

	updateTotalPower := func(t time.Time) {
		duration := t.Sub(lastTime)
		var p float64
		p, err = momentaryPowerW()
		totalPowerWh += p * duration.Seconds() / 3600
		lastTime = t
	}

	for {
		select {
		case <-quit:
			updateTotalPower(time.Now())
			result <- SamplingResult{
				// 1Wh == 3600 Joules.
				Value: totalPowerWh * 3600,
				Err:   err,
			}
			return
		case t := <-ticker.C:
			updateTotalPower(t)
		}
	}
}
