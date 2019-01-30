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

// cpuLog is the log name of recording CPU usage.
const cpuLog = "cpu.log"

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
	// DMABuf is a mode where video encode accelerator uses DmaBuf-backed input VideoFrame on encode.
	DMABuf
)

// measurementMode represents the measurement mode while executing video_encode_accelerator_unittest binary.
type measurementMode int

const (
	// noMeasurement means no measuring step is needed.
	noMeasurement measurementMode = iota
	// cpuMeasurement is a mode to measure CPU usage while running binary asynchronously.
	cpuMeasurement
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

// binArgs is the arguments and the modes for executing video_encode_accelerator_unittest binary.
type binArgs struct {
	// testFilter specifies test pattern in googletest style for the unittest to run
	// (see https://github.com/google/googletest/blob/master/googletest/docs/primer.md).
	// If unspecified, the unittest runs all tests.
	testFilter string
	// extraArgs is the additional arguments to pass video_encode_accelerator_unittest, for example, "--native_input".
	extraArgs []string
	// measurement specifies the measurement mode while executing video_encode_accelerator_unittest binary.
	measurement measurementMode
}

// runAccelVideoTest runs video_encode_accelerator_unittest for each binArgs.
// It fails if video_encode_accelerator_unittest fails.
func runAccelVideoTest(ctx context.Context, s *testing.State, opts TestOptions, bas ...binArgs) {
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
	commonArgs := []string{logging.ChromeVmoduleFlag(),
		createStreamDataArg(params, opts.Profile, opts.PixelFormat, streamPath, outPath),
		"--ozone-platform=gbm",
	}
	if opts.InputMode == DMABuf {
		commonArgs = append(commonArgs, "--native_input")
	}

	const exec = "video_encode_accelerator_unittest"
	for _, ba := range bas {
		args := append(commonArgs, ba.extraArgs...)
		if ba.testFilter != "" {
			args = append(args, "--gtest_filter="+ba.testFilter)
		}
		switch ba.measurement {
		case noMeasurement:
			if ts, err := bintest.Run(shortCtx, exec, args, s.OutDir()); err != nil {
				for _, t := range ts {
					s.Error(t, " failed")
				}
				s.Fatalf("Failed to run %v: %v", exec, err)
			}
		case cpuMeasurement:
			if err := runBinaryTestWithCPUMeasurement(shortCtx, exec, args, s.OutDir()); err != nil {
				s.Fatalf("Failed to run (CPU measurement) %v: %v", exec, err)
			}
		}
	}
}

// runBinaryTestWithCPUMeasurement measures CPU usage while running video_encode_accelerator_unittest asynchronously.
func runBinaryTestWithCPUMeasurement(ctx context.Context, exec string, args []string, outDir string) error {
	const (
		// stabilizationDuration is the time to wait for CPU to stabilize after launching test binary.
		stabilizationDuration = 1 * time.Second
		// measurementDuration is the duration of the interval during which CPU usage will be measured.
		measurementDuration = 1 * time.Second
	)

	cpuUsage, err := cpu.MeasureUsageOnRunningBinary(ctx, exec, args, outDir, stabilizationDuration, measurementDuration, false /* killAfterMeasure */)
	if err != nil {
		return errors.Wrapf(err, "failed to measure cpu usage on running %v", exec)
	}

	// To align with other measurement procedures, writing cpuUsage into a file located in output directory here and then it will be read later as well as other metrics.
	str := fmt.Sprintf("%f", cpuUsage)
	logPath := filepath.Join(outDir, cpuLog)
	if err := ioutil.WriteFile(logPath, []byte(str), 0644); err != nil {
		return errors.Wrapf(err, "failed to write file: %s", logPath)
	}

	return nil
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

// RunAllAccelVideoTests runs all tests in video_encode_accelerator_unittest.
func RunAllAccelVideoTests(ctx context.Context, s *testing.State, opts TestOptions) {
	runAccelVideoTest(ctx, s, opts, binArgs{measurement: noMeasurement})
}

// RunAccelVideoPerfTest runs video_encode_accelerator_unittest multiple times with different arguments to gather perf metrics.
func RunAccelVideoPerfTest(ctx context.Context, s *testing.State, opts TestOptions) {
	const (
		// testLogSuffix is the log name suffix of dumping log from test binary.
		testLogSuffix = "test.log"
		// frameStatsSuffix is the log name suffix of frame statistics.
		frameStatsSuffix = "frame-data.csv"
	)

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
		binArgs{
			testFilter:  "EncoderPerf/*/0",
			extraArgs:   []string{fmt.Sprintf("--output_log=%s", fpsLogPath)},
			measurement: noMeasurement,
		},
		// Run video_encode_accelerator_unittest to get CPU usage and encode latency under specified frame rate.
		binArgs{
			testFilter:  "SimpleEncode/*/0",
			extraArgs:   []string{fmt.Sprintf("--output_log=%s", latencyLogPath), "--run_at_fps", "--measure_latency"},
			measurement: cpuMeasurement,
		},
		// Run video_encode_accelerator_unittest to generate SSIM/PSNR scores (objective quality metrics).
		binArgs{
			testFilter:  "SimpleEncode/*/0",
			extraArgs:   []string{fmt.Sprintf("--frame_stats=%s", frameStatsPath)},
			measurement: noMeasurement,
		},
	)

	p := perf.NewValues()

	if err := reportFPS(p, sName, fpsLogPath); err != nil {
		s.Fatal("Failed to report FPS value: ", err)
	}

	if err := reportEncodeLatency(p, sName, latencyLogPath); err != nil {
		s.Fatal("Failed to report encode latency: ", err)
	}

	if err := reportCPUUsage(p, sName, cpuLogPath); err != nil {
		s.Fatal("Failed to report CPU usage: ", err)
	}

	if err := reportFrameStats(p, sName, frameStatsPath); err != nil {
		s.Fatal("Failed to report frame stats: ", err)
	}
	p.Save(s.OutDir())
}

// getResultFilePath is the helper function to get the log path for recoding perf data.
func getResultFilePath(outDir, name, subtype, suffix string) string {
	return filepath.Join(outDir, fmt.Sprintf("%s_%s_%s", name, subtype, suffix))
}
