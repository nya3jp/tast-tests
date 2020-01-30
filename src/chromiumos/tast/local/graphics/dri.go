// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package graphics contains graphics-related utility functions for local tests.
package graphics

import (
	"context"
	"io/ioutil"
	"math"
	"os"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

const (
	// The debugfs file providing information on the amount of allocated framebuffers.
	i915FramebufferFile = "/sys/kernel/debug/dri/0/i915_gem_framebuffer"
	// Immedaitely after login there's a lot of graphics activity; wait for a
	// minute until it subsides.
	coolDownTimeAfterLogin = 60 * time.Second
)

// SupportsI915FramebufferInfo returns true if the debug fs for memory usage
// readings is available.
func SupportsI915FramebufferInfo() bool {
	_, err := os.Stat(i915FramebufferFile)
	return err == nil
}

// CompareGraphicsMemoryBeforeAfter compares the graphics memory consumption
// before and after running payload.
func CompareGraphicsMemoryBeforeAfter(ctx context.Context, payload func()) (err error) {
	var before int
	var after int

	testing.ContextLogf(ctx, "Cooling down %v after log in", coolDownTimeAfterLogin)
	if err := testing.Sleep(ctx, coolDownTimeAfterLogin); err != nil {
		return errors.Wrap(err, "error while cooling down after log in")
	}

	before, err = readStableI915ObjectCount(ctx)
	if err != nil || before == 0 {
		return errors.Wrap(err, "failed to get the i915 framebuffer object count")
	}

	payload()

	after, err = readStableI915ObjectCount(ctx)
	if err != nil || before == 0 {
		return errors.Wrap(err, "failed to get the i915 framebuffer object count")
	}
	testing.ContextLogf(ctx, "i915 objects before=%d, after=%d, delta=%d", before, after, after-before)
	if before != after {
		return errors.Wrapf(err, "i915 objects before=%d, after=%d do not coincide! delta=%d", before, after, after-before)
	}
	return nil
}

// readStableI915ObjectCount waits until a given i915 graphics object count is
// stable, up to a certain timeout, progressively relaxing a similarity
// threshold criteria.
func readStableI915ObjectCount(ctx context.Context) (objectCount int, err error) {
	const (
		pollingInterval = 1 * time.Second
		// Time to wait for the object count to be stable.
		waitTimeout = 120 * time.Second
		// Threshold (in percentage) below which the object count is considered stable.
		objectCountThresholdBase = 0.1
		// Maximum threshold (in percentage) for the object count to be considered stable.
		objectCountThresholdMax = 2.0
		// Maximum steps of relaxing the object count similarity threshold.
		relaxingThresholdSteps = 5
	)

	startTime := time.Now()
	delta := (objectCountThresholdMax - objectCountThresholdBase) / (relaxingThresholdSteps - 1)
	testing.ContextLogf(ctx, "Waiting at most %v for stable graphics object count, threshold will be gradually relaxed from %.1f%% to %.1f%%",
		waitTimeout, objectCountThresholdBase, objectCountThresholdMax)

	for i := 0; i < relaxingThresholdSteps; i++ {
		idlePercent := objectCountThresholdBase + (delta * float64(i))
		timeout := waitTimeout / relaxingThresholdSteps
		testing.ContextLogf(ctx, "Waiting up to %v for object count to settle within %.1f%% (%d/%d)",
			timeout.Round(time.Second), idlePercent, i+1, relaxingThresholdSteps)

		objectCount, err = waitForStableReadings(ctx, readI915FramebufferCount, timeout, pollingInterval, idlePercent)
		if err == nil {
			testing.ContextLogf(ctx, "Waiting for object count stabilisation took %v (value %d, threshold: %.1f%%)",
				time.Now().Sub(startTime).Round(time.Second), objectCount, idlePercent)
			return objectCount, nil
		}
	}
	return objectCount, err
}

// readI915FramebufferCount tries to open the i915FramebufferFile and count
// the amount of lines, which corresponds to the amount of framebuffers
// allocated in the system. See https://dri.freedesktop.org/docs/drm/gpu/i915.html
func readI915FramebufferCount(ctx context.Context) (framebuffers int, err error) {
	f, err := os.Open(i915FramebufferFile)
	if err != nil {
		return framebuffers, errors.Wrap(err, "failed to open dri file")
	}
	text, err := ioutil.ReadAll(f)
	if err != nil {
		return framebuffers, errors.Wrap(err, "failed to read dri file")
	}
	framebuffers = len(strings.Split(string(text), "\n"))
	return
}

type readObjectCountFn func(ctx context.Context) (objects int, err error)

// waitForStableReadings reads values using readFn and waits for up to timeout
// for the moving average of the last numReadings to settle within threshold.
func waitForStableReadings(ctx context.Context, readFn readObjectCountFn, timeout time.Duration, interval time.Duration, threshold float64) (reading int, err error) {
	// Keep the last numReadings for moving average purposes. Make it half the
	// size that the current timeout and interval would allow.
	numReadings := int(math.Floor(float64(timeout / (2.0 * interval))))

	var values []int
	err = testing.Poll(ctx, func(ctx context.Context) error {
		var e error
		reading, e = readFn(ctx)
		if e != nil {
			return testing.PollBreak(errors.Wrap(e, "failed measuring"))
		}

		if len(values) >= numReadings {
			values = values[1:]
		}
		values = append(values, reading)
		if len(values) < numReadings {
			return errors.New("Need more values")
		}

		average := mean(values)

		if math.Abs(float64(reading)-average) > threshold {
			testing.ContextLogf(ctx, "Reading %d is not within %.1f of %.1f", reading, threshold, average)
			return errors.Errorf("Reading %d is not within %.1f of %.1f", reading, threshold, average)
		}
		testing.ContextLogf(ctx, "Reading %d is within %.1f of %.1f", reading, threshold, average)
		return nil
	}, &testing.PollOptions{Timeout: timeout, Interval: interval})
	return reading, err
}

// mean returns the average of values.
func mean(values []int) float64 {
	var sum float64
	for _, v := range values {
		sum += float64(v)
	}
	return sum / float64(len(values))
}
