// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package decode provides common code to run Chrome binary tests for video decoding.
package decode

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/gtest"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/local/media/logging"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

// DecoderType represents the different video decoder types.
type DecoderType int

const (
	// VDA is the video decoder type based on the VideoDecodeAccelerator
	// interface. These are set to be deprecrated.
	VDA DecoderType = iota
	// VD is the video decoder type based on the VideoDecoder interface. These
	// will replace the current VDAs.
	VD
)

// RunAccelVideoTest runs video_decode_accelerator_tests with the specified
// video file. decoderType specifies whether to run the tests against the VDA
// or VD based video decoder implementations.
func RunAccelVideoTest(ctx context.Context, s *testing.State, filename string, decoderType DecoderType) {
	vl, err := logging.NewVideoLogger()
	if err != nil {
		s.Fatal("Failed to set values for verbose logging: ", err)
	}
	defer vl.Close()

	// Reserve time to restart the ui job at the end of the test.
	// Only a single process can have access to the GPU, so we are required
	// to call "stop ui" at the start of the test. This will shut down the
	// chrome process and allow us to claim ownership of the GPU.
	shortCtx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()
	upstart.StopJob(shortCtx, "ui")
	defer upstart.EnsureJobRunning(ctx, "ui")

	args := []string{
		s.DataPath(filename),
		s.DataPath(filename + ".json"),
		"--output_folder=" + s.OutDir(),
	}
	if decoderType == VD {
		args = append(args, "--use_vd")
	}

	const exec = "video_decode_accelerator_tests"
	if report, err := gtest.New(
		filepath.Join(chrome.BinTestDir, exec),
		gtest.Logfile(filepath.Join(s.OutDir(), exec+".log")),
		gtest.ExtraArgs(args...),
		gtest.UID(int(sysutil.ChronosUID)),
	).Run(shortCtx); err != nil {
		s.Errorf("Failed to run %v with video %s: %v", exec, filename, err)
		if report != nil {
			for _, name := range report.FailedTestNames() {
				s.Error(name, " failed")
			}
		}
	}
}

// RunAccelVideoPerfTest runs video_decode_accelerator_perf_tests with the
// specified video file. decoderType specifies whether to run the tests against
// the VDA or VD based video decoder implementations. Both capped and uncapped
// performance is measured.
// - Uncapped performance: the specified test video is decoded from start to
// finish as fast as possible. This provides an estimate of the decoder's max
// performance (e.g. the maximum FPS).
// - Capped decoder performance: uses a more realistic environment by decoding
// the test video from start to finish at its actual frame rate. Rendering is
// simulated and late frames are dropped.
// The test binary is run twice. Once to measure both capped and uncapped
// performance, once to measure CPU usage while running the capped performance
// test.
func RunAccelVideoPerfTest(ctx context.Context, s *testing.State, filename string, decoderType DecoderType) {
	const (
		// Name of the capped performance test.
		cappedTestname = "MeasureCappedPerformance"
		// Name of the uncapped performance test.
		uncappedTestname = "MeasureUncappedPerformance"
		// Duration of the interval during which CPU usage will be measured.
		measureDuration = 20 * time.Second
		// Time reserved for cleanup.
		cleanupTime = 10 * time.Second
	)

	// Only a single process can have access to the GPU, so we are required to
	// call "stop ui" at the start of the test. This will shut down the chrome
	// process and allow us to claim ownership of the GPU.
	if err := upstart.StopJob(ctx, "ui"); err != nil {
		s.Fatal("Failed to stop ui: ", err)
	}
	defer upstart.EnsureJobRunning(ctx, "ui")

	// Setup benchmark mode.
	cleanUpBenchmark, err := cpu.SetUpBenchmark(ctx)
	if err != nil {
		s.Fatal("Failed to set up benchmark mode: ", err)
	}
	defer cleanUpBenchmark(ctx)

	// Reserve time to restart the ui job and perform cleanup at the end of the test.
	ctx, cancel := ctxutil.Shorten(ctx, cleanupTime)
	defer cancel()

	if err := cpu.WaitUntilIdle(ctx); err != nil {
		s.Fatal("Failed waiting for CPU to become idle: ", err)
	}

	// Test 1: Measure capped and uncapped performance.
	args := []string{
		s.DataPath(filename),
		s.DataPath(filename + ".json"),
		"--output_folder=" + s.OutDir(),
	}
	if decoderType == VD {
		args = append(args, "--use_vd")
	}

	const exec = "video_decode_accelerator_perf_tests"
	if report, err := gtest.New(
		filepath.Join(chrome.BinTestDir, exec),
		gtest.Logfile(filepath.Join(s.OutDir(), exec+".1.log")),
		gtest.Filter(fmt.Sprintf("*%s:*%s", cappedTestname, uncappedTestname)),
		gtest.ExtraArgs(args...),
		gtest.UID(int(sysutil.ChronosUID)),
	).Run(ctx); err != nil {
		s.Errorf("Failed to run %v with video %s: %v", exec, filename, err)
		if report != nil {
			for _, name := range report.FailedTestNames() {
				s.Error(name, " failed")
			}
		}
		return
	}

	p := perf.NewValues()
	uncappedJSON := filepath.Join(s.OutDir(), "VideoDecoderTest", uncappedTestname+".json")
	if _, err := os.Stat(uncappedJSON); os.IsNotExist(err) {
		s.Fatal("Failed to find uncapped performance metrics file: ", err)
	}

	cappedJSON := filepath.Join(s.OutDir(), "VideoDecoderTest", cappedTestname+".json")
	if _, err := os.Stat(cappedJSON); os.IsNotExist(err) {
		s.Fatal("Failed to find capped performance metrics file: ", err)
	}

	if err := parseUncappedPerfMetrics(uncappedJSON, p); err != nil {
		s.Fatal("Failed to parse uncapped performance metrics: ", err)
	}
	if err := parseCappedPerfMetrics(cappedJSON, p); err != nil {
		s.Fatal("Failed to parse capped performance metrics: ", err)
	}

	// Test 2: Measure CPU usage while running capped performance test only.
	// TODO(dstaessens) Investigate collecting CPU usage during previous test.
	measurements, err := cpu.MeasureProcessUsage(ctx, measureDuration, cpu.KillProcess, gtest.New(
		filepath.Join(chrome.BinTestDir, exec),
		gtest.Logfile(filepath.Join(s.OutDir(), exec+".2.log")),
		gtest.Filter("*"+cappedTestname),
		// Repeat enough times to run for full measurement duration. We don't
		// use -1 here as this can result in huge log files (b/138822793).
		gtest.Repeat(1000),
		gtest.ExtraArgs(args...),
		gtest.UID(int(sysutil.ChronosUID)),
	))
	if err != nil {
		s.Fatalf("Failed to measure CPU usage %v: %v", exec, err)
	}
	cpuUsage := measurements["cpu"]

	p.Set(perf.Metric{
		Name:      "cpu_usage",
		Unit:      "percent",
		Direction: perf.SmallerIsBetter,
	}, cpuUsage)

	if err := p.Save(s.OutDir()); err != nil {
		s.Fatal("Failed to save performance metrics: ", err)
	}
}

