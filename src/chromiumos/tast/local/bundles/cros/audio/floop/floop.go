// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package floop tests the flexible loopback device
package floop

import (
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
	rate     = 48000
	channels = 2

	// sox -n -t s16 -r 48000 /dev/null synth sine 440 vol 0.05 trim 0 10 stats
	volume              = 0.05
	expectedRMSLevelDB  = -29.03
	rmsLevelDBTolerance = 0.3

	// events happen within this time window will be treated as on schedule
	// used for logging only, does not cause tests to fail
	logTimingTolerance = 200 * time.Millisecond

	// allow a command to exceed this amount of time
	extraTimeout = time.Second
)

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
	cras, err := audio.RestartCras(ctx)
	if err != nil {
		s.Fatal("Failed to restart CRAS: ", err)
	}

	var node *audio.CrasNode
	err = testing.Poll(
		ctx,
		func(ctx context.Context) (err error) {
			node, err = cras.SelectedOutputNode(ctx)
			return err
		},
		&testing.PollOptions{
			Timeout:  3 * time.Second,
			Interval: 500 * time.Millisecond,
		},
	)
	if err != nil {
		s.Fatal("SelectedOutputNode failed: ", err)
	}

	// To avoid making noise during test
	err = cras.SetOutputNodeVolume(ctx, *node, 0)
	if err != nil {
		s.Fatal("SetOutputNodeVolume failed: ", err)
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
	t.logTimelineHeader(ctx)
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

	if t.PlaybackTime != zeroInterval {
		err = audio.ConvertRawToWav(ctx, t.playbackRaw, t.playbackWav, rate, channels)
		if err != nil {
			s.Error("raw -> wav convertion failed: ", err)
		}
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

func (t *testFloopPlaybackCapture) logTimelineHeader(ctx context.Context) {
	b := strings.Builder{}
	for i := 0; i < t.testDurationSec; i++ {
		b.WriteRune('0' + rune(i%10))
	}
	testing.ContextLogf(ctx, "%15s: %s", "--- time -->", b.String())
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

func (t *testFloopPlaybackCapture) logEvent(ctx context.Context, targetSec int, msg string, sleep bool) {
	if sleep {
		testing.Sleep(
			ctx,
			t.t0.Add(time.Duration(targetSec)*time.Second).
				Sub(time.Now()),
		)
	}
	dt := time.Now().Sub(t.t0)
	overdue := dt - time.Duration(targetSec)*time.Second
	overdueMsg := "on schedule"
	if overdue > logTimingTolerance {
		overdueMsg = fmt.Sprintf("%s overdue", overdue)
	} else if overdue < -logTimingTolerance {
		overdueMsg = fmt.Sprintf("%s early", overdue)
	}
	testing.ContextLogf(ctx, "Δt = %s: %s (%s)", dt, msg, overdueMsg)
}

func (t *testFloopPlaybackCapture) runCapture(ctx context.Context, s *testing.State) {
	defer t.wg.Done()

	t.logEvent(ctx, t.RequestFloopSec, "request floop", true)
	floopDev, err := requestFloopMask(ctx, t.FloopMask)
	if err != nil {
		s.Fatal("Failed to request flexible loopback: ", err)
	}
	testing.ContextLog(ctx, "floop device id: ", floopDev)

	if t.CaptureTime == zeroInterval {
		// no capture
		return
	}

	t.logEvent(ctx, t.CaptureTime.StartSec, "start capture", true)
	captureCtx, cancel := context.WithDeadline(
		ctx,
		t.t0.Add(time.Duration(t.CaptureTime.EndSec)*time.Second+extraTimeout),
	)
	defer cancel()
	cmd := testexec.CommandContext(
		captureCtx,
		"cras_test_client",
		fmt.Sprintf("--capture_file=%s", t.captureRaw),
		fmt.Sprintf("--duration_seconds=%d", t.CaptureTime.DurationSec()),
		fmt.Sprintf("--pin_device=%d", floopDev),
	)
	err = cmd.Run()
	t.logEvent(ctx, t.CaptureTime.EndSec, "end capture", false)
	if err != nil {
		s.Fatal("Capture failed: ", err)
	}
}

func (t *testFloopPlaybackCapture) runPlayback(ctx context.Context, s *testing.State) {
	defer t.wg.Done()

	if t.PlaybackTime == zeroInterval {
		// no playback
		return
	}

	t.logEvent(ctx, t.PlaybackTime.StartSec, "start playback", true)
	playbackCtx, cancel := context.WithDeadline(
		ctx,
		t.t0.Add(time.Duration(t.PlaybackTime.EndSec)*time.Second+extraTimeout),
	)
	defer cancel()
	cmd := testexec.CommandContext(
		playbackCtx,
		"cras_test_client",
		fmt.Sprintf("--playback_file=%s", t.playbackRaw),
	)
	err := cmd.Run()
	t.logEvent(ctx, t.PlaybackTime.EndSec, "end playback", false)
	if err != nil {
		s.Fatal("Playback failed: ", err)
	}
}

func requestFloopMask(ctx context.Context, mask int) (dev int, err error) {
	cmd := testexec.CommandContext(
		ctx,
		"cras_test_client",
		fmt.Sprintf("--request_floop_mask=%d", mask),
	)
	stdout, _, err := cmd.SeparatedOutput()
	if err != nil {
		return 0, err
	}

	re := regexp.MustCompile(`flexible loopback dev id: (\d+)`)
	m := re.FindSubmatch(stdout)
	if m == nil {
		return -1, errors.Errorf("output %q not matching %q", string(stdout), re)
	}
	return strconv.Atoi(string(m[1]))
}
