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
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/gtest"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/local/media/logging"
	"chromiumos/tast/local/media/videotype"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

// cpuLog is the name of log file recording CPU usage.
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
	// Level is the requested output level. This value is optional and currently only used by the H264 codec. The value
	// should be aligned with the H264LevelIDC enum in https://cs.chromium.org/chromium/src/media/video/h264_parser.h,
	// as well as level_idc(u8) definition of sequence parameter set data in official H264 spec.
	Level int
}

// InputStorageMode represents the input buffer storage type of video_encode_accelerator_unittest.
type InputStorageMode int

const (
	// SharedMemory is a mode where video encode accelerator uses MEM-backed input VideoFrame on encode.
	SharedMemory InputStorageMode = iota
	// DMABuf is a mode where video encode accelerator uses DmaBuf-backed input VideoFrame on encode.
	DMABuf
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
	// testFilter specifies test pattern in googletest style for the unittest to run and will be passed with "--gtest_filter" (see go/gtest-running-subset).
	// If unspecified, the unittest runs all tests.
	testFilter string
	// extraArgs is the additional arguments to pass video_encode_accelerator_unittest, for example, "--native_input".
	extraArgs []string
	// measureCPU indicates whether to measure CPU usage while running binary and save as a perf metric.
	measureCPU bool
	// measureDuration specifies how long to measure CPU usage when measureCPU is set.
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
func runAccelVideoTest(ctx context.Context, s *testing.State, mode testMode, opts TestOptions, bas ...binArgs) {
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
	streamPath, err := PrepareYUV(shortCtx, s.DataPath(params.Name), opts.PixelFormat, params.Size)
	if err != nil {
		s.Fatal("Failed to prepare YUV file: ", err)
	}
	defer os.Remove(streamPath)
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
		CreateStreamDataArg(params, opts.Profile, opts.PixelFormat, streamPath, outPath),
		"--ozone-platform=gbm",

		// The default timeout for test launcher is 45 seconds, which is not enough for some test cases.
		// Considering we already manage timeout by Tast context.Context, we don't need another timeout at test launcher.
		// Set a huge timeout (3600000 milliseconds, 1 hour) here.
		"--test-launcher-timeout=3600000",
	}
	if opts.InputMode == DMABuf {
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
		if ba.measureCPU {
			cpuUsage, err := cpu.MeasureProcessCPU(shortCtx, ba.measureDuration, cpu.KillProcess, t)
			if err != nil {
				s.Fatalf("Failed to run (measure CPU) %v: %v", exec, err)
			}
			// TODO(akahuang): Don't write CPU usage to disk, as this can increase test flakiness.
			cpuLogPath := filepath.Join(s.OutDir(), cpuLog)
			str := fmt.Sprintf("%f", cpuUsage)
			if err := ioutil.WriteFile(cpuLogPath, []byte(str), 0644); err != nil {
				s.Fatal("Failed to write CPU usage to file: ", err)
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

// runARCVideoTest runs arcvideoencoder_test in ARC.
// It pushes the binary files with different ABI and testing video data into ARC, and runs each binary for each binArgs.
// pv is optional value, passed when we run performance test and record measurement value.
// Note: pv must be provided when measureCPU is set at binArgs.
func runARCVideoTest(ctx context.Context, s *testing.State, a *arc.ARC, opts TestOptions, pv *perf.Values, bas ...binArgs) {
	// Prepare video stream.
	params := opts.Params
	streamPath, err := PrepareYUV(ctx, s.DataPath(params.Name), opts.PixelFormat, params.Size)
	if err != nil {
		s.Fatal("Failed to prepare YUV file: ", err)
	}
	defer os.Remove(streamPath)

	// Push video stream file to ARC container.
	arcStreamPath, err := a.PushFileToTmpDir(ctx, streamPath)
	if err != nil {
		s.Fatal("Failed to push video stream to ARC: ", err)
	}
	defer a.Command(ctx, "rm", arcStreamPath).Run()

	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	if opts.Profile != videotype.H264Prof {
		s.Fatalf("Profile (%d) is not supported", opts.Profile)
	}
	encodeOutFile := strings.TrimSuffix(params.Name, ".vp9.webm") + ".h264"
	outPath := filepath.Join(arc.ARCTmpDirPath, encodeOutFile)
	defer a.Command(ctx, "rm", outPath).Run()

	// Push test binary files to ARC container. For x86_64 device we might install both amd64 and x86 binaries.
	execs, err := a.PushTestBinaryToTmpDir(ctx, "arcvideoencoder_test")
	if err != nil {
		s.Fatal("Failed to push test binary to ARC: ", err)
	}
	if len(execs) == 0 {
		s.Fatal("Test binary is not found in ", arc.TestBinaryDirPath)
	}
	defer a.Command(ctx, "rm", execs...).Run()

	commonArgs := []string{
		CreateStreamDataArg(params, opts.Profile, opts.PixelFormat, arcStreamPath, outPath),
	}
	for _, exec := range execs {
		for _, ba := range bas {
			if err := runARCBinaryWithArgs(ctx, s, a, exec, commonArgs, ba, pv); err != nil {
				s.Errorf("Failed to run %v with %v: %v", exec, ba, err)
			}
		}
	}
}

// runARCBinaryWithArgs runs arcvideoencoder_test binary with one binary argument.
// pv is optional value, passed when we run performance test and record measurement value.
// Note: pv must be provided when measureCPU is set at binArgs.
func runARCBinaryWithArgs(ctx context.Context, s *testing.State, a *arc.ARC, exec string, commonArgs []string, ba binArgs, pv *perf.Values) error {
	outputLogFile := filepath.Join(s.OutDir(), fmt.Sprintf("output_%s_%s.log", filepath.Base(exec), time.Now().Format("20060102-150405")))
	t := gtest.New(
		exec,
		gtest.Logfile(outputLogFile),
		gtest.Filter(ba.testFilter),
		gtest.ExtraArgs(append(append([]string{}, commonArgs...), ba.extraArgs...)...),
		gtest.ARC(a))

	schemaName := filepath.Base(exec)
	if ba.measureCPU {
		if pv == nil {
			return errors.New("pv should not be nil when measuring CPU usage")
		}

		cpuUsage, err := cpu.MeasureProcessCPU(ctx, ba.measureDuration, cpu.KillProcess, t)
		if err != nil {
			return errors.Wrapf(err, "failed to run (measure CPU) %v: %v", exec, err)
		}
		// TODO(akahuang): Don't write CPU usage to disk, as this can increase test flakiness.
		cpuLogPath := filepath.Join(s.OutDir(), cpuLog)
		str := fmt.Sprintf("%f", cpuUsage)
		if err := ioutil.WriteFile(cpuLogPath, []byte(str), 0644); err != nil {
			return errors.Wrap(err, "failed to write CPU usage to file")
		}

		if err := reportCPUUsage(pv, schemaName, cpuLogPath); err != nil {
			return errors.Wrap(err, "failed to report CPU usage")
		}
	} else {
		if _, err := t.Run(ctx); err != nil {
			return errors.Wrapf(err, "failed to run %v", exec)
		}

		// Parse the performance result.
		if pv != nil {
			if err := reportFPS(pv, schemaName, outputLogFile); err != nil {
				return errors.Wrap(err, "failed to report FPS value")
			}
			if err := reportEncodeLatency(pv, schemaName, outputLogFile); err != nil {
				return errors.Wrap(err, "failed to report encode latency")
			}
		}
	}
	return nil
}

// CreateStreamDataArg creates an argument of video_encode_accelerator_unittest from profile, dataPath and outFile.
func CreateStreamDataArg(params StreamParams, profile videotype.CodecProfile, pixelFormat videotype.PixelFormat, dataPath, outFile string) string {
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
	if params.Level != 0 {
		streamDataArgs += fmt.Sprintf(":%d", params.Level)
	}
	return streamDataArgs
}

// RunAllAccelVideoTests runs all tests in video_encode_accelerator_unittest.
func RunAllAccelVideoTests(ctx context.Context, s *testing.State, opts TestOptions) {
	RunAllAccelVideoTestsWithFilter(ctx, s, opts, "")
}

// RunAllAccelVideoTestsWithFilter runs all tests in video_encode_accelerator_unittest with the test pattern in googletest style.
func RunAllAccelVideoTestsWithFilter(ctx context.Context, s *testing.State, opts TestOptions, testFilter string) {
	vl, err := logging.NewVideoLogger()
	if err != nil {
		s.Fatal("Failed to set values for verbose logging")
	}
	defer vl.Close()

	runAccelVideoTest(ctx, s, functionalTest, opts, binArgs{testFilter: testFilter})
}

// RunAccelVideoPerfTest runs video_encode_accelerator_unittest multiple times with different arguments to gather perf metrics.
func RunAccelVideoPerfTest(ctx context.Context, s *testing.State, opts TestOptions) {
	const (
		// testLogSuffix is the log name suffix of dumping log from test binary.
		testLogSuffix = "test.log"
		// frameStatsSuffix is the log name suffix of frame statistics.
		frameStatsSuffix = "frame-data.csv"
		// cpuEncodeFrames is the number of encoded frames for CPU usage test. It should be high enouch to run for measurement duration.
		cpuEncodeFrames = 10000
		// duration of the interval during which CPU usage will be measured.
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

	frameStatsPath := getResultFilePath(s.OutDir(), schemaName, "quality", frameStatsSuffix)

	runAccelVideoTest(ctx, s, performanceTest, opts,
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
		// Run video_encode_accelerator_unittest to get CPU usage under specified frame rate.
		binArgs{
			testFilter: "SimpleEncode/*/0",
			extraArgs: []string{fmt.Sprintf("--num_frames_to_encode=%d", cpuEncodeFrames),
				"--run_at_fps"},
			measureCPU:      true,
			measureDuration: measureDuration,
		},
	)

	p := perf.NewValues()

	if err := reportFPS(p, schemaName, fpsLogPath); err != nil {
		s.Fatal("Failed to report FPS value: ", err)
	}

	if err := reportEncodeLatency(p, schemaName, latencyLogPath); err != nil {
		s.Fatal("Failed to report encode latency: ", err)
	}

	if err := reportCPUUsage(p, schemaName, cpuLogPath); err != nil {
		s.Fatal("Failed to report CPU usage: ", err)
	}

	if err := reportFrameStats(p, schemaName, frameStatsPath); err != nil {
		s.Fatal("Failed to report frame stats: ", err)
	}
	p.Save(s.OutDir())
}

// RunARCVideoTest runs all non-perf tests of arcvideoencoder_test in ARC.
func RunARCVideoTest(ctx context.Context, s *testing.State, a *arc.ARC, opts TestOptions) {
	vl, err := logging.NewVideoLogger()
	if err != nil {
		s.Fatal("Failed to set values for verbose logging: ", err)
	}
	defer vl.Close()

	runARCVideoTest(ctx, s, a, opts, nil, binArgs{testFilter: "ArcVideoEncoderE2ETest.Test*"})
}

// RunARCPerfVideoTest runs all perf tests of arcvideoencoder_test in ARC.
func RunARCPerfVideoTest(ctx context.Context, s *testing.State, a *arc.ARC, opts TestOptions) {
	const (
		// duration of the interval during which CPU usage will be measured.
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

	if err := cpu.WaitUntilIdle(ctx); err != nil {
		s.Fatal("Failed waiting for CPU to become idle: ", err)
	}

	pv := perf.NewValues()
	runARCVideoTest(ctx, s, a, opts, pv,
		// Measure FPS and latency.
		binArgs{
			testFilter: "ArcVideoEncoderE2ETest.Perf*",
		},
		// Measure CPU usage.
		binArgs{
			testFilter:      "ArcVideoEncoderE2ETest.TestSimpleEncode",
			extraArgs:       []string{"--run_at_fps", "--num_encoded_frames=10000"},
			measureCPU:      true,
			measureDuration: measureDuration,
		})
	pv.Save(s.OutDir())
}

// getResultFilePath is the helper function to get the log path for recoding perf data.
func getResultFilePath(outDir, name, subtype, suffix string) string {
	return filepath.Join(outDir, fmt.Sprintf("%s_%s_%s", name, subtype, suffix))
}
