// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package graphics contains graphics-related utility functions for local tests.
package graphics

import (
	"context"
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"strings"
	"sync"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

const (
	// The debugfs file with the information on allocated framebuffers.
	i915FramebufferFile = "/sys/kernel/debug/dri/0/i915_gem_framebuffer"
	// Immediately after login there's a lot of graphics activity; wait for a
	// minute until it subsides. TODO(crbug.com/1047840): Remove when not needed.
	coolDownTimeAfterLogin = 30 * time.Second
)

// Backend contains the necessary methods to interact with the platform
// debug interface: checking if it's supported and getting readings.
type Backend interface {
	// SupportsFramebufferInfo returns true if the platform supports graphics
	// memory allocation debugging and it's available.
	SupportsFramebufferInfo() bool

	// Round implements the platform-specific graphic- or codec- rounding.
	Round(value int) int

	// ReadFramebufferCount tries to retrieve the number of framebuffers of width
	// and height dimensions allocated by the Backend.
	ReadFramebufferCount(ctx context.Context, width, height int) (framebuffers int, err error)
}

// I915Backend implements Backend for the Intel i915 case.
type I915Backend struct{}

// SupportsFramebufferInfo returns true if i915FramebufferFile exists.
func (g I915Backend) SupportsFramebufferInfo() bool {
	_, err := os.Stat(i915FramebufferFile)
	return err == nil
}

// Round rounds up value for the Intel platforms and all codecs.
func (g I915Backend) Round(value int) int {
	const i915Alignment = 16
	// Inspired by Chromium's base/bits.h:Align() function.
	return (value + i915Alignment - 1) & ^(i915Alignment - 1)
}

// ReadFramebufferCount tries to open the i915FramebufferFile and count the
// amount of lines of dimensions width x height, which corresponds to the amount
// of framebuffers allocated in the system.
// See https://dri.freedesktop.org/docs/drm/gpu/i915.html
func (g I915Backend) ReadFramebufferCount(ctx context.Context, width, height int) (framebuffers int, e error) {
	f, err := os.Open(i915FramebufferFile)
	if err != nil {
		return framebuffers, errors.Wrap(err, "failed to open dri file")
	}
	text, err := ioutil.ReadAll(f)
	if err != nil {
		return framebuffers, errors.Wrap(err, "failed to read dri file")
	}
	lines := strings.Split(string(text), "\n")
	for _, line := range lines {
		// The line we're looking for looks like "user size: 1920 x 1080,..."
		var fbWidth, fbHeight int
		if _, err := fmt.Sscanf(line, "user size: %d x %d", &fbWidth, &fbHeight); err != nil {
			continue
		}
		if fbWidth == width && fbHeight == height {
			framebuffers++
		}
	}
	return
}

// CompareGraphicsMemoryBeforeAfter compares the graphics memory consumption
// before and after running the payload function, using the backend. The amount
// of graphics buffer during payload execution must also be non-zero.
func CompareGraphicsMemoryBeforeAfter(ctx context.Context, payload func(), backend Backend, width, height int) (err error) {
	var before, during, after int
	roundedWidth := backend.Round(width)
	roundedHeight := backend.Round(height)

	testing.ContextLogf(ctx, "Cooling down %v after log in", coolDownTimeAfterLogin)
	if err := testing.Sleep(ctx, coolDownTimeAfterLogin); err != nil {
		return errors.Wrap(err, "error while cooling down after log in")
	}

	if before, err = readStableObjectCount(ctx, backend, roundedWidth, roundedHeight); err != nil {
		return errors.Wrap(err, "failed to get the framebuffer object count")
	}

	testing.ContextLog(ctx, "Running the payload() and measuring the number of graphics objects during its execution")
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		payload()
		wg.Done()
	}()
	go func() {
		const pollTimeout = 10 * time.Second
		const pollInterval = 100 * time.Millisecond
		_ = testing.Poll(ctx, func(ctx context.Context) error {
			if during, _ = backend.ReadFramebufferCount(ctx, roundedWidth, roundedHeight); during == 0 {
				return errors.New("Still waiting for graphics objects")
			}
			return nil
		}, &testing.PollOptions{Timeout: pollTimeout, Interval: pollInterval})
	}()
	wg.Wait()

	if after, err = readStableObjectCount(ctx, backend, roundedWidth, roundedHeight); err != nil {
		return errors.Wrap(err, "failed to get the framebuffer object count")
	}
	if before != after || during == 0 {
		return errors.Wrapf(err, "graphics objects of size %d x %d before=%d, during=%d, after=%d", roundedWidth, roundedHeight, before, during, after)
	}
	testing.ContextLogf(ctx, "Graphics objects of size %d x %d before=%d, during=%d, after=%d", roundedWidth, roundedHeight, before, during, after)
	return nil
}

// readStableObjectCount waits until a given graphics object count obtained with
// backend is stable, up to a certain timeout, progressively relaxing a
// similarity threshold criteria.
func readStableObjectCount(ctx context.Context, backend Backend, width, height int) (objectCount int, err error) {
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

		objectCount, err = waitForStableReadings(ctx, backend, width, height, timeout, pollingInterval, idlePercent)
		if err == nil {
			testing.ContextLogf(ctx, "Waiting for object count stabilisation took %v (value %d, threshold: %.1f%%)",
				time.Now().Sub(startTime).Round(time.Second), objectCount, idlePercent)
			return objectCount, nil
		}
	}
	return objectCount, err
}

// waitForStableReadings reads values using backend and waits for up to timeout
// for the moving average of the last numReadings to settle within threshold.
func waitForStableReadings(ctx context.Context, backend Backend, width, height int, timeout, interval time.Duration, threshold float64) (reading int, err error) {
	// Keep the last numReadings for moving average purposes. Make it half the
	// size that the current timeout and interval would allow.
	numReadings := int(math.Floor(float64(timeout / (2.0 * interval))))

	var currentNumReadings int
	var values = make([]int, numReadings)

	err = testing.Poll(ctx, func(ctx context.Context) error {
		var e error
		reading, e = backend.ReadFramebufferCount(ctx, width, height)
		if e != nil {
			return testing.PollBreak(errors.Wrap(e, "failed measuring"))
		}
		values[currentNumReadings%numReadings] = reading
		currentNumReadings++
		if currentNumReadings < numReadings {
			return errors.Errorf("need more values (have: %d and want: %d)", currentNumReadings, numReadings)
		}
		average := mean(values)

		if math.Abs(float64(reading)-average) > threshold {
			testing.ContextLogf(ctx, "reading %d is not within %.1f of %.1f", reading, threshold, average)
			return errors.Errorf("reading %d is not within %.1f of %.1f", reading, threshold, average)
		}
		testing.ContextLogf(ctx, "reading %d is within %.1f of %.1f", reading, threshold, average)
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
