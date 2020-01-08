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

// ChromeVideo returns a precondition that makes sure Chrome is started with
// video tests-specific flags and is already logged in when a test is run. This
// precondition must not be used for performance tests, as verbose logging might
// affect the performance.
func ChromeVideo() testing.Precondition { return chromeVideoPre }

// TODO(b/141652665): Currently the ChromeosVideoDecoder feature is enabled on
// x% of devices depending on the branch, so we need to use both enable and
// disable flags to garantuee correct behavior. Once the feature is always
// enabled we can remove the "--enable-features" flag on chromeVDArgs.
var chromeVideoPre = chrome.NewPrecondition("video", chromeArgs,
	chrome.ExtraArgs("--disable-features=ChromeosVideoDecoder"))

// ChromeVideoWithFakeWebcam returns precondition equal to ChromeVideo above,
// supplementing it with the use of a fake video/audio capture device (a.k.a.
// "fake webcam"), see https://webrtc.org/testing/.
func ChromeVideoWithFakeWebcam() testing.Precondition { return chromeVideoWithFakeWebcamPre }

var chromeVideoWithFakeWebcamPre = chrome.NewPrecondition("videoWithFakeWebcam", chromeArgs, chromeFakeWebcamArgs)

// ChromeVideoWithFakeWebcamAndH264AMDEncoder returns a precondition equal to
// ChromeVideoWithFakeWebcam and with AMD H264 hardware encoder enabled.
// TODO(b/145961243): remove when this is enabled by default.
func ChromeVideoWithFakeWebcamAndH264AMDEncoder() testing.Precondition {
	return chromeVideoWithFakeWebcamAndH264AMDEncoder
}

var chromeVideoWithFakeWebcamAndH264AMDEncoder = chrome.NewPrecondition("videoWithFakeWebcamAndH264AMDEncoder", chromeArgs, chromeFakeWebcamArgs, chromeEnableH264AMDEncoder)

// ChromeVideoVD returns a precondition similar to ChromeVideo specified above.
// In addition this precondition specifies that the new
// media::VideoDecoder-based video decoders need to used (see go/vd-migration).
// This precondition must not be used for performance tests, as verbose logging
// might affect the performance.
func ChromeVideoVD() testing.Precondition { return chromeVideoVDPre }

var chromeVideoVDPre = chrome.NewPrecondition("videoVD", chromeArgs, chromeVDArgs)

// ChromeVideoWithSWDecoding returns a precondition similar to ChromeVideo,
// specified above, and making sure Chrome does not use any potential hardware
// accelerated decoding.
func ChromeVideoWithSWDecoding() testing.Precondition { return chromeVideoWithSWDecoding }

var chromeVideoWithSWDecoding = chrome.NewPrecondition("videoWithSWDecoding", chromeArgs,
	chrome.ExtraArgs("--disable-accelerated-video-decode"))

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

var chromeFakeWebcamArgs = chrome.ExtraArgs(
	// Use a fake media capture device instead of live webcam(s)/microphone(s).
	"--use-fake-device-for-media-stream")

var chromeEnableH264AMDEncoder = chrome.ExtraArgs("--enable-features=VaapiH264AMDEncoder")

var chromeVDArgs = chrome.ExtraArgs(
	// Enable verbose log messages for media::VideoDecoder-related components.
	"--vmodule="+strings.Join([]string{
		"*/media/gpu/*video_decoder.cc=2",
		"*/media/gpu/*mailbox_video_frame_converter.cc=2",
		"*/media/gpu/*platform_video_frame_pool.cc=2",
		"*/media/gpu/*video_decoder_pipeline.cc=2"}, ","),
	// Enable media::VideoDecoder-based video decoders.
	"--enable-features=ChromeosVideoDecoder")

// ChromeCameraPerf returns a precondition that Chrome is started with camera
// tests-specific setting and without verbose logging that can affect the
// performance. This precondition should be used only for performance tests.
func ChromeCameraPerf() testing.Precondition { return chromeCameraPerfPre }

var chromeCameraPerfPre = chrome.NewPrecondition("cameraPerf",
	chrome.ExtraArgs(
		// Avoid the need to grant camera/microphone permissions.
		"--use-fake-ui-for-media-stream"))

// ChromeFakeCameraPerf returns a precondition for Chrome to be started using
// the fake video/audio capture device (a.k.a. "fake webcam", see
// https://webrtc.org/testing), without asking for user permission, and without
// verboselogging that can affect the performance. This precondition should be
// used only used for performance tests.
func ChromeFakeCameraPerf() testing.Precondition { return chromeFakeCameraPerfPre }

var chromeFakeCameraPerfPre = chrome.NewPrecondition("fakeCameraPerf",
	chrome.ExtraArgs(
		// Use a fake video/audio capture device instead of webcam(s)/microphone(s).
		"--use-fake-device-for-media-stream",
		// Avoid the need to grant camera/microphone permissions.
		"--use-fake-ui-for-media-stream"))
