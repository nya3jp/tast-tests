// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package videoenc provides common code to run Chrome video_encode_accelerator_unittest.
package videoenc

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"chromiumos/tast/local/bundles/cros/video/lib/chromebin"
	"chromiumos/tast/local/bundles/cros/video/lib/logging"
	"chromiumos/tast/local/bundles/cros/video/lib/videotype"
	"chromiumos/tast/testing"
)

// StreamParam is the parameter for video_encode_accelerator_unittest.
type StreamParam struct {
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
	// Framerate is the initial framerate in the test. This value is optional, and will be set to
	// thirty if unspecified.
	Framerate int
	// SubseqBitrate is the bitrate to siwtch to in the middle of the stream in some test cases in
	// video_encode_accelerator_unittest. This value is optional, and will be set to twice of Bitrate if unspecified.
	SubseqBitrate int
	// SubseqFramerate is the framerate to siwtch to in the middle of the stream in some test cases in
	// video_encode_accelerator_unittest. This value is optional, and will be set to thirty if unspecified.
	SubseqFramerate int
}

// RunTest runs video_encode_accelerator_unittest with profile and param.
// It fails if video_encode_accelerator_unittest fails.
func RunTest(ctx context.Context, s *testing.State, profile videotype.CodecProfile, param StreamParam) {
	vl, err := logging.NewVideoLogger()
	if err != nil {
		s.Fatal("Failed to set values for verbose logging")
	}
	defer vl.Close()

	streamPath := s.DataPath(param.Name)
	if _, err := os.Stat(streamPath); err != nil {
		s.Fatalf("%s doesn't exist: %v", streamPath, err)
	}

	encodedOutFile := filepath.Join("/tmp", fmt.Sprintf("%s.out", param.Name))
	defer moveToOutDir(s.OutDir(), encodedOutFile)

	testParamList := []string{logging.ChromeVmoduleFlag(), createStreamDataArg(param, profile, streamPath, encodedOutFile), "--ozone-platform=gbm"}

	if err := chromebin.RunChromeTestBinary(ctx, "video_encode_accelerator_unittest", testParamList); err != nil {
		s.Fatal("Failed to run video_encode_accelerator_unittest: ", err)
	}
}

// moveToOutDir moves to file represented by fpath to outDir.
func moveToOutDir(outDir, fpath string) error {
	if _, err := os.Stat(fpath); err != nil {
		return err
	}
	os.Rename(fpath, filepath.Join(outDir, filepath.Base(fpath)))
	return nil
}

// createStreamDataArg creates an argument of video_encode_accelerator_unittest from profile, dataPath and outFile.
func createStreamDataArg(param StreamParam, profile videotype.CodecProfile, dataPath, outFile string) string {
	const (
		defaultFramerate          = 30
		defaultSubseqBitrateRatio = 2
	)

	framerate, subseqBitrate, subseqFramerate := param.Framerate, param.SubseqBitrate, param.SubseqFramerate
	// Fill default values if they are unsettled.
	if framerate == 0 {
		framerate = defaultFramerate
	}
	if subseqBitrate == 0 {
		subseqBitrate = param.Bitrate * defaultSubseqBitrateRatio
	}
	if subseqFramerate == 0 {
		subseqFramerate = defaultFramerate
	}
	streamDataParam := "--test_stream_data=" + dataPath
	streamDataParam += fmt.Sprintf(":%d:%d:%d:%s:%d:%d:%d:%d:%d",
		param.Width, param.Height, int(profile), outFile,
		param.Bitrate, framerate, subseqBitrate, subseqFramerate, int(param.Format))
	return streamDataParam
}
