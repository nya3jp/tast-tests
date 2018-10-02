// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package vea provides common code to run Chrome video_encode_accelerator_unittest.
package vea

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"chromiumos/tast/local/bundles/cros/video/lib/logging"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

// Is this move to constants?
type PixelFormat int

const (
	I420 PixelFormat = 1
	NV12 PixelFormat = 6
)

type VideoCodecProfile int

const (
	H264  VideoCodecProfile = 1  // = H264PROFILE_MAIN
	VP8   VideoCodecProfile = 11 // =VP8PROFILE_ANY
	VP9   VideoCodecProfile = 12 //   =VP9PROFILE_PROFILE0
	VP9_2 VideoCodecProfile = 14 // VP9PROFILE_PROFILE2
)

type StreamParam struct {
	Name            string
	Width           int
	Height          int
	Bitrate         int
	Format          PixelFormat
	Framerate       int
	SubseqBitrate   int
	SubseqFramerate int
}

const (
	defaultFramerate          = 30
	defaultSubseqBitrateRatio = 2
)

func createStreamDataArg(param StreamParam, profile VideoCodecProfile, dataPath, outFile string) string {
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
		param.Bitrate, int(param.Format),
		framerate, subseqBitrate, subseqFramerate)
	return streamDataParam
}

func RunTest(ctx context.Context, s *testing.State, profile VideoCodecProfile, param StreamParam) {
	vl, err := logging.NewVideoLogger()
	if err != nil {
		s.Fatal("Failed to set values for verbose logging")
	}
	defer vl.Close()

	// TODO(hiroh): skip test properly
	// TODO(hiroh) nuke_chrome?

	outFile := filepath.Join(s.OutDir(), fmt.Sprintf("%s.out", param.Name))
	streamPath := s.DataPath(param.Name)
	if _, err := os.Stat(streamPath); err != nil {
		s.Fatalf("%s doesn't exist: %v", streamPath, err)
	}

	testParamList := []string{logging.ChromeVmoduleFlag(), createStreamDataArg(param, profile, streamPath, outFile), "--ozone-platform=gbm"}
	testParams := strings.Join(testParamList, " ")

	if err := chrome.RunChromeTestBinary(ctx, s.OutDir(), "video_encode_accelerator_unittest", testParams); err != nil {
		s.Fatal("Failed to run video_encode_accelerator_unittest: ", err)
	}
}
