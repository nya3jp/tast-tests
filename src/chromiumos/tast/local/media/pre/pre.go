// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package pre provides Chrome Preconditions shared among media tests.
package pre

import (
	"strings"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

// ChromeVideo returns a precondition that makes sure Chrome is started with video tests-specific
// flags and is already logged in when a test is run. This precondition must not be used for
// performance tests, as verbose logging might affect the performance.
func ChromeVideo() testing.Precondition { return chromeVideoPre }

var chromeVideoPre = chrome.NewPrecondition("video", chromeArgs)

// ChromeVideoVD returns a precondition similar to ChromeVideo specified above. In addition this
// precondition specifies that the new media::VideoDecoder-based video decoders need to used
// (see go/vd-migration). This precondition must not be used for performance tests, as verbose
// logging might affect the performance.
func ChromeVideoVD() testing.Precondition { return chromeVideoVDPre }

var chromeVideoVDPre = chrome.NewPrecondition("videoVD", chromeArgs, chromeVDArgs)

var chromeArgs = chrome.ExtraArgs(
	// Enable verbose log messages for video components.
	"--vmodule="+strings.Join([]string{
		"*/media/gpu/*video_decode_accelerator.cc=2",
		"*/media/gpu/*video_encode_accelerator.cc=2",
		"*/media/gpu/*jpeg_decode_accelerator.cc=2",
		"*/media/gpu/*jpeg_encode_accelerator.cc=2",
		"*/media/gpu/*image_processor.cc=2",
		"*/media/gpu/*v4l2_device.cc=2"}, ","),
	// Disable the autoplay policy not to be affected by actions from outside of tests.
	// cf. https://developers.google.com/web/updates/2017/09/autoplay-policy-changes
	"--autoplay-policy=no-user-gesture-required",
	// Avoid the need to grant camera/microphone permissions.
	"--use-fake-ui-for-media-stream")

var chromeVDArgs = chrome.ExtraArgs(
	// Enable verbose log messages for media::VideoDecoder-related components.
	"--vmodule="+strings.Join([]string{
		"*/media/gpu/*video_decoder.cc=2",
		"*/media/gpu/*mailbox_video_frame_converter.cc=2",
		"*/media/gpu/*platform_video_frame_pool.cc=2",
		"*/media/gpu/*video_decoder_pipeline.cc=2"}, ","),
	// Enable media::VideoDecoder-based video decoders.
	"--enable-features=ChromeosVideoDecoder")

// ChromeCameraPerf returns a precondition that Chrome is started with camera tests-specific
// setting and without verbose logging that can affect the performance.
// This precondition should be used only used for performance tests.
func ChromeCameraPerf() testing.Precondition { return chromeCameraPerfPre }

var chromeCameraPerfPre = chrome.NewPrecondition("camera_perf",
	chrome.ExtraArgs(
		// Avoid the need to grant camera/microphone permissions.
		"--use-fake-ui-for-media-stream"))
