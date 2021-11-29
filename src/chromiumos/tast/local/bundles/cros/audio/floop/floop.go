// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package floop tests the flexible loopback device
package floop

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/audio"
	"chromiumos/tast/testing"
)

const (
	rate            = 48000
	channels        = 2
	volume          = 0.1
	expectedPkLevDB = -20.
)

// TI is a shorthand to create TimeIntervals
func TI(startSec, endSec int) TimeInterval {
	return TimeInterval{StartSec: startSec, EndSec: endSec}
}

// TimeInterval specifies when a event should happen in the test timeline
type TimeInterval struct {
	StartSec int
	EndSec   int
}

var zeroInterval TimeInterval

// DurationSec returns the length of the TimeInterval in seconds
func (ti *TimeInterval) DurationSec() int {
	return ti.EndSec - ti.StartSec
}

// Param for testFloopPlaybackCapture
type Param struct {
	// relative time to start/end playback/capture
	// if set to the zero value (start & time are both zero), playback/capture is skipped
	PlaybackTime TimeInterval
	CaptureTime  TimeInterval

	// time when flexible loopback device is requested
	RequestFloopSec int

	// client type bitmask
	FloopMask int

	Checkers []Checker
}

// TestFloopPlaybackCapture configures a flexible loopback device, plays synth audio
// captures using the flexible loopback device.
// It also checks captured samples if specified.
func TestFloopPlaybackCapture(ctx context.Context, s *testing.State, p *Param) {
	t := &testFloopPlaybackCapture{Param: p}
	t.Run(ctx, s)
}

type testFloopPlaybackCapture struct {
	*Param
	t0              time.Time // start time
	wg              *sync.WaitGroup
	playbackRaw     string
	captureRaw      string
	playbackWav     string
	captureWav      string
	testDurationSec int
}

func (t *testFloopPlaybackCapture) Run(ctx context.Context, s *testing.State) {
	err := audio.RestartCras(ctx)
	if err != nil {
		s.Fatal("Failed to restart CRAS: ", err)
	}

	t.playbackRaw = filepath.Join(s.OutDir(), "playback.raw")
	t.playbackWav = filepath.Join(s.OutDir(), "playback.wav")
	t.captureRaw = filepath.Join(s.OutDir(), "capture.raw")
	t.captureWav = filepath.Join(s.OutDir(), "capture.wav")

	t.testDurationSec = t.CaptureTime.EndSec
	if t.PlaybackTime.EndSec > t.testDurationSec {
		t.testDurationSec = t.PlaybackTime.EndSec
	}

	testing.ContextLog(ctx, "scheduled test timeline:")
	t.logTimeline(ctx, "floop active", 'f', TI(t.RequestFloopSec, t.testDurationSec))
	t.logTimeline(ctx, "capture", 'c', t.CaptureTime)
	t.logTimeline(ctx, "playback", 'p', t.PlaybackTime)
	for _, c := range t.Checkers {
		c.maybeLogTimeline(ctx, t)
	}

	if t.PlaybackTime != zeroInterval {
		// prepare playback sample
		err = audio.GenerateTestRawData(ctx, audio.TestRawData{
			Path:          t.playbackRaw,
			BitsPerSample: 16,
			Channels:      channels,
			Rate:          rate,
			Frequencies:   []int{440, 440},
			Volume:        volume,
			Duration:      t.PlaybackTime.DurationSec(),
		})
		if err != nil {
			s.Fatal("Failed to generate test audio data: ", err)
		}
	}

	t.wg = &sync.WaitGroup{}
	t.wg.Add(2)
	t.t0 = time.Now()

	go t.runCapture(ctx, s)
	go t.runPlayback(ctx, s)

	// requestFloopMask changes the state of CRAS, restart it to avoid corruption
	defer audio.RestartCras(ctx)

	t.wg.Wait()

	err = audio.ConvertRawToWav(ctx, t.playbackRaw, t.playbackWav, rate, channels)
	if err != nil {
		s.Error("raw -> wav convertion failed: ", err)
	}
	if t.CaptureTime != zeroInterval {
		err = audio.ConvertRawToWav(ctx, t.captureRaw, t.captureWav, rate, channels)
		if err != nil {
			s.Error("raw -> wav convertion failed: ", err)
		}
	}

	for _, c := range t.Checkers {
		c.Check(ctx, s, t)
	}
}

