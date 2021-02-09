// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package pre

import (
	"strings"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/lacros/launcher"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name: "chromeVideo",
		Desc: "Logged into a user session with logging enabled",
		Impl: chrome.NewLoggedInFixture(
			chrome.ExtraArgs(chromeVModuleArgs...),
			chrome.ExtraArgs(chromeUseHWCodecsForSmallResolutions...),
			chrome.ExtraArgs(chromeBypassPermissionsArgs...),
			chrome.ExtraArgs(chromeSuppressNotificationsArgs...),
		),
		Parent:          "gpuWatchDog",
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name: "chromeVideoLacros",
		Desc: "Logged into a user session with logging enabled (lacros)",
		Impl: launcher.NewStartedByData(launcher.PreExist,
			chrome.ExtraArgs(chromeVModuleArgs...),
			chrome.LacrosExtraArgs(chromeVModuleArgs...),
			chrome.ExtraArgs(chromeUseHWCodecsForSmallResolutions...),
			chrome.LacrosExtraArgs(chromeUseHWCodecsForSmallResolutions...),
			chrome.ExtraArgs(chromeBypassPermissionsArgs...),
			chrome.LacrosExtraArgs(chromeBypassPermissionsArgs...),
			chrome.ExtraArgs(chromeSuppressNotificationsArgs...),
			chrome.LacrosExtraArgs(chromeSuppressNotificationsArgs...),
		),
		Parent:          "gpuWatchDog",
		SetUpTimeout:    chrome.LoginTimeout + 7*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Vars:            []string{"lacrosDeployedBinary"},
	})

	// Chrome has two said implementations: a "legacy" one and a Direct, VD-based on. Selecting one ore the other depends on the hardware and is ultimately determined by the overlays/ flags. Tests should be centered on what the users see, hence most of the testing should use chromeVideo, with a few test cases using this fixture.
	testing.AddFixture(&testing.Fixture{
		Name: "chromeAlternateVideoDecoder",
		Desc: "Logged into a user session with alternate hardware accelerated video decoder.",
		Impl: chrome.NewLoggedInFixture(
			chrome.ExtraArgs(chromeVModuleArgs...),
			chrome.ExtraArgs(chromeUseHWCodecsForSmallResolutions...),
			chrome.ExtraArgs(chromeSuppressNotificationsArgs...),
			chrome.ExtraArgs("--enable-features=UseAlternateVideoDecoderImplementation"),
		),
		Parent:          "gpuWatchDog",
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name: "chromeAlternateVideoDecoderLacros",
		Desc: "Logged into a user session with alternate hardware accelerated video decoder (lacros).",
		Impl: launcher.NewStartedByData(launcher.PreExist,
			chrome.ExtraArgs(chromeVModuleArgs...),
			chrome.LacrosExtraArgs(chromeVModuleArgs...),
			chrome.ExtraArgs(chromeUseHWCodecsForSmallResolutions...),
			chrome.LacrosExtraArgs(chromeUseHWCodecsForSmallResolutions...),
			chrome.ExtraArgs(chromeSuppressNotificationsArgs...),
			chrome.LacrosExtraArgs(chromeSuppressNotificationsArgs...),
			chrome.ExtraArgs("--enable-features=UseAlternateVideoDecoderImplementation"),
			chrome.LacrosExtraArgs("--enable-features=UseAlternateVideoDecoderImplementation"),
		),
		Parent:          "gpuWatchDog",
		SetUpTimeout:    chrome.LoginTimeout + 7*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Vars:            []string{"lacrosDeployedBinary"},
	})

	testing.AddFixture(&testing.Fixture{
		Name: "chromeVideoWithGuestLogin",
		Desc: "Similar to chromeVideo fixture but forcing login as a guest.",
		Impl: chrome.NewLoggedInFixture(
			chrome.ExtraArgs(chromeVModuleArgs...),
			chrome.ExtraArgs(chromeUseHWCodecsForSmallResolutions...),
			chrome.ExtraArgs(chromeSuppressNotificationsArgs...),
			chrome.GuestLogin(),
		),
		Parent:          "gpuWatchDog",
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name: "chromeVideoWithGuestLoginLacros",
		Desc: "Similar to chromeVideo fixture but forcing login as a guest (lacros).",
		Impl: launcher.NewStartedByData(launcher.PreExist,
			chrome.ExtraArgs(chromeVModuleArgs...),
			chrome.LacrosExtraArgs(chromeVModuleArgs...),
			chrome.ExtraArgs(chromeUseHWCodecsForSmallResolutions...),
			chrome.LacrosExtraArgs(chromeUseHWCodecsForSmallResolutions...),
			chrome.ExtraArgs(chromeSuppressNotificationsArgs...),
			chrome.LacrosExtraArgs(chromeSuppressNotificationsArgs...),
			chrome.GuestLogin(),
		),
		Parent:          "gpuWatchDog",
		SetUpTimeout:    chrome.LoginTimeout + 7*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Vars:            []string{"lacrosDeployedBinary"},
	})

	// TODO(crbug.com/958166): Use simply ChromeVideo() when HDR is launched.
	testing.AddFixture(&testing.Fixture{
		Name: "chromeVideoWithHDRScreen",
		Desc: "Similar to chromeVideo fixture but enabling the HDR screen if present.",
		Impl: chrome.NewLoggedInFixture(
			chrome.ExtraArgs(chromeVModuleArgs...),
			chrome.ExtraArgs(chromeUseHWCodecsForSmallResolutions...),
			chrome.ExtraArgs(chromeSuppressNotificationsArgs...),
			chrome.ExtraArgs("--enable-features=UseHDRTransferFunction"),
		),
		Parent:          "gpuWatchDog",
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	// TODO(crbug.com/958166): Use simply ChromeVideo() when HDR is launched.
	testing.AddFixture(&testing.Fixture{
		Name: "chromeVideoWithHDRScreenLacros",
		Desc: "Similar to chromeVideo fixture but enabling the HDR screen if present (lacros).",
		Impl: launcher.NewStartedByData(launcher.PreExist,
			chrome.ExtraArgs(chromeVModuleArgs...),
			chrome.LacrosExtraArgs(chromeVModuleArgs...),
			chrome.ExtraArgs(chromeUseHWCodecsForSmallResolutions...),
			chrome.LacrosExtraArgs(chromeUseHWCodecsForSmallResolutions...),
			chrome.ExtraArgs(chromeSuppressNotificationsArgs...),
			chrome.LacrosExtraArgs(chromeSuppressNotificationsArgs...),
			chrome.ExtraArgs("--enable-features=UseHDRTransferFunction"),
			chrome.LacrosExtraArgs("--enable-features=UseHDRTransferFunction"),
		),
		Parent:          "gpuWatchDog",
		SetUpTimeout:    chrome.LoginTimeout + 7*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Vars:            []string{"lacrosDeployedBinary"},
	})

	testing.AddFixture(&testing.Fixture{
		Name: "chromeCompositedVideo",
		Desc: "Similar to chromeVideo fixture but disabling hardware overlays entirely to force video to be composited.",
		Impl: chrome.NewLoggedInFixture(
			chrome.ExtraArgs(chromeVModuleArgs...),
			chrome.ExtraArgs(chromeUseHWCodecsForSmallResolutions...),
			chrome.ExtraArgs(chromeSuppressNotificationsArgs...),
			chrome.ExtraArgs("--enable-hardware-overlays=\"\""),
		),
		Parent:          "gpuWatchDog",
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name: "chromeCompositedVideoLacros",
		Desc: "Similar to chromeVideo fixture but disabling hardware overlays entirely to force video to be composited (lacros).",
		Impl: launcher.NewStartedByData(launcher.PreExist,
			chrome.ExtraArgs(chromeVModuleArgs...),
			chrome.LacrosExtraArgs(chromeVModuleArgs...),
			chrome.ExtraArgs(chromeUseHWCodecsForSmallResolutions...),
			chrome.LacrosExtraArgs(chromeUseHWCodecsForSmallResolutions...),
			chrome.ExtraArgs(chromeSuppressNotificationsArgs...),
			chrome.LacrosExtraArgs(chromeSuppressNotificationsArgs...),
			chrome.ExtraArgs("--enable-hardware-overlays=\"\""),
		),
		Parent:          "gpuWatchDog",
		SetUpTimeout:    chrome.LoginTimeout + 7*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Vars:            []string{"lacrosDeployedBinary"},
	})

	testing.AddFixture(&testing.Fixture{
		Name: "chromeVideoWithFakeWebcam",
		Desc: "Similar to chromeVideo fixture but supplementing it with the use of a fake video/audio capture device (a.k.a. 'fake webcam'), see https://webrtc.org/testing/.",
		Impl: chrome.NewLoggedInFixture(
			chrome.ExtraArgs(chromeVModuleArgs...),
			chrome.ExtraArgs(chromeUseHWCodecsForSmallResolutions...),
			chrome.ExtraArgs(chromeSuppressNotificationsArgs...),
			chrome.ExtraArgs(chromeFakeWebcamArgs...),
		),
		Parent:          "gpuWatchDog",
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name: "chromeVideoWithFakeWebcamLacros",
		Desc: "Similar to chromeVideo fixture but supplementing it with the use of a fake video/audio capture device (a.k.a. 'fake webcam'), see https://webrtc.org/testing/ (lacros).",
		Impl: launcher.NewStartedByData(launcher.PreExist,
			chrome.ExtraArgs(chromeVModuleArgs...),
			chrome.LacrosExtraArgs(chromeVModuleArgs...),
			chrome.ExtraArgs(chromeUseHWCodecsForSmallResolutions...),
			chrome.LacrosExtraArgs(chromeUseHWCodecsForSmallResolutions...),
			chrome.ExtraArgs(chromeSuppressNotificationsArgs...),
			chrome.LacrosExtraArgs(chromeSuppressNotificationsArgs...),
			chrome.ExtraArgs(chromeFakeWebcamArgs...),
			chrome.LacrosExtraArgs(chromeFakeWebcamArgs...),
		),
		Parent:          "gpuWatchDog",
		SetUpTimeout:    chrome.LoginTimeout + 7*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Vars:            []string{"lacrosDeployedBinary"},
	})

	testing.AddFixture(&testing.Fixture{
		Name: "chromeVideoWithFakeWebcamAndAlternateVideoDecoder",
		Desc: "Similar to chromeVideoWithFakeWebcam fixture but using the alternative video decoder.",
		Impl: chrome.NewLoggedInFixture(
			chrome.ExtraArgs(chromeVModuleArgs...),
			chrome.ExtraArgs(chromeUseHWCodecsForSmallResolutions...),
			chrome.ExtraArgs(chromeSuppressNotificationsArgs...),
			chrome.ExtraArgs(chromeFakeWebcamArgs...),
			chrome.ExtraArgs("--enable-features=UseAlternateVideoDecoderImplementation"),
		),
		Parent:          "gpuWatchDog",
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name: "chromeVideoWithFakeWebcamAndAlternateVideoDecoderLacros",
		Desc: "Similar to chromeVideoWithFakeWebcam fixture but using the alternative video decoder (lacros).",
		Impl: launcher.NewStartedByData(launcher.PreExist,
			chrome.ExtraArgs(chromeVModuleArgs...),
			chrome.LacrosExtraArgs(chromeVModuleArgs...),
			chrome.ExtraArgs(chromeUseHWCodecsForSmallResolutions...),
			chrome.LacrosExtraArgs(chromeUseHWCodecsForSmallResolutions...),
			chrome.ExtraArgs(chromeSuppressNotificationsArgs...),
			chrome.LacrosExtraArgs(chromeSuppressNotificationsArgs...),
			chrome.ExtraArgs(chromeFakeWebcamArgs...),
			chrome.LacrosExtraArgs(chromeFakeWebcamArgs...),
			chrome.ExtraArgs("--enable-features=UseAlternateVideoDecoderImplementation"),
			chrome.LacrosExtraArgs("--enable-features=UseAlternateVideoDecoderImplementation"),
		),
		Parent:          "gpuWatchDog",
		SetUpTimeout:    chrome.LoginTimeout + 7*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Vars:            []string{"lacrosDeployedBinary"},
	})

	testing.AddFixture(&testing.Fixture{
		Name: "chromeVideoWithFakeWebcamAndForceVP9ThreeTemporalLayers",
		Desc: "Similar to chromeVideoWithFakeWebcam fixture but forcing webrtc vp9 stream to be three temporal layers.",
		Impl: chrome.NewLoggedInFixture(
			chrome.ExtraArgs(chromeVModuleArgs...),
			chrome.ExtraArgs(chromeSuppressNotificationsArgs...),
			chrome.ExtraArgs(chromeFakeWebcamArgs...),
			chrome.ExtraArgs("--force-fieldtrials=WebRTC-SupportVP9SVC/EnabledByFlag_1SL3TL/"),
		),
		Parent:          "gpuWatchDog",
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name: "chromeVideoWithFakeWebcamAndForceVP9ThreeTemporalLayersLacros",
		Desc: "Similar to chromeVideoWithFakeWebcam fixture but forcing webrtc vp9 stream to be three temporal layers (lacros).",
		Impl: launcher.NewStartedByData(launcher.PreExist,
			chrome.ExtraArgs(chromeVModuleArgs...),
			chrome.LacrosExtraArgs(chromeVModuleArgs...),
			chrome.ExtraArgs(chromeSuppressNotificationsArgs...),
			chrome.LacrosExtraArgs(chromeSuppressNotificationsArgs...),
			chrome.ExtraArgs(chromeFakeWebcamArgs...),
			chrome.LacrosExtraArgs(chromeFakeWebcamArgs...),
			chrome.ExtraArgs("--force-fieldtrials=WebRTC-SupportVP9SVC/EnabledByFlag_1SL3TL/"),
			chrome.LacrosExtraArgs("--force-fieldtrials=WebRTC-SupportVP9SVC/EnabledByFlag_1SL3TL/"),
		),
		Parent:          "gpuWatchDog",
		SetUpTimeout:    chrome.LoginTimeout + 7*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Vars:            []string{"lacrosDeployedBinary"},
	})

	testing.AddFixture(&testing.Fixture{
		Name: "chromeVideoWithFakeWebcamAndSWDecoding",
		Desc: "Similar to chromeVideoWithFakeWebcam fixture but hardware decoding disabled.",
		Impl: chrome.NewLoggedInFixture(
			chrome.ExtraArgs(chromeVModuleArgs...),
			chrome.ExtraArgs(chromeSuppressNotificationsArgs...),
			chrome.ExtraArgs(chromeFakeWebcamArgs...),
			chrome.ExtraArgs("--disable-accelerated-video-decode"),
		),
		Parent:          "gpuWatchDog",
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name: "chromeVideoWithFakeWebcamAndSWDecodingLacros",
		Desc: "Similar to chromeVideoWithFakeWebcam fixture but hardware decoding disabled (lacros).",
		Impl: launcher.NewStartedByData(launcher.PreExist,
			chrome.ExtraArgs(chromeVModuleArgs...),
			chrome.LacrosExtraArgs(chromeVModuleArgs...),
			chrome.ExtraArgs(chromeSuppressNotificationsArgs...),
			chrome.LacrosExtraArgs(chromeSuppressNotificationsArgs...),
			chrome.ExtraArgs(chromeFakeWebcamArgs...),
			chrome.LacrosExtraArgs(chromeFakeWebcamArgs...),
			chrome.ExtraArgs("--disable-accelerated-video-decode"),
			chrome.LacrosExtraArgs("--disable-accelerated-video-decode"),
		),
		Parent:          "gpuWatchDog",
		SetUpTimeout:    chrome.LoginTimeout + 7*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Vars:            []string{"lacrosDeployedBinary"},
	})

	testing.AddFixture(&testing.Fixture{
		Name: "chromeVideoWithFakeWebcamAndSWEncoding",
		Desc: "Similar to chromeVideoWithFakeWebcam fixture but hardware encoding disabled.",
		Impl: chrome.NewLoggedInFixture(
			chrome.ExtraArgs(chromeVModuleArgs...),
			chrome.ExtraArgs(chromeSuppressNotificationsArgs...),
			chrome.ExtraArgs(chromeFakeWebcamArgs...),
			chrome.ExtraArgs("--disable-accelerated-video-encode"),
		),
		Parent:          "gpuWatchDog",
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name: "chromeVideoWithFakeWebcamAndSWEncodingLacros",
		Desc: "Similar to chromeVideoWithFakeWebcam fixture but hardware encoding disabled (lacros).",
		Impl: launcher.NewStartedByData(launcher.PreExist,
			chrome.ExtraArgs(chromeVModuleArgs...),
			chrome.LacrosExtraArgs(chromeVModuleArgs...),
			chrome.ExtraArgs(chromeSuppressNotificationsArgs...),
			chrome.LacrosExtraArgs(chromeSuppressNotificationsArgs...),
			chrome.ExtraArgs(chromeFakeWebcamArgs...),
			chrome.LacrosExtraArgs(chromeFakeWebcamArgs...),
			chrome.ExtraArgs("--disable-accelerated-video-encode"),
			chrome.LacrosExtraArgs("--disable-accelerated-video-encode"),
		),
		Parent:          "gpuWatchDog",
		SetUpTimeout:    chrome.LoginTimeout + 7*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Vars:            []string{"lacrosDeployedBinary"},
	})

	testing.AddFixture(&testing.Fixture{
		Name: "chromeScreenCapture",
		Desc: "Logged into a user session with flag so that Chrome always picks the entire screen for getDisplayMedia(), bypassing the picker UI.",
		Impl: chrome.NewLoggedInFixture(
			chrome.ExtraArgs(chromeSuppressNotificationsArgs...),
			chrome.ExtraArgs(`--auto-select-desktop-capture-source=display`),
		),
		Parent:          "gpuWatchDog",
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name: "chromeScreenCaptureLacros",
		Desc: "Logged into a user session with flag so that Chrome always picks the entire screen for getDisplayMedia(), bypassing the picker UI (lacros).",
		Impl: launcher.NewStartedByData(launcher.PreExist,
			chrome.ExtraArgs(chromeSuppressNotificationsArgs...),
			chrome.LacrosExtraArgs(chromeSuppressNotificationsArgs...),
			chrome.ExtraArgs(`--auto-select-desktop-capture-source=display`),
			chrome.LacrosExtraArgs(`--auto-select-desktop-capture-source=display`),
		),
		Parent:          "gpuWatchDog",
		SetUpTimeout:    chrome.LoginTimeout + 7*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Vars:            []string{"lacrosDeployedBinary"},
	})

	testing.AddFixture(&testing.Fixture{
		Name: "chromeWindowCapture",
		Desc: "Logged into a user session with flag so that Chrome always picks the Chromium window for getDisplayMedia(), bypassing the picker UI.",
		Impl: chrome.NewLoggedInFixture(
			chrome.ExtraArgs(chromeSuppressNotificationsArgs...),
			chrome.ExtraArgs(`--auto-select-desktop-capture-source=Chrome`),
		),
		Parent:          "gpuWatchDog",
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name: "chromeWindowCaptureLacros",
		Desc: "Logged into a user session with flag so that Chrome always picks the Chromium window for getDisplayMedia(), bypassing the picker UI (lacros).",
		Impl: launcher.NewStartedByData(launcher.PreExist,
			chrome.ExtraArgs(chromeSuppressNotificationsArgs...),
			chrome.LacrosExtraArgs(chromeSuppressNotificationsArgs...),
			chrome.ExtraArgs(`--auto-select-desktop-capture-source=Chrome`),
			chrome.LacrosExtraArgs(`--auto-select-desktop-capture-source=Chrome`),
		),
		Parent:          "gpuWatchDog",
		SetUpTimeout:    chrome.LoginTimeout + 7*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Vars:            []string{"lacrosDeployedBinary"},
	})

	testing.AddFixture(&testing.Fixture{
		Name: "chromeVideoWithSWDecoding",
		Desc: "Similar to chromeVideo fixture but making sure Chrome does not use any potential hardware accelerated decoding.",
		Impl: chrome.NewLoggedInFixture(
			chrome.ExtraArgs(chromeSuppressNotificationsArgs...),
			chrome.ExtraArgs("--disable-accelerated-video-decode"),
		),
		Parent:          "gpuWatchDog",
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name: "chromeVideoWithSWDecodingLacros",
		Desc: "Similar to chromeVideo fixture but making sure Chrome does not use any potential hardware accelerated decoding (lacros).",
		Impl: launcher.NewStartedByData(launcher.PreExist,
			chrome.ExtraArgs(chromeSuppressNotificationsArgs...),
			chrome.LacrosExtraArgs(chromeSuppressNotificationsArgs...),
			chrome.ExtraArgs("--disable-accelerated-video-decode"),
			chrome.LacrosExtraArgs("--disable-accelerated-video-decode"),
		),
		Parent:          "gpuWatchDog",
		SetUpTimeout:    chrome.LoginTimeout + 7*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Vars:            []string{"lacrosDeployedBinary"},
	})

	testing.AddFixture(&testing.Fixture{
		Name: "chromeVideoWithSWDecodingAndLibGAV1",
		Desc: "Similar to chromeVideoWithSWDecoding fixture but enabling the use of LibGAV1 for AV1 decoding.",
		Impl: chrome.NewLoggedInFixture(
			chrome.ExtraArgs(chromeVModuleArgs...),
			chrome.ExtraArgs(chromeSuppressNotificationsArgs...),
			chrome.ExtraArgs("--disable-accelerated-video-decode", "--enable-features=Gav1VideoDecoder"),
		),
		Parent:          "gpuWatchDog",
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name: "chromeVideoWithSWDecodingAndLibGAV1Lacros",
		Desc: "Similar to chromeVideoWithSWDecoding fixture but enabling the use of LibGAV1 for AV1 decoding (lacros).",
		Impl: launcher.NewStartedByData(launcher.PreExist,
			chrome.ExtraArgs(chromeVModuleArgs...),
			chrome.LacrosExtraArgs(chromeVModuleArgs...),
			chrome.ExtraArgs(chromeSuppressNotificationsArgs...),
			chrome.LacrosExtraArgs(chromeSuppressNotificationsArgs...),
			chrome.ExtraArgs("--disable-accelerated-video-decode", "--enable-features=Gav1VideoDecoder"),
			chrome.LacrosExtraArgs("--disable-accelerated-video-decode", "--enable-features=Gav1VideoDecoder"),
		),
		Parent:          "gpuWatchDog",
		SetUpTimeout:    chrome.LoginTimeout + 7*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Vars:            []string{"lacrosDeployedBinary"},
	})

	// TODO(b/172217032): Remove these *HWAV1Decoding preconditions once the hardware av1 decoder feature is enabled by default.
	testing.AddFixture(&testing.Fixture{
		Name: "chromeVideoWithHWAV1Decoding",
		Desc: "Similar to chromeVideo fixture but also enables hardware accelerated av1 decoding.",
		Impl: chrome.NewLoggedInFixture(
			chrome.ExtraArgs(chromeVModuleArgs...),
			chrome.ExtraArgs(chromeUseHWCodecsForSmallResolutions...),
			chrome.ExtraArgs(chromeSuppressNotificationsArgs...),
			chrome.ExtraArgs("--enable-features=VaapiAV1Decoder"),
		),
		Parent:          "gpuWatchDog",
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	// TODO(b/172217032): Remove these *HWAV1Decoding preconditions once the hardware av1 decoder feature is enabled by default.
	testing.AddFixture(&testing.Fixture{
		Name: "chromeVideoWithHWAV1DecodingLacros",
		Desc: "Similar to chromeVideo fixture but also enables hardware accelerated av1 decoding (lacros).",
		Impl: launcher.NewStartedByData(launcher.PreExist,
			chrome.ExtraArgs(chromeVModuleArgs...),
			chrome.LacrosExtraArgs(chromeVModuleArgs...),
			chrome.ExtraArgs(chromeUseHWCodecsForSmallResolutions...),
			chrome.LacrosExtraArgs(chromeUseHWCodecsForSmallResolutions...),
			chrome.ExtraArgs(chromeSuppressNotificationsArgs...),
			chrome.LacrosExtraArgs(chromeSuppressNotificationsArgs...),
			chrome.ExtraArgs("--enable-features=VaapiAV1Decoder"),
			chrome.LacrosExtraArgs("--enable-features=VaapiAV1Decoder"),
		),
		Parent:          "gpuWatchDog",
		SetUpTimeout:    chrome.LoginTimeout + 7*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Vars:            []string{"lacrosDeployedBinary"},
	})

	testing.AddFixture(&testing.Fixture{
		Name: "chromeVideoWithGuestLoginAndHWAV1Decoding",
		Desc: "Similar to chromeVideoWithGuestLogin fixture but also enables hardware accelerated av1 decoding.",
		Impl: chrome.NewLoggedInFixture(
			chrome.ExtraArgs(chromeVModuleArgs...),
			chrome.ExtraArgs(chromeUseHWCodecsForSmallResolutions...),
			chrome.ExtraArgs(chromeSuppressNotificationsArgs...),
			chrome.ExtraArgs("--enable-features=VaapiAV1Decoder"),
			chrome.GuestLogin(),
		),
		Parent:          "gpuWatchDog",
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name: "chromeVideoWithGuestLoginAndHWAV1DecodingLacros",
		Desc: "Similar to chromeVideoWithGuestLogin fixture but also enables hardware accelerated av1 decoding (lacros).",
		Impl: launcher.NewStartedByData(launcher.PreExist,
			chrome.ExtraArgs(chromeVModuleArgs...),
			chrome.LacrosExtraArgs(chromeVModuleArgs...),
			chrome.ExtraArgs(chromeUseHWCodecsForSmallResolutions...),
			chrome.LacrosExtraArgs(chromeUseHWCodecsForSmallResolutions...),
			chrome.ExtraArgs(chromeSuppressNotificationsArgs...),
			chrome.LacrosExtraArgs(chromeSuppressNotificationsArgs...),
			chrome.ExtraArgs("--enable-features=VaapiAV1Decoder"),
			chrome.LacrosExtraArgs("--enable-features=VaapiAV1Decoder"),
			chrome.GuestLogin(),
		),
		Parent:          "gpuWatchDog",
		SetUpTimeout:    chrome.LoginTimeout + 7*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Vars:            []string{"lacrosDeployedBinary"},
	})

	// TODO(crbug.com/958166): Use simply ChromeVideoWithSWDecoding() when HDR is launched.
	testing.AddFixture(&testing.Fixture{
		Name: "chromeVideoWithSWDecodingAndHDRScreen",
		Desc: "Similar to chromeVideoWithSWDecoding but also enalbing the HDR screen if present.",
		Impl: chrome.NewLoggedInFixture(
			chrome.ExtraArgs(chromeVModuleArgs...),
			chrome.ExtraArgs(chromeSuppressNotificationsArgs...),
			chrome.ExtraArgs("--disable-accelerated-video-decode"),
			chrome.ExtraArgs("--enable-features=UseHDRTransferFunction"),
		),
		Parent:          "gpuWatchDog",
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	// TODO(crbug.com/958166): Use simply ChromeVideoWithSWDecoding() when HDR is launched.
	testing.AddFixture(&testing.Fixture{
		Name: "chromeVideoWithSWDecodingAndHDRScreenLacros",
		Desc: "Similar to chromeVideoWithSWDecoding but also enalbing the HDR screen if present (lacros).",
		Impl: launcher.NewStartedByData(launcher.PreExist,
			chrome.ExtraArgs(chromeVModuleArgs...),
			chrome.LacrosExtraArgs(chromeVModuleArgs...),
			chrome.ExtraArgs(chromeSuppressNotificationsArgs...),
			chrome.LacrosExtraArgs(chromeSuppressNotificationsArgs...),
			chrome.ExtraArgs("--disable-accelerated-video-decode"),
			chrome.LacrosExtraArgs("--disable-accelerated-video-decode"),
			chrome.ExtraArgs("--enable-features=UseHDRTransferFunction"),
			chrome.LacrosExtraArgs("--enable-features=UseHDRTransferFunction"),
		),
		Parent:          "gpuWatchDog",
		SetUpTimeout:    chrome.LoginTimeout + 7*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Vars:            []string{"lacrosDeployedBinary"},
	})

	testing.AddFixture(&testing.Fixture{
		Name: "chromeCameraPerf",
		Desc: "Logged into a user session with camera tests-specific setting and without verbose logging that can affect the performance. This fixture should be used only for performance tests.",
		Impl: chrome.NewLoggedInFixture(
			chrome.ExtraArgs(chromeBypassPermissionsArgs...),
			chrome.ExtraArgs(chromeSuppressNotificationsArgs...),
		),
		Parent:          "gpuWatchDog",
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name: "chromeCameraPerfLacros",
		Desc: "Logged into a user session with camera tests-specific setting and without verbose logging that can affect the performance. This fixture should be used only for performance tests (lacros).",
		Impl: launcher.NewStartedByData(launcher.PreExist,
			chrome.ExtraArgs(chromeBypassPermissionsArgs...),
			chrome.LacrosExtraArgs(chromeBypassPermissionsArgs...),
			chrome.ExtraArgs(chromeSuppressNotificationsArgs...),
			chrome.LacrosExtraArgs(chromeSuppressNotificationsArgs...),
		),
		Parent:          "gpuWatchDog",
		SetUpTimeout:    chrome.LoginTimeout + 7*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Vars:            []string{"lacrosDeployedBinary"},
	})

	testing.AddFixture(&testing.Fixture{
		Name: "chromeFakeCameraPerf",
		Desc: "Logged into a user session with fake video/audio capture device (a.k.a. 'fake webcam', see https://webrtc.org/testing), without asking for user permission, and without verboselogging that can affect the performance. This fixture should be used only used for performance tests.",
		Impl: chrome.NewLoggedInFixture(
			chrome.ExtraArgs(chromeFakeWebcamArgs...),
			chrome.ExtraArgs(chromeSuppressNotificationsArgs...),
		),
		Parent:          "gpuWatchDog",
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name: "chromeFakeCameraPerfLacros",
		Desc: "Logged into a user session with fake video/audio capture device (a.k.a. 'fake webcam', see https://webrtc.org/testing), without asking for user permission, and without verboselogging that can affect the performance. This fixture should be used only used for performance tests (lacros).",
		Impl: launcher.NewStartedByData(launcher.PreExist,
			chrome.ExtraArgs(chromeFakeWebcamArgs...),
			chrome.LacrosExtraArgs(chromeFakeWebcamArgs...),
			chrome.ExtraArgs(chromeSuppressNotificationsArgs...),
			chrome.LacrosExtraArgs(chromeSuppressNotificationsArgs...),
		),
		Parent:          "gpuWatchDog",
		SetUpTimeout:    chrome.LoginTimeout + 7*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Vars:            []string{"lacrosDeployedBinary"},
	})
}

var chromeVModuleArgs = []string{
	// Enable verbose log messages for video components.
	"--vmodule=" + strings.Join([]string{
		"*/media/gpu/chromeos/*=2",
		"*/media/gpu/vaapi/*=2",
		"*/media/gpu/v4l2/*=2"}, ",")}

var chromeUseHWCodecsForSmallResolutions = []string{
	// The Renderer video stack might have a policy of not using hardware
	// accelerated decoding for certain small resolutions (see crbug.com/684792).
	// Disable that for testing.
	"--disable-features=ResolutionBasedDecoderPriority",
	// VA-API HW decoder and encoder might reject small resolutions for
	// performance (see crbug.com/1008491 and b/171041334).
	// Disable that for testing.
	"--disable-features=VaapiEnforceVideoMinMaxResolution"}

var chromeFakeWebcamArgs = []string{
	// Use a fake media capture device instead of live webcam(s)/microphone(s).
	"--use-fake-device-for-media-stream",
	// Avoid the need to grant camera/microphone permissions.
	"--use-fake-ui-for-media-stream"}

var chromeBypassPermissionsArgs = []string{
	// Avoid the need to grant camera/microphone permissions.
	"--use-fake-ui-for-media-stream"}

var chromeSuppressNotificationsArgs = []string{
	// Do not show message center notifications.
	"--suppress-message-center-popups"}
