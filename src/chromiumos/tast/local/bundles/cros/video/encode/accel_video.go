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
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/video/lib/cpu"
	"chromiumos/tast/local/bundles/cros/video/lib/logging"
	"chromiumos/tast/local/bundles/cros/video/lib/videotype"
	"chromiumos/tast/local/chrome/bintest"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

// StreamParams is the parameter for video_encode_accelerator_unittest.
type StreamParams struct {
	// Name is the name of input raw data file.
	Name string
	// Size is the width and height of YUV image in the input raw data.
	Size videotype.Size
	// Bitrate is the requested bitrate in bits per second. VideoEncodeAccelerator is forced to output
	// encoded video in expected range around the bitrate.
	Bitrate int
	// FrameRate is the initial frame rate in the test. This value is optional, and will be set to
	// 30 if unspecified.
	FrameRate int
	// SubseqBitrate is the bitrate to switch to in the middle of the stream in some test cases in
	// video_encode_accelerator_unittest. This value is optional, and will be set to two times of Bitrate if unspecified.
	SubseqBitrate int
	// SubseqFrameRate is the frame rate to switch to in the middle of the stream in some test cases in
	// video_encode_accelerator_unittest. This value is optional, and will be set to 30 if unspecified.
	SubseqFrameRate int
}

// InputStorageMode represents the input buffer storage type of video_encode_accelerator_unittest.
type InputStorageMode int

const (
	// SharedMemory is a mode where video encode accelerator uses MEM-backed input VideoFrame on encode.
	SharedMemory InputStorageMode = iota
	// DmaBuf is a mode where video encode accelerator uses DmaBuf-backed input VideoFrame on encode.
	DmaBuf
)

// bintestRunMode represents the mode of executing video_encode_accelerator_unittest binary.
type bintestRunMode int

const (
	// runSync is a mode to run binary synchronously.
	runSync bintestRunMode = iota
	// runAsyncCPUBenchmark is a mode to run binary asynchronously, and measure CPU usage at the same time.
	runAsyncCPUBenchmark
)

// TestOptions is the options for runAccelVideoTest.
type TestOptions struct {
	// Profile is the codec profile to encode.
	Profile videotype.CodecProfile
	// Params is the test parameters for video_encode_accelerator_unittest.
	Params StreamParams
	// PixelFormat is the pixel format of input raw video data.
	PixelFormat videotype.PixelFormat
	// InputMode indicates which input storage mode the unittest runs with.
	InputMode InputStorageMode
}

// testArgs is the arguments and the modes for executing video_encode_accelerator_unittest binary.
type testArgs struct {
	// testFilter specifies test pattern for the unittest to run. If unspecified, the unittest runs all tests.
	testFilter string
	// extraArgs is the additional arguments to pass video_encode_accelerator_unittest, for example, "--native_input".
	extraArgs []string
	// bintestMode specifies the mode of executing video_encode_accelerator_unittest binary.
	bintestMode bintestRunMode
}

// runAccelVideoTest runs video_encode_accelerator_unittest for each testArgs.
// It fails if video_encode_accelerator_unittest fails.
func runAccelVideoTest(ctx context.Context, s *testing.State, opts TestOptions, targss ...testArgs) {
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

	params := opts.Params
	if !strings.HasSuffix(params.Name, ".vp9.webm") {
		s.Fatalf("Source video %v must be VP9 WebM", params.Name)
	}

	streamPath, err := prepareYUV(shortCtx, s.DataPath(params.Name), opts.PixelFormat, params.Size)
	if err != nil {
		s.Fatal("Failed to prepare YUV file: ", err)
	}
	defer os.Remove(streamPath)
	encodeOutFile := strings.TrimSuffix(params.Name, ".vp9.webm")
	if opts.Profile == videotype.H264Prof {
		encodeOutFile += ".h264"
	} else {
		encodeOutFile += ".vp8.ivf"
	}

	outPath := filepath.Join(s.OutDir(), encodeOutFile)
	preArgs := []string{logging.ChromeVmoduleFlag(),
		createStreamDataArg(params, opts.Profile, opts.PixelFormat, streamPath, outPath),
		"--ozone-platform=gbm",
	}
	if opts.InputMode == DmaBuf {
		preArgs = append(preArgs, "--native_input")
	}

	const exec = "video_encode_accelerator_unittest"

	for _, targs := range targss {
		args := append(preArgs, targs.extraArgs...)
		if targs.testFilter != "" {
			args = append(args, fmt.Sprintf("--gtest_filter=%s", targs.testFilter))
		}
		switch targs.bintestMode {
		case runSync:
			if err := bintest.Run(shortCtx, exec, args, s.OutDir()); err != nil {
				s.Fatalf("Failed to run %v: %v", exec, err)
			}
		case runAsyncCPUBenchmark:
			if err := runAsyncWithCPUBenchmark(shortCtx, exec, args, s.OutDir()); err != nil {
				s.Fatalf("Failed to run async (CPU benchmark) %v: %v", exec, err)
			}
		}
	}
}

