// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package encode provides common code to run Chrome binary tests for video encoding.
package encode

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/gtest"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/local/media/encoding"
	"chromiumos/tast/local/media/logging"
	"chromiumos/tast/local/media/videotype"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

// cpuLog is the name of log file recording CPU usage.
const cpuLog = "cpu.log"

// powerLog is the name of lof file recording power consumption.
const powerLog = "power.log"

// bitrateTestFilter is the test pattern in googletest style for disabling bitrate control related tests.
const bitrateTestFilter = "-MidStreamParamSwitchBitrate/*:ForceBitrate/*:MultipleEncoders/VideoEncodeAcceleratorTest.TestSimpleEncode/1"

// binArgs is the arguments and the modes for executing video_encode_accelerator_unittest binary.
type binArgs struct {
	// testFilter specifies test pattern in googletest style for the unittest to run and will be passed with "--gtest_filter" (see go/gtest-running-subset).
	// If unspecified, the unittest runs all tests.
	testFilter string
	// extraArgs is the additional arguments to pass video_encode_accelerator_unittest, for example, "--native_input".
	extraArgs []string
	// measureUsage indicates whether to measure CPU usage and power consumption while running binary and save as perf metrics.
	measureUsage bool
	// measureDuration specifies how long to measure CPU usage and power consumption when measureUsage is set.
	measureDuration time.Duration
}

// testMode represents the test's running mode.
type testMode int

const (
	// functionalTest indicates a functional test.
	functionalTest testMode = iota
	// performanceTest indicates a performance test. CPU scaling should be adujst to performance.
	performanceTest
)

