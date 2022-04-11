// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package audio

import (
	"context"
	"time"

	perfpkg "chromiumos/tast/common/perf"
	"chromiumos/tast/local/audio"
	"chromiumos/tast/local/audio/crastestclient"
	"chromiumos/tast/local/bundles/cros/audio/device"
	"chromiumos/tast/local/procutil"
	"chromiumos/tast/local/profiler"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
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
				ExtraHardwareDeps: hwdep.D(hwdep.Speaker()),
			},
			{
				Name: "capture",
				Val: testParameters{
					Playback: false,
					Capture:  true,
				},
				ExtraHardwareDeps: hwdep.D(hwdep.Microphone()),
			},
			{
				Name: "playback_capture",
				Val: testParameters{
					Playback: true,
					Capture:  true,
				},
				ExtraHardwareDeps: hwdep.D(hwdep.Microphone(), hwdep.Speaker()),
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
	var outSched profiler.PerfSchedOutput

	profs := []profiler.Profiler{
		profiler.Top(&profiler.TopOpts{
			Interval: topInterval,
		}),
		profiler.Perf(profiler.PerfStatOpts(&out, pid)),
		profiler.Perf(profiler.PerfRecordOpts()),
		profiler.Perf(profiler.PerfSchedOpts(&outSched, "cras")),
	}

	s.Log("start audio")
	playbackCommand := crastestclient.PlaybackCommand(runCtx, int(commandDuration.Seconds()), blocksize)
	captureCommand := crastestclient.CaptureCommand(runCtx, int(commandDuration.Seconds()), blocksize)

	if param.Capture {
		captureCommand.Start()
	}

	if param.Capture && param.Playback {
		// Wait one second to simulate WebRTC creating an output
		// stream about 1 seconds after creating an input stream.
		if err := testing.Sleep(ctx, 1*time.Second); err != nil {
			s.Fatal("Timed out on sleep: ", err)
		}
	}

	if param.Playback {
		playbackCommand.Start()
	}

	// Wait one second for audio processing stream to be ready.
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
		if err := runningProfs.End(ctx); err != nil {
			s.Error("Failure in ending the profiler: ", err)
		} else {
			// Append one measurement to PerfValue.
			pv.Append(perfpkg.Metric{
				Name:      "cras_cycles_per_second",
				Unit:      "cycles",
				Direction: perfpkg.SmallerIsBetter,
				Multiple:  true,
			}, out.CyclesPerSecond)

			// Append one measurement to PerfValue.
			pv.Append(perfpkg.Metric{
				Name:      "cras_max_latency_ms",
				Unit:      "milliseconds",
				Direction: perfpkg.SmallerIsBetter,
				Multiple:  true,
			}, outSched.MaxLatencyMs)
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
		crasPath   = "/usr/bin/cras"
		iterations = 10
	)

	// TODO(b/194820340): aarch64 is disabled while perf HW counters don't
	// all work reliably.
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
		if err := audio.RestartCras(ctx); err != nil {
			s.Fatal("Failed to restart CRAS: ", err)
		}

		proc, err := procutil.FindUnique(procutil.ByExe(crasPath))
		if err != nil {
			s.Fatal("Failed to find PID of cras: ", err)
		}

		s.Log("Get PID done: ", proc.Pid)
		crasPerfOneIteration(ctx, s, int(proc.Pid), pv)
	}

	if err := pv.Save(s.OutDir()); err != nil {
		s.Fatal("Cannot save perf data: ", err)
	}
}
