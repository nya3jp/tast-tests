// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package decode provides common code to run Chrome binary tests for video decoding.
package decode

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/video/lib/arctest"
	"chromiumos/tast/local/bundles/cros/video/lib/cpu"
	"chromiumos/tast/local/bundles/cros/video/lib/logging"
	"chromiumos/tast/local/bundles/cros/video/lib/videotype"
	"chromiumos/tast/local/chrome/bintest"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/local/testexec"
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

// binaryType represents the type of test binary.
type binaryType int

const (
	// vdaUnittest represents video_decode_accelerator_unittest.
	vdaUnittest binaryType = iota
	// arcVideoDecoderTest represents arcvideodecoder_test.
	arcVideoDecoderTest
)

// testConfig stores test configuration to run video_decode_accelerator_unittest and arcvideodecoder_test.
type testConfig struct {
	// binType indicates the test binary type of this configuration.
	binType binaryType
	// testData stores the test video's name and metadata.
	// Used by all binaries.
	testData TestVideoData
	// dataPath stores the absolute path of the video file.
	// Used by all binaries.
	dataPath string
	// bufferMode indicates which buffer mode the unittest runs with.
	// Only used by video_decode_accelerator_unittest.
	bufferMode VDABufferMode
	// requireMD5Files indicates whether to prepare MD5 files for test.
	// Used by all binaries.
	// TODO(crbug.com/953118) Move metadata parsing code into the ARC Tast test once video_decode_accelerator_unittest
	//                        is deprecated. The new video_decode_accelerator_tests use the metadata file directly.
	requireMD5Files bool
	// thumbnailOutputDir is a directory for the unittest to output thumbnail.
	// If unspecified, the unittest outputs no thumbnail.
	// Only used by video_decode_accelerator_unittest.
	thumbnailOutputDir string
	// testFilter specifies test pattern the test can run.
	// If unspecified, the unittest runs all tests.
	// Used by all binaries.
	testFilter string
}

// toArgsList converts testConfig to a list of argument strings according to binType.
func (t *testConfig) toArgsList() (args []string) {
	if t.binType == vdaUnittest {
		// video_decode_accelerator_unittest only.
		args = append(args, logging.ChromeVmoduleFlag(), "--ozone-platform=gbm", t.testData.toVDAArg(t.dataPath))
		if t.bufferMode == ImportBuffer {
			args = append(args, "--test_import", "--frame_validator=check")
		}
		if t.thumbnailOutputDir != "" {
			args = append(args, fmt.Sprintf("--thumbnail_output_dir=%s", t.thumbnailOutputDir))
		}
	} else {
		// arcvideodecoder_test only.
		dataPath := filepath.Join(arc.ARCTmpDirPath, filepath.Base(t.dataPath))
		args = append(args, t.testData.toVDAArg(dataPath))
	}

	// Common arguments.
	if t.testFilter != "" {
		args = append(args, fmt.Sprintf("--gtest_filter=%s", t.testFilter))
	}
	return args
}

