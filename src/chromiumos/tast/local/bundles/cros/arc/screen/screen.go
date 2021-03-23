// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package screen provides helper functions to assist with verifying screen state in tast tests.
package screen

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/testing"
)

const (
	// KeyTotalFramesRendered key for total frames rendered.
	KeyTotalFramesRendered = "Total frames rendered"
	// KeyJankyFrames key for janky frames.
	KeyJankyFrames = "Janky frames"
)

var gfxinfoSamplesRe = regexp.MustCompile(
	fmt.Sprintf(
		`(%s): (?P<num_frames>\d+)\s+`+
			`(%s): (\d+) \((?:\d+\.\d+|-?nan)%%\)\s+`+
			`(?:50th percentile: \d+ms\s+)?`+
			`(90th percentile): (\d+)ms\s+`+
			`(95th percentile): (\d+)ms\s+`+
			`(99th percentile): (\d+)ms\s+`+
			`(Number Missed Vsync): (\d+)\s+`+
			`(Number High input latency): (\d+)\s+`+
			`(Number Slow UI thread): (\d+)\s+`+
			`(Number Slow bitmap uploads): (\d+)\s+`+
			`(Number Slow issue draw commands): (\d+)\s+`, KeyTotalFramesRendered, KeyJankyFrames))

// WaitForStableFrames waits until no new frames are captured by "dumpsys gfxinfo".
// This "wait" is needed to prevent "polluting" the next capture from frames that don't belong to
// the rotation.
func WaitForStableFrames(ctx context.Context, a *arc.ARC, pkgName string) error {
	var prevNumFrames int
	// testing.Poll is using an "Interval == 1 second" which is more than enough time to capture a
	// frame. If no frames are being captured during that time, it is safe to assume that the
	// activity is "idle" and won't trigger any new frame capture.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		stats, err := GfxinfoDumpStats(ctx, a, pkgName)
		if err != nil {
			return testing.PollBreak(err)
		}
		numFrames, ok := stats[KeyTotalFramesRendered]
		if !ok {
			return testing.PollBreak(errors.New("could not get numFrames"))
		}
		if numFrames != prevNumFrames {
			prevNumFrames = numFrames
			return errors.New("frames are still being captured")
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second, Interval: 1 * time.Second}); err != nil {
		return errors.Wrap(err, "failed to rotate device")
	}
	return nil
}

// GfxinfoDumpStats parses and returns the output of "dumpsys gfxinfo" using the global gfxinfoSampleRe regexp.
func GfxinfoDumpStats(ctx context.Context, a *arc.ARC, pkgName string) (map[string]int, error) {
	// Returning dumpsys text output as it doesn't support Protobuf.
	output, err := a.Command(ctx, "dumpsys", "gfxinfo", pkgName).Output(testexec.DumpLogOnError)
	if err != nil {
		return nil, errors.Wrap(err, "failed to launch dumpsys")
	}
	ss := string(output)
	stats := gfxinfoSamplesRe.FindStringSubmatch(ss)
	if stats == nil {
		testing.ContextLog(ctx, "Dumpsys output: ", ss)
		return nil, errors.New("failed to parse output")
	}
	if len(stats)%2 != 1 {
		return nil, errors.Errorf("length of stats is not even, got: %d", len(stats))
	}

	m := make(map[string]int)
	// Skip group 0 since it is the one that contains all the groups together.
	for i := 1; i < len(stats); i += 2 {
		k := stats[i]
		v, err := strconv.Atoi(stats[i+1])
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse output")
		}
		m[k] = v
	}
	return m, nil
}
