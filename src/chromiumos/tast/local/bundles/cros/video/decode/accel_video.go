// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package decode provides common code to run Chrome binary tests for video decoding.
package decode

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/ctxutil"
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

// TestVideoData represents a test video data file for video_decode_accelerator_unittest with
// metadata.
type TestVideoData struct {
	// Name is the file name of input video file.
	Name string
	// Size is the width and height of input stream data.
	Size videotype.Size
	// NumFrames is the number of picture frames in the file.
	NumFrames int
	// NumFragments is NALU (h264) or frame (VP8/9) count in the stream.
	NumFragments int
	// MinFPSWithRender is the minimum frames/second speeds expected to be
	// achieved with rendering to the screen.
	MinFPSWithRender int
	// MinFPSNoRender is the minimum frames/second speeds expected to be
	// achieved without rendering to the screen.
	// In other words, this is the expected speed for decoding.
	MinFPSNoRender int
	// Profile is the VideoCodecProfile set during Initialization.
	Profile videotype.CodecProfile
}

// toVDAArg returns a string that can be used for an argument of video_decode_accelerator_unittest.
// dataPath is the absolute path of the video file.
func (d *TestVideoData) toVDAArg(dataPath string) string {
	streamDataArgs := fmt.Sprintf("--test_video_data=%s:%d:%d:%d:%d:%d:%d:%d",
		dataPath, d.Size.W, d.Size.H, d.NumFrames, d.NumFragments,
		d.MinFPSWithRender, d.MinFPSNoRender, int(d.Profile))
	return streamDataArgs
}

// VDABufferMode represents a buffer mode of video_decode_accelerator_unittest.
type VDABufferMode int

