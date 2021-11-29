// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package lifecycle tests loopback device lifecycles
package lifecycle

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

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

// Param for TestLifecycle
type Param struct {
	Playback     PlaybackAction // Action which plays the generated t.playbackRaw
	Capture      CaptureAction  // Action which captures audio to t.captureRaw
	ExtraActions []Action
	Checks       []Checker // Checks to perform on the captured file
}

// TestLifecycle configures a loopback device, plays synth audio
// captures using the flexible loopback device.
// It also checks captured samples if specified.
func TestLifecycle(ctx context.Context, s *testing.State, p *Param) {
	t := &tester{Param: p}
	t.Run(ctx, s)
}

type tester struct {
	*Param
	t0              time.Time // start time
	wg              *sync.WaitGroup
	playbackRaw     string
	captureRaw      string
	playbackWav     string
	captureWav      string
	testDurationSec int
}

func (t *tester) actions() []Action {
	var as []Action
	if t.Capture != nil {
		as = append(as, t.Capture)
	}
	if t.Playback != nil {
		as = append(as, t.Playback)
	}
	return append(as, t.ExtraActions...)
}

func (t *tester) Run(ctx context.Context, s *testing.State) {
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

	// Mute to avoid making noise during test
	err = cras.SetOutputNodeVolume(ctx, *node, 0)
	if err != nil {
		s.Fatal("SetOutputNodeVolume failed: ", err)
	}

	t.playbackRaw = filepath.Join(s.OutDir(), "playback.raw")
	t.playbackWav = filepath.Join(s.OutDir(), "playback.wav")
	t.captureRaw = filepath.Join(s.OutDir(), "capture.raw")
	t.captureWav = filepath.Join(s.OutDir(), "capture.wav")

	t.testDurationSec = 0
	for _, tl := range t.actions() {
		if tl != nil {
			endSec := tl.getSchedule().endSec
			if t.testDurationSec < endSec {
				t.testDurationSec = endSec
			}
		}
	}

	testing.ContextLog(ctx, "scheduled test timeline:")
	t.logTimelineHeader(ctx)
	for _, a := range t.actions() {
		a.maybeLogSchedule(ctx, t)
	}
	for _, c := range t.Checks {
		c.maybeLogSchedule(ctx, t)
	}

	if t.Playback != nil {
		// prepare playback sample
		err = audio.GenerateTestRawData(ctx, audio.TestRawData{
			Path:          t.playbackRaw,
			BitsPerSample: 16,
			Channels:      channels,
			Rate:          rate,
			Frequencies:   []int{440, 440},
			Volume:        volume,
			Duration:      t.Playback.durationSec(),
		})
		if err != nil {
			s.Fatal("Failed to generate test audio data: ", err)
		}
	}

	t.wg = &sync.WaitGroup{}
	t.wg.Add(len(t.actions()))
	t.t0 = time.Now()

	// Do playback & capture
	for _, a := range t.actions() {
		go func(a Action) {
			defer t.wg.Done()
			if a != nil {
				a.Do(ctx, s, t)
			}
		}(a)
	}

	// requestFloopMask changes the state of CRAS, restart it to avoid corruption
	defer audio.RestartCras(ctx)

	t.wg.Wait()

	if t.Playback != nil {
		err = audio.ConvertRawToWav(ctx, t.playbackRaw, t.playbackWav, rate, channels)
		if err != nil {
			s.Error("raw -> wav convertion failed: ", err)
		}
	}
	if t.Capture != nil {
		err = audio.ConvertRawToWav(ctx, t.captureRaw, t.captureWav, rate, channels)
		if err != nil {
			s.Error("raw -> wav convertion failed: ", err)
		}
	}

	for _, c := range t.Checks {
		c.Check(ctx, s, t)
	}
}

func (t *tester) logTimelineHeader(ctx context.Context) {
	b := strings.Builder{}
	for i := 0; i < t.testDurationSec; i++ {
		b.WriteRune('0' + rune(i%10))
	}
	testing.ContextLogf(ctx, "%15s: %s", "--- time -->", b.String())
}

func (t *tester) logScheduleRow(ctx context.Context, name string, c rune, s schedule) {
	b := strings.Builder{}
	i := 0
	for ; i < s.startSec; i++ {
		b.WriteRune('-')
	}
	for ; i < s.endSec; i++ {
		b.WriteRune(c)
	}
	for ; i < t.testDurationSec; i++ {
		b.WriteRune('-')
	}
	testing.ContextLogf(ctx, "%15s: %s", name, b.String())
}

func (t *tester) logEvent(ctx context.Context, targetSec int, msg string, sleep bool) {
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
