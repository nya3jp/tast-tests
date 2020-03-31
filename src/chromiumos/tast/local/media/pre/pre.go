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
var chromeVideoPre = chrome.NewPrecondition("video", chromeVModuleArgs,
	chrome.ExtraArgs("--disable-features=ChromeosVideoDecoder"))

// ChromeVideoWithGuestLogin returns a precondition equal to ChromeVideo but
// forcing login as a guest, which is known to be different from a "normal"
// user in, at least, the flag set used.
func ChromeVideoWithGuestLogin() testing.Precondition { return chromeVideoWithGuestLoginPre }

var chromeVideoWithGuestLoginPre = chrome.NewPrecondition("videoWithGuestLogin", chromeVModuleArgs,
	chrome.ExtraArgs("--disable-features=ChromeosVideoDecoder"), chrome.GuestLogin())

// ChromeVideoWithHDRScreen returns a precondition equal to ChromeVideo but
// also enabling the HDR screen if present.
func ChromeVideoWithHDRScreen() testing.Precondition { return chromeVideoWithHDRScreenPre }

var chromeVideoWithHDRScreenPre = chrome.NewPrecondition("videoWithHDRScreen", chromeVModuleArgs,
	chrome.ExtraArgs("--enable-features=EnableUseHDRTransferFunction"))

// ChromeVideoWithFakeWebcam returns precondition equal to ChromeVideo above,
// supplementing it with the use of a fake video/audio capture device (a.k.a.
// "fake webcam"), see https://webrtc.org/testing/.
func ChromeVideoWithFakeWebcam() testing.Precondition { return chromeVideoWithFakeWebcamPre }

var chromeVideoWithFakeWebcamPre = chrome.NewPrecondition("videoWithFakeWebcam", chromeVModuleArgs, chromeFakeWebcamArgs)

// ChromeVideoWithFakeWebcamAndH264AMDEncoder returns a precondition equal to
// ChromeVideoWithFakeWebcam and with AMD H264 hardware encoder enabled.
// TODO(b/145961243): remove when this is enabled by default.
func ChromeVideoWithFakeWebcamAndH264AMDEncoder() testing.Precondition {
	return chromeVideoWithFakeWebcamAndH264AMDEncoder
}

var chromeVideoWithFakeWebcamAndH264AMDEncoder = chrome.NewPrecondition("videoWithFakeWebcamAndH264AMDEncoder", chromeVModuleArgs, chromeFakeWebcamArgs, chrome.ExtraArgs("--enable-features=VaapiH264AMDEncoder"))

// ChromeVideoWithFakeWebcamAndVP9VaapiEncoder returns a precondition equal to
// ChromeVideoWithFakeWebcam and with VA-API VP9 hardware encoder enabled.
// TODO(crbug.com/811912): remove when this is enabled by default.
func ChromeVideoWithFakeWebcamAndVP9VaapiEncoder() testing.Precondition {
	return chromeVideoWithFakeWebcamAndVP9VaapiEncoder
}

var chromeVideoWithFakeWebcamAndVP9VaapiEncoder = chrome.NewPrecondition("videoWithFakeWebcamAndVP9VaapiEncoder", chromeVModuleArgs, chromeFakeWebcamArgs, chrome.ExtraArgs("--enable-features=VaapiVP9Encoder"))

// ChromeVideoWithFakeWebcamAndSWDecoding returns a precondition equal to
// ChromeVideoWithFakeWebcam and with hardware decoding disabled.
func ChromeVideoWithFakeWebcamAndSWDecoding() testing.Precondition {
	return chromeVideoWithFakeWebcamAndSWDecoding
}

var chromeVideoWithFakeWebcamAndSWDecoding = chrome.NewPrecondition("videoWithFakeWebcamAndSWDecoding", chromeVModuleArgs, chromeFakeWebcamArgs, chrome.ExtraArgs("--disable-accelerated-video-decode"))

// ChromeScreenCapture returns a precondition so that Chrome always picks
// the entire screen for getDisplayMedia(), bypassing the picker UI.
func ChromeScreenCapture() testing.Precondition { return chromeScreenCapturePre }

var chromeScreenCapturePre = chrome.NewPrecondition("screenCapturePre",
	chrome.ExtraArgs(`--auto-select-desktop-capture-source=display`))

// ChromeWindowCapture returns a precondition so that Chrome always picks
// the Chromium window for getDisplayMedia(), bypassing the picker UI.
func ChromeWindowCapture() testing.Precondition { return chromeWindowCapturePre }

var chromeWindowCapturePre = chrome.NewPrecondition("windowCapturePre",
	chrome.ExtraArgs(`--auto-select-desktop-capture-source=Chrome`))

// ChromeVideoVD returns a precondition similar to ChromeVideo specified above.
// In addition this precondition specifies that the new
// media::VideoDecoder-based video decoders need to used (see go/vd-migration).
// This precondition must not be used for performance tests, as verbose logging
// might affect the performance.
func ChromeVideoVD() testing.Precondition { return chromeVideoVDPre }

var chromeVideoVDPre = chrome.NewPrecondition("videoVD", chromeVModuleArgs,
	chrome.ExtraArgs("--enable-features=ChromeosVideoDecoder"))

// ChromeVideoWithSWDecoding returns a precondition similar to ChromeVideo,
// specified above, and making sure Chrome does not use any potential hardware
// accelerated decoding.
func ChromeVideoWithSWDecoding() testing.Precondition { return chromeVideoWithSWDecoding }

var chromeVideoWithSWDecoding = chrome.NewPrecondition("videoWithSWDecoding", chromeVModuleArgs,
	chrome.ExtraArgs("--disable-accelerated-video-decode"))

var chromeVModuleArgs = chrome.ExtraArgs(
	// Enable verbose log messages for video components.
	"--vmodule=" + strings.Join([]string{
		"*/media/gpu/chromeos/*=2",
		"*/media/gpu/vaapi/*==2",
		"*/media/gpu/v4l2/*=2"}, ","))

var chromeFakeWebcamArgs = chrome.ExtraArgs(
	// Use a fake media capture device instead of live webcam(s)/microphone(s).
	"--use-fake-device-for-media-stream",
	// Avoid the need to grant camera/microphone permissions.
	"--use-fake-ui-for-media-stream")

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