const (
	// AllocateBuffer is a mode where video decode accelerator allocates buffer by itself.
	AllocateBuffer VDABufferMode = iota
	// ImportBuffer is a mode where video decode accelerator uses provided buffers.
	// In this mode, we run tests using frame validator.
	ImportBuffer
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

// testConfig stores test configuration to run video_decode_accelerator_unittest.
type testConfig struct {
	// testData stores the test video's name and metadata.
	testData TestVideoData
	// dataPath stores the absolute path of the video file.
	dataPath string
	// bufferMode indicates which buffer mode the unittest runs with.
	bufferMode VDABufferMode
	// requireMD5Files indicates whether to prepare MD5 files for test.
	// TODO(crbug.com/953118) Move metadata parsing code into the ARC Tast test once video_decode_accelerator_unittest
	//                        is deprecated. The new video_decode_accelerator_tests use the metadata file directly.
	requireMD5Files bool
	// thumbnailOutputDir is a directory for the unittest to output thumbnail.
	// If unspecified, the unittest outputs no thumbnail.
	thumbnailOutputDir string
	// testFilter specifies test pattern the test can run.
	// If unspecified, the unittest runs all tests.
	testFilter string
}

// toArgsList converts testConfig to a list of argument strings.
func (t *testConfig) toArgsList() []string {
	args := []string{logging.ChromeVmoduleFlag(), "--ozone-platform=gbm", t.testData.toVDAArg(t.dataPath)}
	if t.bufferMode == ImportBuffer {
		args = append(args, "--test_import", "--frame_validator=check")
	}
	if t.thumbnailOutputDir != "" {
		args = append(args, "--thumbnail_output_dir="+t.thumbnailOutputDir)
	}
	return args
}

// DataFiles returns a list of required files that tests that use this package
// should include in their Data fields.
func DataFiles(profile videotype.CodecProfile) []string {
	var codec string
	switch profile {
	case videotype.H264Prof:
		codec = "h264"
	case videotype.VP8Prof:
		codec = "vp8"
	case videotype.VP9Prof:
		codec = "vp9"
	case videotype.VP9_2Prof:
		codec = "vp9_2"
	}

	fname := "test-25fps." + codec
	return []string{fname, fname + ".json"}
}

// runAccelVideoTest runs video_decode_accelerator_unittest with given testConfig.
func runAccelVideoTest(ctx context.Context, s *testing.State, cfg testConfig) {
	vl, err := logging.NewVideoLogger()
	if err != nil {
		s.Fatal("Failed to set values for verbose logging: ", err)
	}
	defer vl.Close()

	if cfg.requireMD5Files {
		// Parse JSON metadata.
		jf, err := os.Open(cfg.dataPath + ".json")
		if err != nil {
			s.Fatal("Failed to open JSON file: ", err)
		}
		defer jf.Close()

		// Note: decodeMetadata is declared in arc_accel_video.go under the same package.
		//       This is the intermediate state that decodeMetadata will be useless after
		//       video_decode_accelerator_unittest is deprecated.
		var md decodeMetadata
		if err := json.NewDecoder(jf).Decode(&md); err != nil {
			s.Fatal("Failed to parse metadata from JSON file: ", err)
		}

		// Prepare thumbnail MD5 file.
		md5Path := cfg.dataPath + ".md5"
		s.Logf("Preparing thumbnail MD5 file %v from JSON metadata", md5Path)
		if err := writeLinesToFile(md.ThumbnailChecksums, md5Path); err != nil {
			s.Fatalf("Failed to prepare thumbnail MD5 file %s: %v", md5Path, err)
		}
		defer os.Remove(md5Path)

		// Prepare frames MD5 file if config's bufferMode is ImportBuffer.
		if cfg.bufferMode == ImportBuffer {
			frameMD5Path := cfg.dataPath + ".frames.md5"
			s.Logf("Preparing frames MD5 file %v from JSON metadata", frameMD5Path)
			if err := writeLinesToFile(md.MD5Checksums, frameMD5Path); err != nil {
				s.Fatalf("Failed to prepare frames MD5 file %s: %v", frameMD5Path, err)
			}
			defer os.Remove(frameMD5Path)
		}
	}

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

	const exec = "video_decode_accelerator_unittest"
	if report, err := gtest.New(
		filepath.Join(chrome.BinTestDir, exec),
		gtest.Logfile(filepath.Join(s.OutDir(), exec+".log")),
		gtest.Filter(cfg.testFilter),
		gtest.ExtraArgs(cfg.toArgsList()...),
		gtest.UID(int(sysutil.ChronosUID)),
	).Run(ctx); err != nil {
		s.Errorf("Failed to run %v with video %s: %v", exec, cfg.dataPath, err)
		if report != nil {
			for _, name := range report.FailedTestNames() {
				s.Error(name, " failed")
			}
		}
	}
}

// RunAccelVideoTestNew runs video_decode_accelerator_tests with the specified video file.
// TODO(crbug.com/933034) Rename this function once the video_decode_accelerator_unittest
// have been completely replaced. decoderType specifies whether to run the tests against
// the VDA or VD based video decoder implementations.
func RunAccelVideoTestNew(ctx context.Context, s *testing.State, filename string, decoderType DecoderType) {
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
	if err := parseUncappedPerfMetrics(filepath.Join(s.OutDir(), uncappedTestname+".json"), p); err != nil {
		s.Fatal("Failed to parse uncapped performance metrics: ", err)
	}
	if err := parseCappedPerfMetrics(filepath.Join(s.OutDir(), cappedTestname+".json"), p); err != nil {
		s.Fatal("Failed to parse capped performance metrics: ", err)
	}

	// Test 2: Measure CPU usage while running capped performance test only.
	// TODO(dstaessens) Investigate collecting CPU usage during previous test.
	cpuUsage, err := cpu.MeasureProcessCPU(ctx, measureDuration,
		cpu.KillProcess, []*gtest.GTest{gtest.New(
			filepath.Join(chrome.BinTestDir, exec),
			gtest.Logfile(filepath.Join(s.OutDir(), exec+".2.log")),
			gtest.Filter("*"+cappedTestname),
			// Repeat enough times to run for full measurement duration. We don't
			// use -1 here as this can result in huge log files (b/138822793).
			gtest.Repeat(1000),
			gtest.ExtraArgs(args...),
			gtest.UID(int(sysutil.ChronosUID)),
		)})
	if err != nil {
		s.Fatalf("Failed to measure CPU usage %v: %v", exec, err)
	}

	// TODO(dstaessens@): Remove "tast_" prefix after removing video_VDAPerf in autotest.
	p.Set(perf.Metric{
		Name:      "tast_cpu_usage",
		Unit:      "percent",
		Direction: perf.SmallerIsBetter,
	}, cpuUsage)

	if err := p.Save(s.OutDir()); err != nil {
		s.Fatal("Failed to save performance metrics: ", err)
	}
}

// RunAllAccelVideoTest runs all tests in video_decode_accelerator_unittest with thumbnail stored in
// output directory.
func RunAllAccelVideoTest(ctx context.Context, s *testing.State, testData TestVideoData, bufferMode VDABufferMode) {
	vl, err := logging.NewVideoLogger()
	if err != nil {
		s.Fatal("Failed to set values for verbose logging")
	}
	defer vl.Close()

	runAccelVideoTest(ctx, s, testConfig{
		testData:           testData,
		dataPath:           s.DataPath(testData.Name),
		bufferMode:         bufferMode,
		requireMD5Files:    true,
		thumbnailOutputDir: s.OutDir(),
	})
}

// RunAccelVideoSanityTest runs NoCrash test in video_decode_accelerator_unittest.
// NoCrash test fails if video decoder's kernel driver crashes.
// The motivation of the sanity test: on certain devices, when playing VP9
// profile 1 or 3, the kernel crashed. Though the profile was not supported
// by the decoder, kernel driver should not crash in any circumstances.
// Refer to https://crbug.com/951189 for more detail.
func RunAccelVideoSanityTest(ctx context.Context, s *testing.State, testData TestVideoData) {
	vl, err := logging.NewVideoLogger()
	if err != nil {
		s.Fatal("Failed to set values for verbose logging")
	}
	defer vl.Close()

	runAccelVideoTest(ctx, s, testConfig{
		testData:   testData,
		dataPath:   s.DataPath(testData.Name),
		bufferMode: AllocateBuffer,
		testFilter: "VideoDecodeAcceleratorTest.NoCrash",
	})
}