// runAccelVideoTest runs video_encode_accelerator_unittest for each binArgs.
// It fails if video_encode_accelerator_unittest fails.
func runAccelVideoTest(ctx context.Context, s *testing.State, mode testMode,
	opts encoding.TestOptions, cacheExtractedVideo bool, bas ...binArgs) {
	// Reserve time to restart the ui job at the end of the test.
	// Only a single process can have access to the GPU, so we are required
	// to call "stop ui" at the start of the test. This will shut down the
	// chrome process and allow us to claim ownership of the GPU.
	shortCtx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	if err := upstart.StopJob(shortCtx, "ui"); err != nil {
		s.Error("Failed to stop ui: ", err)
	}
	defer upstart.EnsureJobRunning(ctx, "ui")

	params := opts.Params
	streamPath, err := encoding.PrepareYUV(shortCtx, s.DataPath(params.Name), opts.PixelFormat, params.Size)
	if err != nil {
		s.Fatal("Failed to prepare YUV file: ", err)
	}
	if !cacheExtractedVideo {
		defer os.Remove(streamPath)
	}
	encodeOutFile := strings.TrimSuffix(params.Name, ".vp9.webm")
	switch opts.Profile {
	case videotype.H264Prof:
		encodeOutFile += ".h264"
	case videotype.VP8Prof:
		encodeOutFile += ".vp8.ivf"
	case videotype.VP9Prof, videotype.VP9_2Prof:
		encodeOutFile += ".vp9.ivf"
	default:
		s.Fatalf("Failed to setup encoded output path for profile %d", opts.Profile)
	}

	outPath := filepath.Join(s.OutDir(), encodeOutFile)
	commonArgs := []string{logging.ChromeVmoduleFlag(),
		encoding.CreateStreamDataArg(params, opts.Profile, opts.PixelFormat, streamPath, outPath),
		"--ozone-platform=drm",

		// The default timeout for test launcher is 45 seconds, which is not enough for some test cases.
		// Considering we already manage timeout by Tast context.Context, we don't need another timeout at test launcher.
		// Set a huge timeout (3600000 milliseconds, 1 hour) here.
		"--test-launcher-timeout=3600000",
	}
	if opts.InputMode == encoding.DMABuf {
		commonArgs = append(commonArgs, "--native_input")
	}

	if mode == performanceTest {
		if err := cpu.WaitUntilIdle(shortCtx); err != nil {
			s.Fatal("Failed waiting for CPU to become idle: ", err)
		}
	}

	exec := filepath.Join(chrome.BinTestDir, "video_encode_accelerator_unittest")
	for _, ba := range bas {
		args := append(commonArgs, ba.extraArgs...)
		logfile := filepath.Join(s.OutDir(), fmt.Sprintf("output_%s_%d.txt", filepath.Base(exec), time.Now().Unix()))

		t := gtest.New(
			exec,
			gtest.Logfile(logfile),
			gtest.Filter(ba.testFilter),
			gtest.ExtraArgs(args...),
			gtest.UID(int(sysutil.ChronosUID)))
		if ba.measureUsage {
			measurements, err := cpu.MeasureProcessUsage(shortCtx, ba.measureDuration, cpu.KillProcess, t)
			if err != nil {
				s.Fatalf("Failed to run (measure CPU) %v: %v", exec, err)
			}
			cpuUsage := measurements["cpu"]
			// TODO(b/143190876): Don't write value to disk, as this can increase test flakiness.
			cpuLogPath := filepath.Join(s.OutDir(), cpuLog)
			if err := ioutil.WriteFile(cpuLogPath, []byte(fmt.Sprintf("%f", cpuUsage)), 0644); err != nil {
				s.Fatal("Failed to write CPU usage to file: ", err)
			}

			powerConsumption, ok := measurements["power"]
			if ok {
				// TODO(b/143190876): Don't write value to disk, as this can increase test flakiness.
				powerLogPath := filepath.Join(s.OutDir(), powerLog)
				if err := ioutil.WriteFile(powerLogPath, []byte(fmt.Sprintf("%f", powerConsumption)), 0644); err != nil {
					s.Fatal("Failed to write power consumption to file: ", err)
				}
			}
		} else {
			if report, err := t.Run(ctx); err != nil {
				if report != nil {
					for _, name := range report.FailedTestNames() {
						s.Error(name, " failed")
					}
				}
				s.Fatalf("Failed to run %v: %v", exec, err)
			}
		}
		// Only keep the encoded result when there's something wrong.
		if err := os.Remove(outPath); err != nil {
			s.Log("Failed to remove output file: ", err)
		}
	}
}

// RunAllAccelVideoTests runs all tests in video_encode_accelerator_unittest.
func RunAllAccelVideoTests(ctx context.Context, s *testing.State, opts encoding.TestOptions, cacheExtractedVideo bool) {
	vl, err := logging.NewVideoLogger()
	if err != nil {
		s.Fatal("Failed to set values for verbose logging")
	}
	defer vl.Close()

	// Currently the Intel driver cannot set the VP9 bitrate correctly.
	// TODO(b/134538840): Remove after the driver is fixed.
	testFilter := ""
	if opts.Profile == videotype.VP9Prof {
		testFilter = bitrateTestFilter
	}

	runAccelVideoTest(ctx, s, functionalTest, opts, cacheExtractedVideo, binArgs{testFilter: testFilter})
}

