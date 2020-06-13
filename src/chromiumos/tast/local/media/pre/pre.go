// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package pre provides Chrome Preconditions shared among media tests.
package pre

import (
	"strings"
	"sync"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/graphics"
	"chromiumos/tast/testing"
)

// ChromeVideo returns a precondition with Chrome started and logging enabled.
func ChromeVideo() testing.Precondition { return chromeVideoPre }

var chromeVideoPre = chrome.NewPrecondition("video", chromeVModuleArgs)

var alternateVideoDecoderPre testing.Precondition
var alternateVideoDecoderOnce sync.Once

// ChromeAlternateVideoDecoder returns a precondition with flags selecting the
// alternate hardware accelerated video decoder implementation. Chrome has two
// said implementations: a "legacy" one (VDA-based) and a "new" (VD-based) one.
// Selecting one or the other depends on the hardware and is ultimately
// determined by the overlays/ flags. Tests should be centered on what the users
// see, hence most of the testing should use ChromeVideo(), with a few test
// cases using this alternate precondition.
func ChromeAlternateVideoDecoder() testing.Precondition {
	alternateVideoDecoderOnce.Do(func() {
		if graphics.IsNewVideoDecoderDisabled() {
			alternateVideoDecoderPre = chrome.NewPrecondition("alternateVideo", chromeVModuleArgs, chrome.ExtraArgs("--enable-features=ChromeosVideoDecoder"))
		} else {
			alternateVideoDecoderPre = chrome.NewPrecondition("alternateVideo", chromeVModuleArgs, chrome.ExtraArgs("--disable-features=ChromeosVideoDecoder"))
		}
	})
	return alternateVideoDecoderPre
}

// ChromeVideoWithGuestLogin returns a precondition equal to ChromeVideo but
// forcing login as a guest, which is known to be different from a "normal"
// user in, at least, the flag set used.
func ChromeVideoWithGuestLogin() testing.Precondition { return chromeVideoWithGuestLoginPre }

var chromeVideoWithGuestLoginPre = chrome.NewPrecondition("videoWithGuestLogin", chromeVModuleArgs, chrome.GuestLogin())

// ChromeVideoWithHDRScreen returns a precondition equal to ChromeVideo but
// also enabling the HDR screen if present.
// TODO(crbug.com/958166): Use simply ChromeVideo() when HDR is launched.
func ChromeVideoWithHDRScreen() testing.Precondition { return chromeVideoWithHDRScreenPre }

var chromeVideoWithHDRScreenPre = chrome.NewPrecondition("videoWithHDRScreen", chromeVModuleArgs,
	chrome.ExtraArgs("--enable-use-hdr-transfer-function"))

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

// ChromeVideoWithFakeWebcamAndSWEncoding returns a precondition equal to
// ChromeVideoWithFakeWebcam and with hardware encoding disabled.
func ChromeVideoWithFakeWebcamAndSWEncoding() testing.Precondition {
	return chromeVideoWithFakeWebcamAndSWEncoding
}

var chromeVideoWithFakeWebcamAndSWEncoding = chrome.NewPrecondition("videoWithFakeWebcamAndSWEncoding", chromeVModuleArgs, chromeFakeWebcamArgs, chrome.ExtraArgs("--disable-accelerated-video-encode"))

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

// ChromeVideoWithSWDecoding returns a precondition similar to ChromeVideo,
// specified above, and making sure Chrome does not use any potential hardware
// accelerated decoding.
func ChromeVideoWithSWDecoding() testing.Precondition { return chromeVideoWithSWDecoding }

var chromeVideoWithSWDecoding = chrome.NewPrecondition("videoWithSWDecoding", chromeVModuleArgs,
	chrome.ExtraArgs("--disable-accelerated-video-decode"))

// ChromeVideoWithSWDecodingAndLibGAV1 returns a precondition similar to
// ChromeVideoWithSWDecoding specified above, while enabling the use of LibGAV1
// for AV1 decoding.
func ChromeVideoWithSWDecodingAndLibGAV1() testing.Precondition {
	return chromeVideoWithSWDecodingAndLibGAV1
}

var chromeVideoWithSWDecodingAndLibGAV1 = chrome.NewPrecondition("videoWithSWDecodingAndLibGAV1", chromeVModuleArgs,
	chrome.ExtraArgs("--disable-accelerated-video-decode", "--enable-features=Gav1VideoDecoder"))

// ChromeVideoWithSWDecodingAndHDRScreen returns a precondition similar to
// ChromeVideoWithSWDecoding, specified above, and also enabling the HDR screen
// if present.
// TODO(crbug.com/958166): Use simply ChromeVideoWithSWDecoding() when HDR is
// launched.
func ChromeVideoWithSWDecodingAndHDRScreen() testing.Precondition {
	return chromeVideoWithSWDecodingAndHDRScreen
}

var chromeVideoWithSWDecodingAndHDRScreen = chrome.NewPrecondition("videoWithSWDecodingAndHDRScreen", chromeVModuleArgs,
	chrome.ExtraArgs("--disable-accelerated-video-decode"), chrome.ExtraArgs("--enable-use-hdr-transfer-function"))

var chromeVModuleArgs = chrome.ExtraArgs(
	// Enable verbose log messages for video components.
	"--vmodule=" + strings.Join([]string{
		"*/media/gpu/chromeos/*=2",
		"*/media/gpu/vaapi/*=2",
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

var chromeCameraPerfPre = chrome.NewPrecondition("cameraPerf", chromeBypassPermissionsArgs)

// ChromeCameraPerfWithVP9VaapiEncoder returns a precondition equal to
// ChromeCameraPerf and with VA-API VP9 hardware encoder enabled.
// TODO(crbug.com/811912): remove when this is enabled by default.
func ChromeCameraPerfWithVP9VaapiEncoder() testing.Precondition {
	return chromeCameraPerfWithVP9VaapiEncoder
}

var chromeCameraPerfWithVP9VaapiEncoder = chrome.NewPrecondition("cameraPerfWithVP9VaapiEncoder", chromeVModuleArgs, chromeBypassPermissionsArgs, chrome.ExtraArgs("--enable-features=VaapiVP9Encoder"))

// ChromeFakeCameraPerf returns a precondition for Chrome to be started using
// the fake video/audio capture device (a.k.a. "fake webcam", see
// https://webrtc.org/testing), without asking for user permission, and without
// verboselogging that can affect the performance. This precondition should be
// used only used for performance tests.
func ChromeFakeCameraPerf() testing.Precondition { return chromeFakeCameraPerfPre }

var chromeFakeCameraPerfPre = chrome.NewPrecondition("fakeCameraPerf", chromeFakeWebcamArgs)

var chromeBypassPermissionsArgs = chrome.ExtraArgs(
	// Avoid the need to grant camera/microphone permissions.
	"--use-fake-ui-for-media-stream")
