// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package audio

import (
	"context"
	"time"

	perfpkg "chromiumos/tast/common/perf"
	"chromiumos/tast/local/audio"
	"chromiumos/tast/local/bundles/cros/audio/crastestclient"
	"chromiumos/tast/local/bundles/cros/audio/device"
	"chromiumos/tast/local/profiler"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

// TestParameters contains all the data needed to run a single test iteration.
type testParameters struct {
	Playback bool
	Capture  bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func:     CrasPerf,
		Desc:     "Performance measurement of CRAS",
		Contacts: []string{"yuhsuan@chromium.org", "cychiang@chromium.org", "paulhsia@chromium.org"},
		Attr:     []string{"group:crosbolt", "crosbolt_perbuild"},
		Timeout:  5 * time.Minute,
		Params: []testing.Param{
			{
				Name: "playback",
				Val: testParameters{
					Playback: true,
					Capture:  false,
				},
				ExtraSoftwareDeps: []string{"audio_play"},
			},
			{
				Name: "capture",
				Val: testParameters{
					Playback: false,
					Capture:  true,
				},
				ExtraSoftwareDeps: []string{"audio_record"},
			},
			{
				Name: "playback_capture",
				Val: testParameters{
					Playback: true,
					Capture:  true,
				},
				ExtraSoftwareDeps: []string{"audio_play", "audio_record"},
			},
		},
	})
}

func crasPerfOneIteration(ctx context.Context, s *testing.State, pid int, pv *perfpkg.Values) {
	const (
		getDeviceTimeout = 3 * time.Second
		blocksize        = 480
		topInterval      = 1 * time.Second
		perfDuration     = 10 * time.Second              // Duration to run perf command.
		commandDuration  = perfDuration + 2*time.Second  // Duration to run audio command
		contextDuration  = perfDuration + 15*time.Second // Upper bound for one iteration.
	)

	param := s.Param().(testParameters)

	runCtx, cancel := context.WithTimeout(ctx, contextDuration)
	defer cancel()

	var out profiler.PerfStatOutput

	profs := []profiler.Profiler{
		profiler.Top(&profiler.TopOpts{
			Interval: topInterval,
		}),
		profiler.Perf(profiler.PerfStatOpts(&out, pid)),
		profiler.Perf(profiler.PerfRecordOpts()),
	}

	s.Log("start audio")
	playbackCommand := crastestclient.CRASPlaybackCommand(runCtx, (int64)(commandDuration.Seconds()), blocksize)
	captureCommand := crastestclient.CRASCaptureCommand(runCtx, (int64)(commandDuration.Seconds()), blocksize)

	if param.Playback {
		playbackCommand.Start()
	}

	if param.Capture {
		captureCommand.Start()
	}

	// Wait one second for audio processing to be stable.
	// TODO(b/165995912) remove the sleep once we can query
	// stream state from CRAS.
	if err := testing.Sleep(ctx, 1*time.Second); err != nil {
		s.Fatal("Timed out on sleep: ", err)
	}

	runningProfs, err := profiler.Start(ctx, s.OutDir(), profs...)
	if err != nil {
		s.Fatal("Failure in starting the profiler: ", err)
	}

	checkRunningDevice := func(ctx context.Context) error {
		return device.CheckRunningDevice(ctx, bool(param.Playback), bool(param.Capture))
	}
	if err := testing.Poll(ctx, checkRunningDevice, &testing.PollOptions{Timeout: getDeviceTimeout}); err != nil {
		s.Fatal("Failed to detect running device: ", err)
	}

	defer func() {
		// The perf value is stored when ending the profiler.
		if err := runningProfs.End(); err != nil {
			s.Error("Failure in ending the profiler: ", err)
		} else {
			// Append one measurement to PerfValue.
			pv.Append(perfpkg.Metric{
				Name:      "cras_cycles_per_second",
				Unit:      "cycles",
				Direction: perfpkg.SmallerIsBetter,
				Multiple:  true,
			}, out.CyclesPerSecond)
		}

		if param.Playback {
			if err := playbackCommand.Wait(); err != nil {
				s.Error("Playback did not finish in time: ", err)
			}
		}

		if param.Capture {
			if err := captureCommand.Wait(); err != nil {
				s.Error("Capture did not finish in time: ", err)
			}
		}

		s.Log("Finished one iteration")
	}()

	// Record for perfDuration seconds.
	// This is to make sure that audio is being used during whole
	// perf recording.
	if err := testing.Sleep(ctx, perfDuration); err != nil {
		s.Fatal("Timed out on sleep: ", err)
	}

}

func CrasPerf(ctx context.Context, s *testing.State) {
	const (
		iterations = 10
	)

	// TODO(crbug.com/996728): aarch64 is disabled before the kernel crash is fixed.
	if u, err := sysutil.Uname(); err != nil {
		s.Fatal("Failed to get uname: ", err)
	} else if u.Machine == "aarch64" {
		s.Log("Can not run perf on aarch64 machine")
		return
	}

	// Use this perf value to hold CPU cycles per second spent in CRAS of each iteration.
	pv := perfpkg.NewValues()

	for i := 0; i < iterations; i++ {
		s.Log("Iteration: ", i)

		// Stop CRAS to make sure the audio device won't be occupied.
		s.Log("Restarting CRAS")

		if err := upstart.RestartJob(ctx, "cras"); err != nil {
			s.Fatal("Failed to stop CRAS: ", err)
		}

		// Any device being available means CRAS is ready.
		if err := audio.WaitForDevice(ctx, audio.OutputStream|audio.InputStream); err != nil {
			s.Fatal("Failed to wait for any output or input device: ", err)
		}

		pid, err := audio.GetCRASPID()
		s.Log("get PID done: ", pid)

		if err != nil {
			s.Fatal("Failed to find PID of cras: ", err)
		}

		crasPerfOneIteration(ctx, s, pid, pv)
	}

	if err := pv.Save(s.OutDir()); err != nil {
		s.Fatal("Cannot save perf data: ", err)
	}
}
