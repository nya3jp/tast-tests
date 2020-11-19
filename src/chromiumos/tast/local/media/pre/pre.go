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

// ChromeVideo returns a precondition with Chrome started and logging enabled.
func ChromeVideo() testing.Precondition { return chromeVideoPre }

var chromeVideoPre = chrome.NewPrecondition("video",
	chromeVModuleArgs,
	chromeUseHWCodecsForSmallResolutions,
	chromeBypassPermissionsArgs,
	chromeSuppressNotificationsArgs)

// ChromeAlternateVideoDecoder returns a precondition with flags selecting the
// alternate hardware accelerated video decoder implementation. Chrome has two
// said implementations: a "legacy" one and a Direct, VD-based one. Selecting
// one or the other depends on the hardware and is ultimately determined by the
// overlays/ flags. Tests should be centered on what the users see, hence most
// of the testing should use ChromeVideo(), with a few test cases using this
// alternate precondition.
func ChromeAlternateVideoDecoder() testing.Precondition { return chromeAlternateVideoDecoderPre }

var chromeAlternateVideoDecoderPre = chrome.NewPrecondition("alternateVideoDecoder",
	chromeVModuleArgs,
	chromeUseHWCodecsForSmallResolutions,
	chromeSuppressNotificationsArgs,
	chrome.EnableFeatures("UseAlternateVideoDecoderImplementation"))

// ChromeVideoWithGuestLogin returns a precondition equal to ChromeVideo but
// forcing login as a guest, which is known to be different from a "normal"
// user in, at least, the flag set used.
func ChromeVideoWithGuestLogin() testing.Precondition { return chromeVideoWithGuestLoginPre }

var chromeVideoWithGuestLoginPre = chrome.NewPrecondition("videoWithGuestLogin",
	chromeVModuleArgs,
	chromeUseHWCodecsForSmallResolutions,
	chromeSuppressNotificationsArgs,
	chrome.GuestLogin())

// ChromeVideoWithHDRScreen returns a precondition equal to ChromeVideo but
// also enabling the HDR screen if present.
// TODO(crbug.com/958166): Use simply ChromeVideo() when HDR is launched.
func ChromeVideoWithHDRScreen() testing.Precondition { return chromeVideoWithHDRScreenPre }

var chromeVideoWithHDRScreenPre = chrome.NewPrecondition("videoWithHDRScreen",
	chromeVModuleArgs,
	chromeUseHWCodecsForSmallResolutions,
	chromeSuppressNotificationsArgs,
	chrome.EnableFeatures("UseHDRTransferFunction"))

// ChromeCompositedVideo returns a precondition equal to ChromeVideo but also
// disabling hardware overlays entirely to force video to be composited.
func ChromeCompositedVideo() testing.Precondition { return chromeCompositedVideoPre }

var chromeCompositedVideoPre = chrome.NewPrecondition("compositedVideo",
	chromeVModuleArgs,
	chromeUseHWCodecsForSmallResolutions,
	chromeSuppressNotificationsArgs,
	chrome.ExtraArgs("--enable-hardware-overlays=\"\""))

// ChromeVideoWithFakeWebcam returns precondition equal to ChromeVideo above,
// supplementing it with the use of a fake video/audio capture device (a.k.a.
// "fake webcam"), see https://webrtc.org/testing/.
func ChromeVideoWithFakeWebcam() testing.Precondition { return chromeVideoWithFakeWebcamPre }

var chromeVideoWithFakeWebcamPre = chrome.NewPrecondition("videoWithFakeWebcam",
	chromeVModuleArgs,
	chromeUseHWCodecsForSmallResolutions,
	chromeSuppressNotificationsArgs,
	chromeFakeWebcamArgs)

// ChromeVideoWithFakeWebcamAndAlternateVideoDecoder returns a precondition
// equal to ChromeVideoWithFakeWebcam above, and using the alternative video
// decoder (see ChromeAlternateVideoDecoder comments).
func ChromeVideoWithFakeWebcamAndAlternateVideoDecoder() testing.Precondition {
	return chromeVideoWithFakeWebcamAndAlternateVideoDecoderPre
}

var chromeVideoWithFakeWebcamAndAlternateVideoDecoderPre = chrome.NewPrecondition("videoWithFakeWebcamAndAlternateVideoDecoder",
	chromeVModuleArgs,
	chromeUseHWCodecsForSmallResolutions,
	chromeSuppressNotificationsArgs,
	chromeFakeWebcamArgs,
	chrome.EnableFeatures("UseAlternateVideoDecoderImplementation"))

