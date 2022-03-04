// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package audio

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"syscall"
	"time"

	perfpkg "chromiumos/tast/common/perf"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/audio"
	"chromiumos/tast/local/procutil"
	"chromiumos/tast/local/profiler"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     CrasCspPerf,
		Desc:     "Performance measurement of CRAS client side processing",
		Contacts: []string{"aaronyu@google.com", "cychiang@chromium.org"},
		Attr:     []string{"group:crosbolt", "crosbolt_perbuild"},
		Timeout:  5 * time.Minute,
	})
}

type sampleTimer struct {
	step    int
	next    int
	current int
	timings []time.Time
}

func (w *sampleTimer) Write(b []byte) (int, error) {
	t := time.Now()
	for w.current += len(b); w.next < w.current; w.next += w.step {
		w.timings = append(w.timings, t)
	}
	return len(b), nil
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

func crasCspPerfOneIteration(ctx context.Context, s *testing.State, pid int, pv *perfpkg.Values) {
	const (
		getDeviceTimeout = 3 * time.Second
		blocksize        = 480
		topInterval      = 1 * time.Second
		perfDuration     = 10 * time.Second              // Duration to run perf command.
		commandDuration  = perfDuration + 2*time.Second  // Duration to run audio command
		contextDuration  = perfDuration + 15*time.Second // Upper bound for one iteration.
		unprocessedRAW   = "unprocessed.raw"
		processedRAW     = "processed.raw"
		timerStep        = 48000 * 2 * 2
	)

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

	cras, err := audio.NewCras(runCtx)
	if err != nil {
		s.Fatal("NewCras", err)
	}

	// mute to avoid echo
	outputNode, err := cras.SelectedOutputNode(runCtx)
	if err != nil {
		s.Fatal("SelectdOutputNode", err)
	}
	err = cras.SetOutputNodeVolume(runCtx, *outputNode, 0)
	if err != nil {
		s.Fatal("SetOutputNodeVolume", err)
	}

	dev, err := requestFloopMask(runCtx, 0x4)
	if err != nil {
		s.Fatal(err)
	}

	stopCommand := func(cmd *testexec.Cmd) {
		if err := cmd.Kill(); err != nil {
			s.Error(err)
		} else {
			return // TODO
			if err := cmd.Wait(); err != nil {
				if eerr, ok := err.(*exec.ExitError); ok {
					wstatus := eerr.Sys().(syscall.WaitStatus)
					if wstatus.Signal() == syscall.SIGKILL {
						// killed is OK
						return
					}
				}
				s.Error(err)
			}
		}
	}

	// the processing pipeline
	// cras_test_client --capture_file=/dev/stdout | tee unprocessed.raw | cras_test_client --playback_file=/dev/stdin
	unprocessedBuf := &bytes.Buffer{}
	rawTimings := &sampleTimer{step: timerStep}
	pipeCaptureCmd := testexec.CommandContext(
		runCtx,
		"cras_test_client",
		"--capture_file=/dev/stdout",
		fmt.Sprintf("--duration_seconds=%d", commandDuration/time.Second),
	)
	pipePlaybackCmd := testexec.CommandContext(runCtx, "cras_test_client", "--playback_file=/dev/stdin")
	playbackReader, captureWriter := io.Pipe()
	pipeCaptureCmd.Stdout = io.MultiWriter(unprocessedBuf, rawTimings, captureWriter)
	pipePlaybackCmd.Stdin = playbackReader
	pipeCaptureCmd.Stderr = os.Stderr
	pipePlaybackCmd.Stderr = os.Stderr

	// cras_test_client --capture_file=/tmp/processed.raw --pin_device=$floop_dev
	appTimings := &sampleTimer{step: timerStep}
	processedBuf := &bytes.Buffer{}
	appCaptureCmd := testexec.CommandContext(
		runCtx,
		"cras_test_client",
		fmt.Sprintf("--capture_file=/dev/stdout"),
		fmt.Sprintf("--pin_device=%d", dev),
		fmt.Sprintf("--duration_seconds=%d", commandDuration/time.Second),
	)
	appCaptureCmd.Stdout = io.MultiWriter(appTimings, processedBuf)
	appCaptureCmd.Stderr = os.Stderr

	if err := appCaptureCmd.Start(); err != nil {
		s.Fatal(err)
	}

	// wait for the capture client to be actually running
	testing.Sleep(runCtx, time.Second)

	err = pipeCaptureCmd.Start()
	if err != nil {
		s.Fatal(err)
	}

	err = pipePlaybackCmd.Start()
	if err != nil {
		s.Fatal(err)
	}
	defer playbackReader.Close()
	defer stopCommand(pipePlaybackCmd)

	runningProfs, err := profiler.Start(ctx, s.OutDir(), profs...)
	if err != nil {
		s.Fatal("Failure in starting the profiler: ", err)
	}

	// Record for perfDuration seconds.
	// This is to make sure that audio is being used during whole
	// perf recording.
	if err := testing.Sleep(ctx, perfDuration); err != nil {
		s.Fatal("Timed out on sleep: ", err)
	}

	if err := runningProfs.End(ctx); err != nil {
		s.Error("Failure in ending the profiler: ", err)
	}

	if err := pipeCaptureCmd.Wait(); err != nil {
		s.Fatal(err)
	}

	pv.Append(perfpkg.Metric{
		Name:      "cras_cycles_per_second",
		Unit:      "cycles",
		Direction: perfpkg.SmallerIsBetter,
		Multiple:  true,
	}, out.CyclesPerSecond)

	pv.Append(perfpkg.Metric{
		Name:      "cras_max_latency_ms",
		Unit:      "milliseconds",
		Direction: perfpkg.SmallerIsBetter,
		Multiple:  true,
	}, outSched.MaxLatencyMs)

	s.Log("sizes ", unprocessedBuf.Len(), processedBuf.Len())
	for i := range appTimings.timings {
		s.Log(i, appTimings.timings[i].Sub(rawTimings.timings[i]))
	}

	err = os.WriteFile(filepath.Join(s.OutDir(), unprocessedRAW), unprocessedBuf.Bytes(), 0644)
	if err != nil {
		s.Fatal(err)
	}
	err = os.WriteFile(filepath.Join(s.OutDir(), processedRAW), processedBuf.Bytes(), 0644)
	if err != nil {
		s.Fatal(err)
	}

	if err := appCaptureCmd.Wait(); err != nil {
		s.Fatal(err)
	}

	s.Log("Finished one iteration")
}

func CrasCspPerf(ctx context.Context, s *testing.State) {
	const (
		crasPath   = "/usr/bin/cras"
		iterations = 1
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
		s.Log("Iteration:", i)

		// Stop CRAS to make sure the audio device won't be occupied.
		s.Log("Restarting CRAS")

		if err := upstart.RestartJob(ctx, "cras"); err != nil {
			s.Fatal("Failed to stop CRAS:", err)
		}

		// Any device being available means CRAS is ready.
		if err := audio.WaitForDevice(ctx, audio.OutputStream|audio.InputStream); err != nil {
			s.Fatal("Failed to wait for any output or input device: ", err)
		}

		proc, err := procutil.FindUnique(procutil.ByExe(crasPath))
		if err != nil {
			s.Fatal("Failed to find PID of CRAS:", err)
		}

		s.Log("Get CRAS PID:", proc.Pid)
		crasCspPerfOneIteration(ctx, s, int(proc.Pid), pv)
	}

	s.Log("out of loop")

	if err := pv.Save(s.OutDir()); err != nil {
		s.Fatal("Cannot save perf data: ", err)
	}
}