// decodeMetadata stores parsed metadata from test video JSON files, which are external files located in
// gs://chromiumos-test-assets-public/tast/cros/video/, e.g. test-25fps.h264.json.
type decodeMetadata struct {
	Profile            string   `json:"profile"`
	Width              int      `json:"width"`
	Height             int      `json:"height"`
	FrameRate          int      `json:"frame_rate"`
	NumFrames          int      `json:"num_frames"`
	NumFragments       int      `json:"num_fragments"`
	MD5Checksums       []string `json:"md5_checksums"`
	ThumbnailChecksums []string `json:"thumbnail_checksums"`
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

// writeLinesToFile writes lines to filepath line by line.
func writeLinesToFile(lines []string, filepath string) error {
	return ioutil.WriteFile(filepath, []byte(strings.Join(lines, "\n")+"\n"), 0644)
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

	args := cfg.toArgsList()
	const exec = "video_decode_accelerator_unittest"
	if ts, err := bintest.Run(shortCtx, exec, args, s.OutDir()); err != nil {
		s.Errorf("Failed to run %v with video %s: %v", exec, cfg.dataPath, err)
		for _, t := range ts {
			s.Error(t, " failed")
		}
	}
}

// runARCVideoTest runs arcvideodecoder_test in ARC.
// It fails if arcvideodecoder_test fails.
func runARCVideoTest(ctx context.Context, s *testing.State, a *arc.ARC, cfg testConfig) {
	shortCtx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	pushFiles := []string{cfg.dataPath}

	if cfg.requireMD5Files {
		// Parse JSON metadata.
		// TODO(johnylin) Adapt ARC decoder test to use the json file directly.
		jf, err := os.Open(cfg.dataPath + ".json")
		if err != nil {
			s.Fatal("Failed to open JSON file: ", err)
		}
		defer jf.Close()

		var md decodeMetadata
		if err := json.NewDecoder(jf).Decode(&md); err != nil {
			s.Fatal("Failed to parse metadata from JSON file: ", err)
		}

		// Prepare frames MD5 file.
		frameMD5Path := cfg.dataPath + ".frames.md5"
		s.Logf("Preparing frames MD5 file %v from JSON metadata", frameMD5Path)
		if err := writeLinesToFile(md.MD5Checksums, frameMD5Path); err != nil {
			s.Fatalf("Failed to prepare frames MD5 file %s: %v", frameMD5Path, err)
		}
		defer os.Remove(frameMD5Path)

		pushFiles = append(pushFiles, frameMD5Path)
	}

	// Push files to ARC container.
	for _, pushFile := range pushFiles {
		arcPath, err := a.PushFileToTmpDir(shortCtx, pushFile)
		if err != nil {
			s.Fatal("Failed to push video stream to ARC: ", err)
		}
		defer a.Command(ctx, "rm", arcPath).Run()
	}

	args := cfg.toArgsList()

	// Push test binary files to ARC container. For x86_64 device we might install both amd64 and x86 binaries.
	const testexec = "arcvideodecoder_test"
	execs, err := a.PushTestBinaryToTmpDir(shortCtx, testexec)
	if err != nil {
		s.Fatal("Failed to push test binary to ARC: ", err)
	}
	if len(execs) == 0 {
		s.Fatal("Test binary is not found in ", arc.TestBinaryDirPath)
	}
	defer a.Command(ctx, "rm", execs...).Run()

	// Execute binary in ARC.
	for _, exec := range execs {
		outputLogFile := filepath.Join(s.OutDir(), fmt.Sprintf("output_%s_%s.log", filepath.Base(exec), time.Now().Format("20060102-150405")))
		outFile, err := os.Create(outputLogFile)
		if err != nil {
			s.Fatal("failed to create output log file: ", err)
		}
		defer outFile.Close()

		if err := arctest.RunARCBinary(shortCtx, a, exec, args, s.OutDir(), outFile); err != nil {
			s.Errorf("Failed to run %v: %v", exec, err)
		}
	}
}

// RunAccelVideoTestNew runs video_decode_accelerator_tests with the specified video file.
// TODO(crbug.com/933034) Rename this function once the video_decode_accelerator_unittest
// have been completely replaced.
func RunAccelVideoTestNew(ctx context.Context, s *testing.State, filename string) {
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

	var args []string
	// ARC++ is disabled on devices that don't support IMPORT mode. As frame
	// validation also requires IMPORT mode we need to disable it on these
	// devices. (cf. crbug.com/881729)
	if !arc.Supported() {
		args = append(args, "--disable_validator")
	}
	args = append(args, s.DataPath(filename), s.DataPath(filename+".json"))

	const exec = "video_decode_accelerator_tests"
	if ts, err := bintest.Run(shortCtx, exec, args, s.OutDir()); err != nil {
		s.Errorf("Failed to run %v with video %s: %v", exec, filename, err)
		for _, t := range ts {
			s.Error(t, " failed")
		}
	}
}

// RunAccelVideoPerfTest runs video_decode_accelerator_perf_tests with the
// specified video file. Both capped and uncapped performance is measured.
// - Uncapped performance: the specified test video is decoded from start to
// finish as fast as possible. This provides an estimate of the decoder's max
// performance (e.g. the maximum FPS).
// - Capped decoder performance: uses a more realistic environment by decoding
// the test video from start to finish at its actual frame rate. Rendering is
// simulated and late frames are dropped.
// The test binary is run twice. Once to measure both capped and uncapped
// performance, once to measure CPU usage while running the capped performance
// test.
func RunAccelVideoPerfTest(ctx context.Context, s *testing.State, filename string) {
	const (
		// Name of the capped performance test.
		cappedTestname = "MeasureCappedPerformance"
		// Name of the uncapped performance test.
		uncappedTestname = "MeasureUncappedPerformance"
		// Time to wait for CPU to stabilize after launching test binary.
		stabilizeDuration = 1 * time.Second
		// Duration of the interval during which CPU usage will be measured.
		measureDuration = 20 * time.Second
	)

	shortCtx, cleanupBenchmark, err := cpu.SetUpBenchmark(ctx)
	if err != nil {
		s.Fatal("Failed to set up benchmark mode: ", err)
	}
	defer cleanupBenchmark()

	// Only a single process can have access to the GPU, so we are required to
	// call "stop ui" at the start of the test. This will shut down the chrome
	// process and allow us to claim ownership of the GPU.
	if err := upstart.StopJob(shortCtx, "ui"); err != nil {
		s.Error("Failed to stop ui: ", err)
	}
	defer upstart.EnsureJobRunning(ctx, "ui")

	// Test 1: Measure capped and uncapped performance.
	args := []string{
		"--output_folder=" + s.OutDir(),
		s.DataPath(filename),
		s.DataPath(filename + ".json"),
	}

	const exec = "video_decode_accelerator_perf_tests"
	if ts, err := bintest.Run(shortCtx, exec, args, s.OutDir()); err != nil {
		s.Errorf("Failed to run %v with video %s: %v", exec, filename, err)
		for _, t := range ts {
			s.Error(t, " failed")
		}
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
	cappedArgs := append(args, "--gtest_filter="+cappedTestname, "--gtest_repeat=-1")
	cmd, err := bintest.RunAsync(shortCtx, exec, cappedArgs, nil, s.OutDir())
	if err != nil {
		s.Fatalf("Failed to run %v: %v", exec, err)
	}

	s.Logf("Sleeping %v to wait for CPU usage to stabilize", stabilizeDuration.Round(time.Second))
	if err := testing.Sleep(shortCtx, stabilizeDuration); err != nil {
		s.Fatal("Failed waiting for CPU usage to stabilize: ", err)
	}

	s.Logf("Sleeping %v to measure CPU usage", measureDuration.Round(time.Second))
	cpuUsage, err := cpu.MeasureUsage(shortCtx, measureDuration)
	if err != nil {
		s.Fatal("Failed to measure CPU usage: ", err)
	}

	// We got our measurements, now kill the process. After killing a process we
	// still need to wait for all resources to get released.
	if err := cmd.Kill(); err != nil {
		s.Fatalf("Failed to kill %v: %v", exec, err)
	}
	if err := cmd.Wait(); err != nil {
		ws, ok := testexec.GetWaitStatus(err)
		if !ok || !ws.Signaled() || ws.Signal() != syscall.SIGKILL {
			s.Fatalf("Failed to run %v: %v", exec, err)
		}
	}

	// TODO(dstaessens@): Remove "tast_" prefix after removing video_VDAPerf in autotest.
	p.Set(perf.Metric{
		Name:      "tast_cpu_usage",
		Unit:      "ratio",
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
		binType:            vdaUnittest,
		testData:           testData,
		dataPath:           s.DataPath(testData.Name),
		bufferMode:         bufferMode,
		requireMD5Files:    true,
		thumbnailOutputDir: s.OutDir(),
	})
}

// RunAccelVideoSanityTest runs NoCrash test in video_decode_accelerator_unittest.
// NoCrash test only fails if video decoder accelerator crashes.
func RunAccelVideoSanityTest(ctx context.Context, s *testing.State, testData TestVideoData) {
	vl, err := logging.NewVideoLogger()
	if err != nil {
		s.Fatal("Failed to set values for verbose logging")
	}
	defer vl.Close()

	runAccelVideoTest(ctx, s, testConfig{
		binType:    vdaUnittest,
		testData:   testData,
		dataPath:   s.DataPath(testData.Name),
		bufferMode: AllocateBuffer,
		testFilter: "VideoDecodeAcceleratorTest.NoCrash",
	})
}

// RunAllARCVideoTests runs all tests in arcvideodecoder_test.
func RunAllARCVideoTests(ctx context.Context, s *testing.State, a *arc.ARC, testData TestVideoData) {
	vl, err := logging.NewVideoLogger()
	if err != nil {
		s.Fatal("Failed to set values for verbose logging")
	}
	defer vl.Close()

	runARCVideoTest(ctx, s, a, testConfig{
		binType:         arcVideoDecoderTest,
		testData:        testData,
		dataPath:        s.DataPath(testData.Name),
		requireMD5Files: true,
	})
}