// runAsyncWithCPUBenchmark measures CPU usage while running video_encode_accelerator_unittest asynchronously.
func runAsyncWithCPUBenchmark(ctx context.Context, exec string, args []string, outDir string) error {
	// CPU frequency scaling and thermal throttling might influence our test results.
	restoreCPUFrequencyScaling, err := cpu.DisableCPUFrequencyScaling()
	if err != nil {
		return errors.Wrap(err, "Failed to disable CPU frequency scaling.")
	}
	defer restoreCPUFrequencyScaling()

	restoreThermalThrottling, err := cpu.DisableThermalThrottling(ctx)
	if err != nil {
		return errors.Wrap(err, "Failed to disable thermal throttling.")
	}
	defer restoreThermalThrottling(ctx)

	if err := cpu.WaitForIdle(ctx, waitIdleCPUTimeout, idleCPUUsagePercent); err != nil {
		return errors.Wrap(err, "Failed waiting for CPU to become idle.")
	}

	cmd, err := bintest.RunAsync(ctx, exec, args, outDir)
	if err != nil {
		return errors.Wrap(err, "Failed to run binary.")
	}

	select {
	case <-ctx.Done():
		return errors.Wrap(err, "Failed waiting for CPU usage to stabilize.")
	case <-time.After(stabilizationDuration):
	}

	cpuUsage, err := cpu.MeasureUsage(ctx, measurementDuration)
	if err != nil {
		return errors.Wrap(err, "Failed to measure CPU usage")
	}

	str := strconv.FormatFloat(cpuUsage, 'f', -1, 64)

	// Write cpuUsage into a file located in output directory.
	logPath := filepath.Join(outDir, cpuLog)
	if err := ioutil.WriteFile(logPath, []byte(str), 0644); err != nil {
		return errors.Wrapf(err, "Failed to write file: %s", logPath)
	}

	return cmd.Wait()
}

// createStreamDataArg creates an argument of video_encode_accelerator_unittest from profile, dataPath and outFile.
func createStreamDataArg(params StreamParams, profile videotype.CodecProfile, pixelFormat videotype.PixelFormat, dataPath, outFile string) string {
	const (
		defaultFrameRate          = 30
		defaultSubseqBitrateRatio = 2
	)

	// Fill default values if they are unsettled.
	if params.FrameRate == 0 {
		params.FrameRate = defaultFrameRate
	}
	if params.SubseqBitrate == 0 {
		params.SubseqBitrate = params.Bitrate * defaultSubseqBitrateRatio
	}
	if params.SubseqFrameRate == 0 {
		params.SubseqFrameRate = defaultFrameRate
	}
	streamDataArgs := fmt.Sprintf("--test_stream_data=%s:%d:%d:%d:%s:%d:%d:%d:%d:%d",
		dataPath, params.Size.W, params.Size.H, int(profile), outFile,
		params.Bitrate, params.FrameRate, params.SubseqBitrate,
		params.SubseqFrameRate, int(pixelFormat))
	return streamDataArgs
}

// RunAllAccelVideoTest runs all tests in video_encode_accelerator_unittest.
func RunAllAccelVideoTest(ctx context.Context, s *testing.State, opts TestOptions) {
	runAccelVideoTest(ctx, s, opts, testArgs{bintestMode: runSync})
}

// RunAccelVideoPerfTest runs video_encode_accelerator_unittest multiple times with different arguments to gather perf metrics.
func RunAccelVideoPerfTest(ctx context.Context, s *testing.State, opts TestOptions) {
	sName := strings.TrimSuffix(opts.Params.Name, ".vp9.webm")
	if opts.Profile == videotype.H264Prof {
		sName += "_h264"
	} else {
		sName += "_vp8"
	}

	fpsLogPath := getResultFilePath(s.OutDir(), sName, "fullspeed", testLogSuffix)

	latencyLogPath := getResultFilePath(s.OutDir(), sName, "fixedspeed", testLogSuffix)
	cpuLogPath := filepath.Join(s.OutDir(), cpuLog)

	frameStatsPath := getResultFilePath(s.OutDir(), sName, "quality", frameStatsSuffix)

	runAccelVideoTest(ctx, s, opts,
		// Run video_encode_accelerator_unittest to get FPS.
		testArgs{
			testFilter:  "EncoderPerf/*/0",
			extraArgs:   []string{fmt.Sprintf("--output_log=%s", fpsLogPath)},
			bintestMode: runSync,
		},
		// Run video_encode_accelerator_unittest to get CPU usage and encode latency under specified frame rate.
		testArgs{
			testFilter:  "SimpleEncode/*/0",
			extraArgs:   []string{fmt.Sprintf("--output_log=%s", latencyLogPath), "--run_at_fps", "--measure_latency"},
			bintestMode: runAsyncCPUBenchmark,
		},
		// Run video_encode_accelerator_unittest to generate SSIM/PSNR scores (objective quality metrics).
		testArgs{
			testFilter:  "SimpleEncode/*/0",
			extraArgs:   []string{fmt.Sprintf("--frame_stats=%s", frameStatsPath)},
			bintestMode: runSync,
		},
	)

	p := &perf.Values{}

	// Analyze FPS and report metrics.
	if err := analyzeFPS(p, sName, fpsLogPath); err != nil {
		s.Fatal(err)
	}

	// Analyze encode latency and report metrics.
	if err := analyzeEncodeLatency(p, sName, latencyLogPath); err != nil {
		s.Fatal(err)
	}

	// Analyze CPU usage and report metrics.
	if err := analyzeCPUUsage(p, sName, cpuLogPath); err != nil {
		s.Fatal(err)
	}

	// Analyze SSIM/PSNR scores and report metrics.
	if err := analyzeFrameStats(p, sName, frameStatsPath); err != nil {
		s.Fatal(err)
	}
	p.Save(s.OutDir())
}

// getResultFilePath is the helper function to get the log path for recoding perf data.
func getResultFilePath(outDir string, name string, subtype string, suffix string) string {
	return filepath.Join(outDir, fmt.Sprintf("%s_%s_%s", name, subtype, suffix))
}
