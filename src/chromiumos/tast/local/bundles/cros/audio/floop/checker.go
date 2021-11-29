// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package floop

import (
	"context"
	"math"
	"regexp"
	"strconv"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// Checker checks the result of a TestFloopPlaybackCapture run
type Checker interface {
	Check(ctx context.Context, s *testing.State, t *testFloopPlaybackCapture)
	maybeLogTimeline(ctx context.Context, t *testFloopPlaybackCapture)
}

type zeroSampleChecker struct {
	TimeInterval
}

func (c *zeroSampleChecker) Check(ctx context.Context, s *testing.State, t *testFloopPlaybackCapture) {
	stats, err := getSoxStats(
		ctx, t.captureWav,
		c.StartSec-t.CaptureTime.StartSec, c.EndSec-t.CaptureTime.StartSec,
	)
	if err != nil {
		s.Errorf("Failed to get sox stats from %s: %v", t.captureWav, err)
		return
	}

	if !math.IsInf(stats.pkLevDB[leftChannel], -1) {
		s.Errorf("Expected left pk lev dB to be -inf, got %f", stats.pkLevDB[leftChannel])
	}
	if !math.IsInf(stats.pkLevDB[rightChannel], -1) {
		s.Errorf("Expected right pk lev dB to be -inf, got %f", stats.pkLevDB[rightChannel])
	}
}

func (c *zeroSampleChecker) maybeLogTimeline(ctx context.Context, t *testFloopPlaybackCapture) {
	t.logTimeline(ctx, "check zero", '0', c.TimeInterval)
}

// CheckZeroSample checks that the audio captured in the specified time
// has zero samples
func CheckZeroSample(startSec, endSec int) Checker {
	return &zeroSampleChecker{TimeInterval{startSec, endSec}}
}

type captureSampleChecker struct {
	TimeInterval
}

func (c *captureSampleChecker) Check(ctx context.Context, s *testing.State, t *testFloopPlaybackCapture) {
	stats, err := getSoxStats(
		ctx, t.captureWav,
		c.StartSec-t.CaptureTime.StartSec, c.EndSec-t.CaptureTime.StartSec,
	)
	if err != nil {
		s.Errorf("Failed to get sox stats from %s: %v", t.captureWav, err)
		return
	}

	if stats.pkLevDB[leftChannel] != expectedPkLevDB {
		s.Errorf("Expected left pk lev dB to be %f, got %f", expectedPkLevDB, stats.pkLevDB[leftChannel])
	}
	if stats.pkLevDB[rightChannel] != expectedPkLevDB {
		s.Errorf("Expected right pk lev dB to be %f, got %f", expectedPkLevDB, stats.pkLevDB[rightChannel])
	}
}

func (c *captureSampleChecker) maybeLogTimeline(ctx context.Context, t *testFloopPlaybackCapture) {
	t.logTimeline(ctx, "check capture", '1', c.TimeInterval)
}

// CheckCaptureSample checks the audio captured in the specified time
// is -20dB
func CheckCaptureSample(startSec, endSec int) Checker {
	return &captureSampleChecker{TimeInterval{startSec, endSec}}
}

const (
	leftChannel = iota
	rightChannel
	numChannels
)

// soxStats is parsed result from `sox -n stats`
type soxStats struct {
	pkLevDB [numChannels]float64 // "Pk lev dB"
	length  time.Duration        // "Length s"
}

func getSoxStats(ctx context.Context, file string, startSec, endSec int) (stats soxStats, err error) {
	cmd := testexec.CommandContext(
		ctx,
		"sox", file, "-n",
		"trim", strconv.Itoa(startSec), strconv.Itoa(endSec),
		"stats",
	)
	_, stderr, err := cmd.SeparatedOutput()
	if err != nil {
		testing.ContextLog(ctx, "sox command failed: ", err)
		return
	}

	m := regexp.MustCompile(`Pk lev dB +\S+ +(\S+) +(\S+)`).FindSubmatch(stderr)
	if m == nil {
		err = errors.New("cannot find `Pk lev dB` in sox stats")
		return
	}

	stats.pkLevDB[leftChannel], err = strconv.ParseFloat(string(m[1]), 64)
	if err != nil {
		return
	}
	stats.pkLevDB[rightChannel], err = strconv.ParseFloat(string(m[2]), 64)
	if err != nil {
		return
	}

	m = regexp.MustCompile(`Length s +(\S+)`).FindSubmatch(stderr)
	if m == nil {
		err = errors.New("cannt find `Length s` in sox stats")
		return
	}
	lengthFloat, err := strconv.ParseFloat(string(m[1]), 64)
	if err != nil {
		return
	}
	stats.length = time.Duration(lengthFloat * float64(time.Second))
	return
}
