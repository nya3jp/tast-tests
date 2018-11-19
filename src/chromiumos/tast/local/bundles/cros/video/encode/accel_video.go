// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package encode provides common code to run Chrome binary tests for encoding.
package encode

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"chromiumos/tast/local/bundles/cros/video/lib/chrometest"
	"chromiumos/tast/local/bundles/cros/video/lib/logging"
	"chromiumos/tast/local/bundles/cros/video/lib/videotype"
	"chromiumos/tast/testing"
)

// StreamParams is the parameter for video_encode_accelerator_unittest.
type StreamParams struct {
	// Name is the name of input raw data file.
	Name string
	// Size is the width and height of yuv image in the input raw data.
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

// RunAccelVideoTest runs video_encode_accelerator_unittest with profile and params.
// It fails if video_encode_accelerator_unittest fails.
func RunAccelVideoTest(ctx context.Context, s *testing.State, profile videotype.CodecProfile, params StreamParams, pixelFormat videotype.PixelFormat) {
	vl, err := logging.NewVideoLogger()
	if err != nil {
		s.Fatal("Failed to set values for verbose logging: ", err)
	}
	defer vl.Close()

	if !strings.HasSuffix(params.Name, ".vp9.webm") {
		s.Fatalf("Source video must %v must be VP9 WebM", params.Name)
	}

	streamPath, err := prepareYUV(ctx, s.DataPath(params.Name), pixelFormat, params.Size)
	if err != nil {
		s.Fatal("Failed to prepare yuv file: ", err)
	}
	defer os.Remove(streamPath)

	encodeOutFile := strings.TrimSuffix(params.Name, ".vp9.webm")
	if profile == videotype.H264Prof {
		encodeOutFile += ".h264"
	} else {
		encodeOutFile += ".vp8.ivf"
	}
	tmpEncodeOutFile, err := chrometest.CreateWritableTempFile(encodeOutFile)
	if err != nil {
		s.Fatalf("Failed to create test output file %s: %v", encodeOutFile, err)
	}
	defer func() {
		dstEncodeOutFile := filepath.Join(s.OutDir(), encodeOutFile)
		if err := chrometest.MoveFile(tmpEncodeOutFile, dstEncodeOutFile); err != nil {
			s.Errorf("Failed to move output file %s to %s: %v", tmpEncodeOutFile, dstEncodeOutFile, err)
		}
	}()

	testParamList := []string{
		logging.ChromeVmoduleFlag(),
		createStreamDataArg(params, profile, pixelFormat, streamPath, tmpEncodeOutFile),
		"--ozone-platform=gbm"}
	const veabinTest = "video_encode_accelerator_unittest"
	if err := chrometest.Run(ctx, s.OutDir(), veabinTest, testParamList); err != nil {
		s.Fatal(err)
	}
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
