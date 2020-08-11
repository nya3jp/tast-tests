// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package audio

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	"github.com/shirou/gopsutil/process"

	perfpkg "chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/audio"
	"chromiumos/tast/local/bundles/cros/audio/crastestutils"
	"chromiumos/tast/local/profiler"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/local/testexec"
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
		Func:         CrasPerf,
		Desc:         "Performance measurement of CRAS",
		Contacts:     []string{"yuhsuan@chromium.org", "cychiang@chromium.org"},
		SoftwareDeps: []string{"audio_play", "audio_record"},
		Attr:         []string{"group:mainline", "informational"},
		Timeout:      5 * time.Minute,
		Params: []testing.Param{
			{
				Name: "playback",
				Val: testParameters{
					Playback: true,
					Capture:  false,
				},
			},
			{
				Name: "capture",
				Val: testParameters{
					Playback: false,
					Capture:  true,
				},
			},
			{
				Name: "playback_capture",
				Val: testParameters{
					Playback: true,
					Capture:  true,
				},
			},
		},
	})
}

func crasPerfOneIteration(ctx context.Context, s *testing.State, pid int, pv *perfpkg.Values) {
	const (
		duration         = 10 // second
		getDeviceTimeout = 3 * time.Second
		blocksize        = 480
	)

	outputDevName := ""
	inputDevName := ""

	param := s.Param().(testParameters)

	// Get the first running output device by parsing asound status.
	// A device may not be opened immediately so it will repeat a query until there is a running output device.
	playbackDevicePath := "/proc/asound/card*/pcm*p/sub0/status"
	captureDevicePath := "/proc/asound/card*/pcm*c/sub0/status"

	findRunningDevice := func(ctx context.Context, pathPattern string) (string, error) {
		paths, err := filepath.Glob(pathPattern)
		if err != nil {
			return "", err
		}
		for _, p := range paths {
			s.Log("checking path ", p)
			err := testexec.CommandContext(ctx, "grep", "RUNNING", p).Run()
			if err == nil {
				return p, nil
			}
		}
		return "", err
	}

	getRunningtDevice := func(ctx context.Context) error {
		s.Log("Dump asound status to check running devices")

		devPath, err := findRunningDevice(ctx, playbackDevicePath)
		if err != nil {
			return errors.Errorf("failed to grep playback asound status: %s", err)
		}
		outputDevName = devPath

		devPath, err = findRunningDevice(ctx, captureDevicePath)
		if err != nil {
			return errors.Errorf("failed to grep capture asound status: %s", err)
		}
		inputDevName = devPath

		if param.Playback && outputDevName == "" {
			return errors.New("can not detect running playback device")
		}

		if param.Capture && inputDevName == "" {
			return errors.New("can not detect running capture device")
		}

		return nil
	}

	// Set timeout to duration + 15s, which is the time buffer to complete the normal execution.
	runCtx, cancel := context.WithTimeout(ctx, (duration+15)*time.Second)
	defer cancel()

	profs := []profiler.Profiler{
		profiler.Top(&profiler.TopOpts{
			Interval: duration * time.Second,
		}),
	}

	// TODO(crbug.com/996728): aarch64 is disabled before the kernel crash is fixed.
	if u, err := sysutil.Uname(); err == nil && u.Machine != "aarch64" {
		perfStatOpts, statOptErr := profiler.GetPerfStatOpts(pid)
		if statOptErr != nil {
			s.Fatal("Fail to create PerfStat opts", statOptErr)
		}
		profs = append(profs, profiler.Perf(perfStatOpts))
		profs = append(profs, profiler.Perf(profiler.GetPerfRecordOpts()))
	}

	s.Log("start audio")
	playbackCommand := crastestutils.CRASPlaybackCommand(runCtx, duration+2, blocksize)
	captureCommand := crastestutils.CRASCaptureCommand(runCtx, duration+2, blocksize)

	if param.Playback {
		playbackCommand.Start()
	}

	if param.Capture {
		captureCommand.Start()
	}

	// Wait one second for audio processing to be stable.
	if err := testing.Sleep(ctx, 1*time.Second); err != nil {
		s.Fatal("Timed out on sleep: ", err)
	}

	p, err := profiler.Start(ctx, s.OutDir(), profs...)

	if err != nil {
		s.Fatal("Failure in starting the profiler: ", err)
	}

	if err := testing.Poll(ctx, getRunningtDevice, &testing.PollOptions{Timeout: getDeviceTimeout}); err != nil {
		s.Fatal("Failed to detect running device: ", err)
	}

	s.Log("Found running output device: ", outputDevName)
	s.Log("Found running input device: ", inputDevName)

	if strings.Contains(outputDevName, "Silent") {
		s.Fatal("Playback fallback to the silent device")
	}
	if strings.Contains(inputDevName, "Silent") {
		s.Fatal("Capture fallback to the silent device")
	}

	// Record for duration seconds.
	// This is to make sure that audio is being used during whole
	// perf recording.
	if err := testing.Sleep(ctx, duration*time.Second); err != nil {
		s.Fatal("Timed out on sleep: ", err)
	}

	defer func() {
		// The perf value is stored when ending the profiler.
		if allOutput, err := p.End(); err != nil {
			s.Fatal("Failure in ending the profiler: ", err)
		} else {
			cyclesPerSecond := allOutput[1].Props["cyclesPerSecond"].(float64)

			// Append one measurement to PerfValue.
			pv.Append(perfpkg.Metric{
				Name:      "cras_cycles_per_second",
				Unit:      "cycles",
				Direction: perfpkg.SmallerIsBetter,
				Multiple:  true,
			}, cyclesPerSecond)
		}

		if param.Playback {
			if err := playbackCommand.Wait(); err != nil {
				s.Fatal("Playback did not finish in time: ", err)
			}
		}

		if param.Capture {
			if err := captureCommand.Wait(); err != nil {
				s.Fatal("Capture did not finish in time: ", err)
			}
		}

		s.Log("Finished one iteration")
	}()
}

func CrasPerf(ctx context.Context, s *testing.State) {
	const (
		iterations = 10
		crasPath   = "/usr/bin/cras"
	)

	// Find PID of cras.
	getPID := func() (int, error) {
		all, err := process.Pids()

		if err != nil {
			return -1, err
		}

		for _, pid := range all {
			if proc, err := process.NewProcess(pid); err != nil {
				// Assume that the process exited.
				continue
			} else if exe, err := proc.Exe(); err == nil && exe == crasPath {
				return int(pid), nil
			}
		}
		return -1, errors.Errorf("%v process not found", crasPath)
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

		if err := audio.WaitForDevice(ctx, audio.OutputStream&audio.InputStream); err != nil {
			s.Fatal("Failed to wait for output and input device: ", err)
		}

		pid, err := getPID()

		s.Log("get PID done")

		if err != nil {
			s.Fatal("Failed to find PID of cras: ", err)
		}

		crasPerfOneIteration(ctx, s, pid, pv)
	}

	if err := pv.Save(s.OutDir()); err != nil {
		s.Fatal("Cannot save perf data: ", err)
	}

}