func (t *testFloopPlaybackCapture) logTimeline(ctx context.Context, name string, c rune, ti TimeInterval) {
	b := strings.Builder{}
	i := 0
	for ; i < ti.StartSec; i++ {
		b.WriteRune('-')
	}
	for ; i < ti.EndSec; i++ {
		b.WriteRune(c)
	}
	for ; i < t.testDurationSec; i++ {
		b.WriteRune('-')
	}
	testing.ContextLogf(ctx, "%15s: %s", name, b.String())
}

// sleepUntil the specified time in seconds has passed since the test started
func (t *testFloopPlaybackCapture) sleepUntil(ctx context.Context, second int) {
	testing.Sleep(
		ctx,
		t.t0.Add(time.Duration(second)*time.Second).
			Sub(time.Now()),
	)
}

func (t *testFloopPlaybackCapture) runCapture(ctx context.Context, s *testing.State) {
	defer t.wg.Done()

	t.sleepUntil(ctx, t.RequestFloopSec)
	t.timestamp(ctx, "request floop")
	floopDev, err := requestFloopMask(ctx, t.FloopMask)
	if err != nil {
		s.Fatal("Failed to request flexible loopback: ", err)
	}

	if t.CaptureTime == zeroInterval {
		// no capture
		return
	}

	t.sleepUntil(ctx, t.CaptureTime.EndSec)
	t.timestamp(ctx, "start capture")
	captureCtx, cancel := context.WithDeadline(
		ctx,
		t.t0.Add(time.Duration(t.CaptureTime.StartSec)*time.Second+500*time.Millisecond),
	)
	defer cancel()
	cmd := testexec.CommandContext(
		captureCtx,
		"cras_test_client",
		fmt.Sprintf("--capture_file=%s", filepath.Join(s.OutDir(), "capture.raw")),
		fmt.Sprintf("--duration_seconds=%d", t.CaptureTime.DurationSec()),
		fmt.Sprintf("--pin_device=%d", floopDev),
	)
	err = cmd.Run()
	t.timestamp(ctx, "end capture")
	if err != nil {
		s.Fatal("capture failed: ", err)
	}
}

func (t *testFloopPlaybackCapture) runPlayback(ctx context.Context, s *testing.State) {
	defer t.wg.Done()

	if t.PlaybackTime == zeroInterval {
		// no playback
		return
	}

	t.sleepUntil(ctx, t.PlaybackTime.StartSec)
	t.timestamp(ctx, "start playback")
	playbackCtx, cancel := context.WithDeadline(
		ctx,
		t.t0.Add(time.Duration(t.PlaybackTime.EndSec)*time.Second+500*time.Millisecond),
	)
	defer cancel()
	cmd := testexec.CommandContext(
		playbackCtx,
		"cras_test_client",
		fmt.Sprintf("--playback_file=%s", filepath.Join(s.OutDir(), "playback.raw")),
	)
	err := cmd.Run()
	t.timestamp(ctx, "end playback")
	if err != nil {
		s.Fatal("playback failed: ", err)
	}
}

func (t *testFloopPlaybackCapture) timestamp(ctx context.Context, msg string) {
	testing.ContextLogf(ctx, "Δt = %s: %s", time.Now().Sub(t.t0), msg)
}

func requestFloopMask(ctx context.Context, mask int) (dev int, err error) {
	var buf bytes.Buffer
	cmd := testexec.CommandContext(
		ctx,
		"cras_test_client",
		fmt.Sprintf("--request_floop_mask=%d", mask),
	)
	cmd.Stdout = &buf
	err = cmd.Run()
	if err != nil {
		return 0, err
	}

	re := regexp.MustCompile(`flexible loopback dev id: (\d+)`)
	m := re.FindStringSubmatch(buf.String())
	if m == nil {
		return -1, errors.Errorf("output %q not matching %q", buf.String(), re)
	}
	return strconv.Atoi(m[1])
}