// RunAccelVideoPerfTest runs video_encode_accelerator_unittest multiple times with different arguments to gather perf metrics.
func RunAccelVideoPerfTest(ctx context.Context, s *testing.State, opts encoding.TestOptions, cacheExtractedVideo bool) {
	const (
		// testLogSuffix is the log name suffix of dumping log from test binary.
		testLogSuffix = "test.log"
		// frameStatsSuffix is the log name suffix of frame statistics.
		frameStatsSuffix = "frame-data.csv"
		// cpuEncodeFrames is the number of encoded frames for CPU usage test. It should be high enouch to run for measurement duration.
		cpuEncodeFrames = 10000
		// duration of the interval during which CPU usage and power consumption will be measured.
		measureDuration = 10 * time.Second
		// time reserved for cleanup.
		cleanupTime = 5 * time.Second
	)

	cleanUpBenchmark, err := cpu.SetUpBenchmark(ctx)
	if err != nil {
		s.Fatal("Failed to set up benchmark mode: ", err)
	}
	defer cleanUpBenchmark(ctx)
	// Leave a bit of time to clean up benchmark mode.
	ctx, cancel := ctxutil.Shorten(ctx, cleanupTime)
	defer cancel()

	schemaName := strings.TrimSuffix(opts.Params.Name, ".vp9.webm")
	if opts.Profile == videotype.H264Prof {
		schemaName += "_h264"
	} else {
		schemaName += "_vp8"
	}

	fpsLogPath := getResultFilePath(s.OutDir(), schemaName, "fullspeed", testLogSuffix)

	latencyLogPath := getResultFilePath(s.OutDir(), schemaName, "fixedspeed", testLogSuffix)
	cpuLogPath := filepath.Join(s.OutDir(), cpuLog)
	powerLogPath := filepath.Join(s.OutDir(), powerLog)

	frameStatsPath := getResultFilePath(s.OutDir(), schemaName, "quality", frameStatsSuffix)

	runAccelVideoTest(ctx, s, performanceTest, opts, cacheExtractedVideo,
		// Run video_encode_accelerator_unittest to get FPS.
		binArgs{
			testFilter: "EncoderPerf/*/0",
			extraArgs:  []string{fmt.Sprintf("--output_log=%s", fpsLogPath)},
		},
		// Run video_encode_accelerator_unittest to get encode latency under specified frame rate.
		binArgs{
			testFilter: "SimpleEncode/*/0",
			extraArgs: []string{fmt.Sprintf("--output_log=%s", latencyLogPath),
				"--run_at_fps", "--measure_latency"},
		},
		// Run video_encode_accelerator_unittest to generate SSIM/PSNR scores (objective quality metrics).
		binArgs{
			testFilter: "SimpleEncode/*/0",
			extraArgs:  []string{fmt.Sprintf("--frame_stats=%s", frameStatsPath)},
		},
		// Run video_encode_accelerator_unittest to get CPU usage and power consumption under specified frame rate.
		binArgs{
			testFilter: "SimpleEncode/*/0",
			extraArgs: []string{fmt.Sprintf("--num_frames_to_encode=%d", cpuEncodeFrames),
				"--run_at_fps"},
			measureUsage:    true,
			measureDuration: measureDuration,
		},
	)

	p := perf.NewValues()

	if err := encoding.ReportFPS(ctx, p, schemaName, fpsLogPath); err != nil {
		s.Fatal("Failed to report FPS value: ", err)
	}

	if err := encoding.ReportEncodeLatency(ctx, p, schemaName, latencyLogPath); err != nil {
		s.Fatal("Failed to report encode latency: ", err)
	}

	if err := encoding.ReportCPUUsage(ctx, p, schemaName, cpuLogPath); err != nil {
		s.Fatal("Failed to report CPU usage: ", err)
	}

	// TODO(b/143190876): Don't write value to disk, as this can increase test flakiness.
	if _, err := os.Stat(powerLogPath); os.IsNotExist(err) {
		s.Logf("Skipped reporting power consumption because %s does not exist", powerLog)
	} else {
		if err := encoding.ReportPowerConsumption(ctx, p, schemaName, powerLogPath); err != nil {
			s.Fatal("Failed to report power consumption: ", err)
		}
	}

	if err := encoding.ReportFrameStats(p, schemaName, frameStatsPath); err != nil {
		s.Fatal("Failed to report frame stats: ", err)
	}
	p.Save(s.OutDir())
}

// getResultFilePath is the helper function to get the log path for recoding perf data.
func getResultFilePath(outDir, name, subtype, suffix string) string {
	return filepath.Join(outDir, fmt.Sprintf("%s_%s_%s", name, subtype, suffix))
}