// RunAccelVideoSanityTest runs the FlushAtEndOfStream test in the
// video_decode_accelerator_tests. The test only fails if the test binary
// crashes or the video decoder's kernel driver crashes.
// The motivation of the sanity test: on certain devices, when playing VP9
// profile 1 or 3, the kernel crashed. Though the profile was not supported
// by the decoder, kernel driver should not crash in any circumstances.
// Refer to https://crbug.com/951189 for more detail.
func RunAccelVideoSanityTest(ctx context.Context, s *testing.State, filename string) {
	const cleanupTime = 10 * time.Second

	vl, err := logging.NewVideoLogger()
	if err != nil {
		s.Fatal("Failed to set values for verbose logging: ", err)
	}
	defer vl.Close()

	// Only a single process can have access to the GPU, so we are required to
	// call "stop ui" at the start of the test. This will shut down the chrome
	// process and allow us to claim ownership of the GPU.
	if err := upstart.StopJob(ctx, "ui"); err != nil {
		s.Fatal("Failed to stop ui: ", err)
	}
	defer upstart.EnsureJobRunning(ctx, "ui")

	// Reserve time to restart the ui job and perform cleanup at the end of the test.
	ctx, cancel := ctxutil.Shorten(ctx, cleanupTime)
	defer cancel()

	// Run the FlushAtEndOfStream test, but ignore errors. We expect the test
	// binary to cleanly terminate with an "exit status 1" error. We ignore the
	// contents of the test report as we're not interested in actual failures.
	// The tast test only fails if the process or kernel crashed.
	// TODO(crbug.com/998464) Kernel crashes will currently cause remaining
	// tests to be aborted.
	const exec = "video_decode_accelerator_tests"
	testing.ContextLogf(ctx, "Running %v with an invalid video stream, "+
		"test failures are expected but no crashes should occur", exec)
	if _, err := gtest.New(
		filepath.Join(chrome.BinTestDir, exec),
		gtest.Logfile(filepath.Join(s.OutDir(), exec+".log")),
		gtest.Filter("*FlushAtEndOfStream"),
		gtest.ExtraArgs(
			s.DataPath(filename),
			s.DataPath(filename+".json"),
			"--output_folder="+s.OutDir(),
			"--disable_validator"),
		gtest.UID(int(sysutil.ChronosUID)),
	).Run(ctx); err != nil {
		// The test binary should run without crashing, but we expect the tests
		// themselves to fail. We can check the exit code to differentiate
		// between tests failing (exit code 1) and the test binary crashing
		// (e.g. exit code 139 on Linux).
		waitStatus, ok := testexec.GetWaitStatus(err)
		if !ok {
			s.Fatal("Failed to get gtest exit status")
		}
		if waitStatus.ExitStatus() != 1 {
			s.Fatalf("Failed to run %v: %v", exec, err)
		}
		testing.ContextLog(ctx, "No crashes detected, running video sanity test successful")
	}
}