// ChromeVideoWithFakeWebcamAndForceVP9ThreeTemporalLayers returns a precondition equal to
// ChromeVideoWithFakeWebcam, force webrtc vp9 stream to be three temporal layers.
func ChromeVideoWithFakeWebcamAndForceVP9ThreeTemporalLayers() testing.Precondition {
	return chromeVideoWithFakeWebcamAndForceVP9ThreeTemporalLayers
}

var chromeVideoWithFakeWebcamAndForceVP9ThreeTemporalLayers = chrome.NewPrecondition(
	"VideoWithFakeWebcamAndForceVP9ThreeTemporalLayers",
	chromeVModuleArgs, chromeFakeWebcamArgs, chromeSuppressNotificationsArgs,
	chrome.ExtraArgs("--force-fieldtrials=WebRTC-SupportVP9SVC/EnabledByFlag_1SL3TL/"))

// ChromeVideoWithFakeWebcamAndSWDecoding returns a precondition equal to
// ChromeVideoWithFakeWebcam and with hardware decoding disabled.
func ChromeVideoWithFakeWebcamAndSWDecoding() testing.Precondition {
	return chromeVideoWithFakeWebcamAndSWDecoding
}

var chromeVideoWithFakeWebcamAndSWDecoding = chrome.NewPrecondition("videoWithFakeWebcamAndSWDecoding", chromeVModuleArgs, chromeSuppressNotificationsArgs, chromeFakeWebcamArgs, chrome.ExtraArgs("--disable-accelerated-video-decode"))

// ChromeVideoWithFakeWebcamAndSWEncoding returns a precondition equal to
// ChromeVideoWithFakeWebcam and with hardware encoding disabled.
func ChromeVideoWithFakeWebcamAndSWEncoding() testing.Precondition {
	return chromeVideoWithFakeWebcamAndSWEncoding
}

var chromeVideoWithFakeWebcamAndSWEncoding = chrome.NewPrecondition("videoWithFakeWebcamAndSWEncoding", chromeVModuleArgs, chromeSuppressNotificationsArgs, chromeFakeWebcamArgs, chrome.ExtraArgs("--disable-accelerated-video-encode"))

// ChromeScreenCapture returns a precondition so that Chrome always picks
// the entire screen for getDisplayMedia(), bypassing the picker UI.
func ChromeScreenCapture() testing.Precondition { return chromeScreenCapturePre }

var chromeScreenCapturePre = chrome.NewPrecondition("screenCapturePre",
	chromeSuppressNotificationsArgs,
	chrome.ExtraArgs(`--auto-select-desktop-capture-source=display`))

// ChromeWindowCapture returns a precondition so that Chrome always picks
// the Chromium window for getDisplayMedia(), bypassing the picker UI.
func ChromeWindowCapture() testing.Precondition { return chromeWindowCapturePre }

var chromeWindowCapturePre = chrome.NewPrecondition("windowCapturePre",
	chromeSuppressNotificationsArgs,
	chrome.ExtraArgs(`--auto-select-desktop-capture-source=Chrome`))

// ChromeVideoWithSWDecoding returns a precondition similar to ChromeVideo,
// specified above, and making sure Chrome does not use any potential hardware
// accelerated decoding.
func ChromeVideoWithSWDecoding() testing.Precondition { return chromeVideoWithSWDecoding }

var chromeVideoWithSWDecoding = chrome.NewPrecondition("videoWithSWDecoding", chromeVModuleArgs,
	chromeSuppressNotificationsArgs,
	chrome.ExtraArgs("--disable-accelerated-video-decode"))

// ChromeVideoWithSWDecodingAndLibGAV1 returns a precondition similar to
// ChromeVideoWithSWDecoding specified above, while enabling the use of LibGAV1
// for AV1 decoding.
func ChromeVideoWithSWDecodingAndLibGAV1() testing.Precondition {
	return chromeVideoWithSWDecodingAndLibGAV1
}

var chromeVideoWithSWDecodingAndLibGAV1 = chrome.NewPrecondition("videoWithSWDecodingAndLibGAV1", chromeVModuleArgs, chromeSuppressNotificationsArgs,
	chrome.ExtraArgs("--disable-accelerated-video-decode", "--enable-features=Gav1VideoDecoder"))

// ChromeVideoWithHWAV1Decoding returns a precondition similar to ChromeVideo,
// specified above, but also enabls hardware accelerated av1 decoding.
// TODO(b/172217032): Remove these *HWAV1Decoding preconditions once the hardware av1 decoder feature is enabled by default.
func ChromeVideoWithHWAV1Decoding() testing.Precondition {
	return chromeVideoWithHWAV1Decoding
}

