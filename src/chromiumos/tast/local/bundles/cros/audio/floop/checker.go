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

type volumeChecker struct {
	ti             TimeInterval
	checkVolume    func(dB float64) error
	timelineName   string
	timelineSymbol rune
}

func (c *volumeChecker) Check(ctx context.Context, s *testing.State, t *testFloopPlaybackCapture) {
	stats, err := getSoxStats(
		ctx, t.captureRaw,
		c.ti.StartSec-t.CaptureTime.StartSec, c.ti.EndSec-t.CaptureTime.StartSec,
	)
	if err != nil {
		s.Errorf("Failed to get sox stats from %s: %v", t.captureWav, err)
		return
	}

	err = c.checkVolume(stats.peakLevelDB[leftChannel])
	if err != nil {
		s.Errorf("Unexpected left dB in Δt=%s: %v", c.ti, err)
	}
	err = c.checkVolume(stats.peakLevelDB[rightChannel])
	if err != nil {
		s.Errorf("Unexpected right dB in Δt=%s: %v", c.ti, err)
	}
}

func (c *volumeChecker) maybeLogTimeline(ctx context.Context, t *testFloopPlaybackCapture) {
	t.logTimeline(ctx, c.timelineName, c.timelineSymbol, c.ti)
}

// CheckZeroSample checks that the audio captured in the specified time
// has zero samples
func CheckZeroSample(startSec, endSec int) Checker {
	return &volumeChecker{
		ti:             TimeInterval{startSec, endSec},
		checkVolume:    checkVolumeNegativeInf,
		timelineName:   "check zero",
		timelineSymbol: '0',
	}
}

func checkVolumeNegativeInf(dB float64) error {
	if !math.IsInf(dB, -1) {
		return errors.Errorf("want %f, got %f", math.Inf(-1), dB)
	}
	return nil
}

// CheckCaptureSample checks the audio captured in the specified time
// is -20dB
func CheckCaptureSample(startSec, endSec int) Checker {
	return &volumeChecker{
		ti:             TimeInterval{startSec, endSec},
		checkVolume:    checkVolumeMatchesPlayback,
		timelineName:   "check volume",
		timelineSymbol: '1',
	}
}

func checkVolumeMatchesPlayback(dB float64) error {
	if dB != expectedPeakLevelDB {
		return errors.Errorf("want %f, got %f", expectedPeakLevelDB, dB)
	}
	return nil
}

const (
	leftChannel = iota
	rightChannel
	numChannels
)

// soxStats is parsed result from `sox -n stats`
type soxStats struct {
	peakLevelDB [numChannels]float64 // "Pk lev dB"
	lengthSec   time.Duration        // "Length s"
}

func getSoxStats(ctx context.Context, file string, startSec, endSec int) (stats soxStats, err error) {
	cmd := testexec.CommandContext(
		ctx,
		"sox",
		"-t", "s16", "-r", "48000", "-c", "2",
		file,
		"-n",
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

	stats.peakLevelDB[leftChannel], err = strconv.ParseFloat(string(m[1]), 64)
	if err != nil {
		return
	}
	stats.peakLevelDB[rightChannel], err = strconv.ParseFloat(string(m[2]), 64)
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
	stats.lengthSec = time.Duration(lengthFloat * float64(time.Second))
	return
}
