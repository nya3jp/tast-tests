// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package decode provides common code to run Chrome binary tests for video decoding.
package decode

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/video/lib/arctest"
	"chromiumos/tast/local/bundles/cros/video/lib/logging"
	"chromiumos/tast/local/bundles/cros/video/lib/videotype"
	"chromiumos/tast/local/chrome/bintest"
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

// testConfig stores test configuration to run video_decode_accelerator_unittest and arcvideodecoder_test.
// arcvideodecoder_test only regard testData, dataPath, and testFilter now.
type testConfig struct {
	// testData stores the test video's name and metadata.
	testData TestVideoData
	// dataPath stores the absolute path of the video file.
	dataPath string
	// bufferMode indicates which buffer mode the unittest runs with.
	bufferMode VDABufferMode
	// thumbnailOutputDir is a directory for the unittest to output thumbnail.
	// If unspecified, the unittest outputs no thumbnail.
	thumbnailOutputDir string
	// testFilter specifies test pattern the test can run.
	// If unspecified, the unittest runs all tests.
	testFilter string
}

// toArgsList converts testConfig to a list of argument strings of video_decode_accelerator_unittest.
func (t *testConfig) toArgsList() []string {
	args := []string{
		logging.ChromeVmoduleFlag(),
		"--ozone-platform=gbm",
		t.testData.toVDAArg(t.dataPath),
	}
	if t.bufferMode == ImportBuffer {
		args = append(args, "--test_import", "--frame_validator=check")
	}
	if t.thumbnailOutputDir != "" {
		args = append(args, fmt.Sprintf("--thumbnail_output_dir=%s", t.thumbnailOutputDir))
	}
	if t.testFilter != "" {
		args = append(args, fmt.Sprintf("--gtest_filter=%s", t.testFilter))
	}
	return args
}

// toARCArgsList converts testConfig to a list of argument strings of arcvideodecoder_test in ARC.
func (t *testConfig) toARCArgsList(arcDataPath string) []string {
	args := []string{t.testData.toVDAArg(arcDataPath)}
	if t.testFilter != "" {
		args = append(args, fmt.Sprintf("--gtest_filter=%s", t.testFilter))
	}
	return args
}

// DataFiles returns a list of required files that tests that use this package
// should include in their Data fields.
func DataFiles(profile videotype.CodecProfile, mode VDABufferMode) []string {
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
	files := []string{fname, fname + ".md5"}
	if mode == ImportBuffer {
		files = append(files, fname+".frames.md5")
	}

	return files
}

// runAccelVideoTest runs video_decode_accelerator_unittest with given testConfig.
func runAccelVideoTest(ctx context.Context, s *testing.State, cfg testConfig) {
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
	shortCtx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	// Push video stream file to ARC container.
	arcVideoPath, err := a.PushFileToTmpDir(shortCtx, cfg.dataPath)
	if err != nil {
		s.Fatal("Failed to push video stream to ARC: ", err)
	}
	defer a.Command(ctx, "rm", arcVideoPath).Run()

	args := cfg.toARCArgsList(arcVideoPath)

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
		if err := arctest.RunARCBinary(shortCtx, a, exec, args, s.OutDir()); err != nil {
			s.Errorf("Failed to run %v: %v", exec, err)
		}
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
		testData: testData,
		dataPath: s.DataPath(testData.Name),
	})
}
