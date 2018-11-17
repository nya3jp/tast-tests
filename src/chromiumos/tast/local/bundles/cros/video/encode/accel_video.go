// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package encode provides common code to run Chrome binary tests for encoding.
package encode

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/bundles/cros/video/lib/logging"
	"chromiumos/tast/local/bundles/cros/video/lib/videotype"
	"chromiumos/tast/local/chrome/bintest"
	"chromiumos/tast/testing"
)

// StreamParams is the parameter for video_encode_accelerator_unittest.
type StreamParams struct {
	// Name is the name of input raw data file.
	Name string
	// Width is the width of the input raw data.
	Width int
	// Height is the height of input raw data file.
	Height int
	// Bitrate is the requested bitrate in bits per second. VideoEncodeAccelerator is forced to output
	// encoded video in expected range around the bitrate.
	Bitrate int
	// Format is the pixel format of raw data.
	Format videotype.PixelFormat
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

// RunAccelVideoTest runs video_encode_accelerator_unittest with profile and params.
// It fails if video_encode_accelerator_unittest fails.
func RunAccelVideoTest(ctx context.Context, s *testing.State, profile videotype.CodecProfile, params StreamParams) {
	vl, err := logging.NewVideoLogger()
	if err != nil {
		s.Fatal("Failed to set values for verbose logging: ", err)
	}
	defer vl.Close()

	// Create an temporary output file that the test can write to.
	tempFile, err := ioutil.TempFile("", params.Name+".out.tast.")
	if err != nil {
		s.Fatal("Failed to create temp output file: ", err)
	}
	tempFile.Close()

	defer func() {
		dst := filepath.Join(s.OutDir(), params.Name+".out")
		if err := fsutil.MoveFile(tempFile.Name(), dst); err != nil {
			s.Errorf("Failed to move output file %s to %s: %v", tempFile.Name(), dst, err)
		}
	}()

	if err := os.Chmod(tempFile.Name(), 0666); err != nil {
		s.Fatalf("Failed to chmod %v: %v", tempFile.Name(), err)
	}

	args := []string{logging.ChromeVmoduleFlag(),
		createStreamDataArg(params, profile, s.DataPath(params.Name), tempFile.Name()),
		"--ozone-platform=gbm"}
	const exec = "video_encode_accelerator_unittest"
	if err := bintest.Run(ctx, exec, args, s.OutDir()); err != nil {
		s.Fatalf("Failed to run %v: %v", exec, err)
	}
}

// createStreamDataArg creates an argument of video_encode_accelerator_unittest from profile, dataPath and outFile.
func createStreamDataArg(params StreamParams, profile videotype.CodecProfile, dataPath, outFile string) string {
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
		dataPath, params.Width, params.Height, int(profile), outFile,
		params.Bitrate, params.FrameRate, params.SubseqBitrate,
		params.SubseqFrameRate, int(params.Format))
	return streamDataArgs
}