// ChromeVideoWithGuestLoginAndHWAV1Decoding returns a precondition similar to
// ChromeVideoWithGuestLogin, specified above, but also enables hardware accelerated av1 decoding.
func ChromeVideoWithGuestLoginAndHWAV1Decoding() testing.Precondition {
	return chromeVideoWithGuestLoginAndHWAV1Decoding
}

var chromeVideoWithHWAV1Decoding = chrome.NewPrecondition("chromeVideoWithHWAV1Decoding",
	chromeVModuleArgs,
	chromeUseHWCodecsForSmallResolutions,
	chromeSuppressNotificationsArgs,
	chrome.ExtraArgs("--enable-features=VaapiAV1Decoder"))

var chromeVideoWithGuestLoginAndHWAV1Decoding = chrome.NewPrecondition("chromeVideoWithGuestLoginAndHWAV1Decoding",
	chromeVModuleArgs,
	chromeUseHWCodecsForSmallResolutions,
	chromeSuppressNotificationsArgs,
	chrome.GuestLogin(),
	chrome.ExtraArgs("--enable-features=VaapiAV1Decoder"))

// ChromeVideoWithSWDecodingAndHDRScreen returns a precondition similar to
// ChromeVideoWithSWDecoding, specified above, and also enabling the HDR screen
// if present.
// TODO(crbug.com/958166): Use simply ChromeVideoWithSWDecoding() when HDR is
// launched.
func ChromeVideoWithSWDecodingAndHDRScreen() testing.Precondition {
	return chromeVideoWithSWDecodingAndHDRScreen
}

var chromeVideoWithSWDecodingAndHDRScreen = chrome.NewPrecondition("videoWithSWDecodingAndHDRScreen", chromeVModuleArgs, chromeSuppressNotificationsArgs,
	chrome.ExtraArgs("--disable-accelerated-video-decode"), chrome.EnableFeatures("UseHDRTransferFunction"))

var chromeVModuleArgs = chrome.ExtraArgs(
	// Enable verbose log messages for video components.
	"--vmodule=" + strings.Join([]string{
		"*/media/gpu/chromeos/*=2",
		"*/media/gpu/vaapi/*=2",
		"*/media/gpu/v4l2/*=2"}, ","))

var chromeUseHWCodecsForSmallResolutions = chrome.ExtraArgs(
	// The Renderer video stack might have a policy of not using hardware
	// accelerated decoding for certain small resolutions (see crbug.com/684792).
	// Disable that for testing.
	"--disable-features=ResolutionBasedDecoderPriority",
	// VA-API HW decoder and encoder might deny a smaller resolution for the
	// performance (see crbug.com/1008491 and b/171041334).
	// Disable that for testing.
	"--disable-features=VaapiEnforceVideoMinMaxResolution")

var chromeFakeWebcamArgs = chrome.ExtraArgs(
	// Use a fake media capture device instead of live webcam(s)/microphone(s).
	"--use-fake-device-for-media-stream",
	// Avoid the need to grant camera/microphone permissions.
	"--use-fake-ui-for-media-stream")

// ChromeCameraPerf returns a precondition that Chrome is started with camera
// tests-specific setting and without verbose logging that can affect the
// performance. This precondition should be used only for performance tests.
func ChromeCameraPerf() testing.Precondition { return chromeCameraPerfPre }

var chromeCameraPerfPre = chrome.NewPrecondition("cameraPerf", chromeBypassPermissionsArgs, chromeSuppressNotificationsArgs)

// ChromeFakeCameraPerf returns a precondition for Chrome to be started using
// the fake video/audio capture device (a.k.a. "fake webcam", see
// https://webrtc.org/testing), without asking for user permission, and without
// verboselogging that can affect the performance. This precondition should be
// used only used for performance tests.
func ChromeFakeCameraPerf() testing.Precondition { return chromeFakeCameraPerfPre }

var chromeFakeCameraPerfPre = chrome.NewPrecondition("fakeCameraPerf", chromeFakeWebcamArgs, chromeSuppressNotificationsArgs)

var chromeBypassPermissionsArgs = chrome.ExtraArgs(
	// Avoid the need to grant camera/microphone permissions.
	"--use-fake-ui-for-media-stream")

var chromeSuppressNotificationsArgs = chrome.ExtraArgs(
	// Do not show message center notifications.
	"--suppress-message-center-popups")
