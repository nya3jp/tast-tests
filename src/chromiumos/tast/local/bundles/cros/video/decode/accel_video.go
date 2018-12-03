// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package decode provides common code to run Chrome binary tests for decoding.
package decode

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/video/lib/logging"
	"chromiumos/tast/local/bundles/cros/video/lib/videotype"
	"chromiumos/tast/local/chrome/bintest"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

// TestVideoData represents a test video data for video_decode_accelerator_unittest with metadata.
type TestVideoData struct {
	// Name is the file name of input video file.
	Name string
	// Size is the width and height of input stream data.
	Size videotype.Size
	// FrameNum is the number of picture frames in the file.
	FrameNum int
	// FragmentsNum is NALU (h264) or frame (VP8/9) count in the stream.
	FragmentsNum int
	// MinFPSWithRender is the minimum frames/second speeds expected to be
	// achieved with rendering to the screen.
	//   (the latter tests just decode speed).
	MinFPSWithRender int
	// MinFPSNoRender is the minimum frames/second speeds expected to be
	// achieved without rendering to the screen.
	// In other words, this is the expected speed for decoding.
	MinFPSNoRender int
	// Profile is the VideoCodecProfile set during Initialization.
	Profile videotype.CodecProfile
}

// toVDAArg returns string can be used for an argument of video_decode_accelerator_unittest.
func (d *TestVideoData) toVDAArg(dataPath string) string {
	streamDataArgs := fmt.Sprintf("--test_video_data=%s:%d:%d:%d:%d:%d:%d:%d",
		dataPath, d.Size.W, d.Size.H, d.FrameNum, d.FragmentsNum,
		d.MinFPSWithRender, d.MinFPSNoRender, int(d.Profile))
	return streamDataArgs
}

// VDABufferMode represents a mode of video_decode_accelerator_unittest.
type VDABufferMode int

const (
	// AllocateBuffer is a mode where video decode accelerator allocates buffer by itself.
	AllocateBuffer VDABufferMode = iota
	// ImportBuffer is a mode where video decode accelerator uses provided buffers.
	// In this mode, we run tests using frame validator.
	ImportBuffer
)

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

// RunAccelVideoTest runs video_decode_accelerator_unittest with given data.
// It fails if video_decode_accelerator_unittest fails.
func RunAccelVideoTest(ctx context.Context, s *testing.State, data TestVideoData, mode VDABufferMode) {
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
		logging.ChromeVmoduleFlag(),
		"--ozone-platform=gbm",
		// While thumbnail test fails, write thumbnail image to
		// s.OutDir() so that it will be accessible to host and packed
		// along with test logs.
		fmt.Sprintf("--thumbnail_output_dir=%s", s.OutDir()),
		data.toVDAArg(s.DataPath(data.Name)),
	}

	if mode == ImportBuffer {
		args = append(args, "--test_import", "--frame_validator=check")
	}

	const exec = "video_decode_accelerator_unittest"
	if err := bintest.Run(shortCtx, exec, args, s.OutDir()); err != nil {
		s.Fatalf("Failed to run %v: %v", exec, err)
	}
}
