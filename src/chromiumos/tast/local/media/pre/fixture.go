// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package pre

import (
	"context"
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
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{
				chrome.ExtraArgs(chromeVModuleArgs...),
				chrome.ExtraArgs(chromeUseHWCodecsForSmallResolutions...),
				chrome.ExtraArgs(chromeBypassPermissionsArgs...),
				chrome.ExtraArgs(chromeSuppressNotificationsArgs...),
			}, nil
		}),
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
		Vars:            []string{"lacrosDeployedBinary", "lacrosIsChromeBranded"},
	})

	// Chrome has two said implementations: a "legacy" one and a Direct, VD-based on. Selecting one ore the other depends on the hardware and is ultimately determined by the overlays/ flags. Tests should be centered on what the users see, hence most of the testing should use chromeVideo, with a few test cases using this fixture.
	testing.AddFixture(&testing.Fixture{
		Name: "chromeAlternateVideoDecoder",
		Desc: "Logged into a user session with alternate hardware accelerated video decoder.",
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{
				chrome.ExtraArgs(chromeVModuleArgs...),
				chrome.ExtraArgs(chromeUseHWCodecsForSmallResolutions...),
				chrome.ExtraArgs(chromeSuppressNotificationsArgs...),
				chrome.EnableFeatures("UseAlternateVideoDecoderImplementation"),
			}, nil
		}),
		Parent:          "gpuWatchDog",
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name: "chromeVideoWithGuestLogin",
		Desc: "Similar to chromeVideo fixture but forcing login as a guest.",
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{
				chrome.ExtraArgs(chromeVModuleArgs...),
				chrome.ExtraArgs(chromeUseHWCodecsForSmallResolutions...),
				chrome.ExtraArgs(chromeSuppressNotificationsArgs...),
				chrome.GuestLogin(),
			}, nil
		}),
		Parent:          "gpuWatchDog",
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	// TODO(crbug.com/958166): Use simply ChromeVideo() when HDR is launched.
	testing.AddFixture(&testing.Fixture{
		Name: "chromeVideoWithHDRScreen",
		Desc: "Similar to chromeVideo fixture but enabling the HDR screen if present.",
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{
				chrome.ExtraArgs(chromeVModuleArgs...),
				chrome.ExtraArgs(chromeUseHWCodecsForSmallResolutions...),
				chrome.ExtraArgs(chromeSuppressNotificationsArgs...),
				chrome.EnableFeatures("UseHDRTransferFunction"),
			}, nil
		}),
		Parent:          "gpuWatchDog",
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name: "chromeCompositedVideo",
		Desc: "Similar to chromeVideo fixture but disabling hardware overlays entirely to force video to be composited.",
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{
				chrome.ExtraArgs(chromeVModuleArgs...),
				chrome.ExtraArgs(chromeUseHWCodecsForSmallResolutions...),
				chrome.ExtraArgs(chromeSuppressNotificationsArgs...),
				chrome.ExtraArgs("--enable-hardware-overlays=\"\""),
			}, nil
		}),
		Parent:          "gpuWatchDog",
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name: "chromeVideoWithFakeWebcam",
		Desc: "Similar to chromeVideo fixture but supplementing it with the use of a fake video/audio capture device (a.k.a. 'fake webcam'), see https://webrtc.org/testing/.",
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{
				chrome.ExtraArgs(chromeVModuleArgs...),
				chrome.ExtraArgs(chromeUseHWCodecsForSmallResolutions...),
				chrome.ExtraArgs(chromeSuppressNotificationsArgs...),
				chrome.ExtraArgs(chromeFakeWebcamArgs...),
			}, nil
		}),
		Parent:          "gpuWatchDog",
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name: "chromeVideoWithFakeWebcamAndAlternateVideoDecoder",
		Desc: "Similar to chromeVideoWithFakeWebcam fixture but using the alternative video decoder.",
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{
				chrome.ExtraArgs(chromeVModuleArgs...),
				chrome.ExtraArgs(chromeUseHWCodecsForSmallResolutions...),
				chrome.ExtraArgs(chromeSuppressNotificationsArgs...),
				chrome.ExtraArgs(chromeFakeWebcamArgs...),
				chrome.EnableFeatures("UseAlternateVideoDecoderImplementation"),
			}, nil
		}),
		Parent:          "gpuWatchDog",
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name: "chromeVideoWithFakeWebcamAndForceVP9ThreeTemporalLayers",
		Desc: "Similar to chromeVideoWithFakeWebcam fixture but forcing webrtc vp9 stream to be three temporal layers..",
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{
				chrome.ExtraArgs(chromeVModuleArgs...),
				chrome.ExtraArgs(chromeSuppressNotificationsArgs...),
				chrome.ExtraArgs(chromeFakeWebcamArgs...),
				chrome.ExtraArgs("--force-fieldtrials=WebRTC-SupportVP9SVC/EnabledByFlag_1SL3TL/"),
			}, nil
		}),
		Parent:          "gpuWatchDog",
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name: "chromeVideoWithFakeWebcamAndSWDecoding",
		Desc: "Similar to chromeVideoWithFakeWebcam fixture but hardware decoding disabled.",
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{
				chrome.ExtraArgs(chromeVModuleArgs...),
				chrome.ExtraArgs(chromeSuppressNotificationsArgs...),
				chrome.ExtraArgs(chromeFakeWebcamArgs...),
				chrome.ExtraArgs("--disable-accelerated-video-decode"),
			}, nil
		}),
		Parent:          "gpuWatchDog",
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name: "chromeVideoWithFakeWebcamAndSWEncoding",
		Desc: "Similar to chromeVideoWithFakeWebcam fixture but hardware encoding disabled.",
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{
				chrome.ExtraArgs(chromeVModuleArgs...),
				chrome.ExtraArgs(chromeSuppressNotificationsArgs...),
				chrome.ExtraArgs(chromeFakeWebcamArgs...),
				chrome.ExtraArgs("--disable-accelerated-video-encode"),
			}, nil
		}),
		Parent:          "gpuWatchDog",
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name: "chromeScreenCapture",
		Desc: "Logged into a user session with flag so that Chrome always picks the entire screen for getDisplayMedia(), bypassing the picker UI.",
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{
				chrome.ExtraArgs(chromeSuppressNotificationsArgs...),
				chrome.ExtraArgs(`--auto-select-desktop-capture-source=display`),
			}, nil
		}),
		Parent:          "gpuWatchDog",
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name: "chromeWindowCapture",
		Desc: "Logged into a user session with flag so that Chrome always picks the Chromium window for getDisplayMedia(), bypassing the picker UI.",
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{
				chrome.ExtraArgs(chromeSuppressNotificationsArgs...),
				chrome.ExtraArgs(`--auto-select-desktop-capture-source=Chrome`),
			}, nil
		}),
		Parent:          "gpuWatchDog",
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name: "chromeVideoWithSWDecoding",
		Desc: "Similar to chromeVideo fixture but making sure Chrome does not use any potential hardware accelerated decoding.",
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{
				chrome.ExtraArgs(chromeSuppressNotificationsArgs...),
				chrome.ExtraArgs("--disable-accelerated-video-decode"),
			}, nil
		}),
		Parent:          "gpuWatchDog",
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name: "chromeVideoWithSWDecodingAndLibGAV1",
		Desc: "Similar to chromeVideoWithSWDecoding fixture but enabling the use of LibGAV1 for AV1 decoding.",
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{
				chrome.ExtraArgs(chromeVModuleArgs...),
				chrome.ExtraArgs(chromeSuppressNotificationsArgs...),
				chrome.ExtraArgs("--disable-accelerated-video-decode", "--enable-features=Gav1VideoDecoder"),
			}, nil
		}),
		Parent:          "gpuWatchDog",
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	// TODO(b/172217032): Remove these *HWAV1Decoding preconditions once the hardware av1 decoder feature is enabled by default.
	testing.AddFixture(&testing.Fixture{
		Name: "chromeVideoWithHWAV1Decoding",
		Desc: "Similar to chromeVideo fixture but also enables hardware accelerated av1 decoding.",
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{
				chrome.ExtraArgs(chromeVModuleArgs...),
				chrome.ExtraArgs(chromeUseHWCodecsForSmallResolutions...),
				chrome.ExtraArgs(chromeSuppressNotificationsArgs...),
				chrome.ExtraArgs("--enable-features=VaapiAV1Decoder"),
			}, nil
		}),
		Parent:          "gpuWatchDog",
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name: "chromeVideoWithGuestLoginAndHWAV1Decoding",
		Desc: "Similar to chromeVideoWithGuestLogin fixture but also enables hardware accelerated av1 decoding.",
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{
				chrome.ExtraArgs(chromeVModuleArgs...),
				chrome.ExtraArgs(chromeUseHWCodecsForSmallResolutions...),
				chrome.ExtraArgs(chromeSuppressNotificationsArgs...),
				chrome.ExtraArgs("--enable-features=VaapiAV1Decoder"),
				chrome.GuestLogin(),
			}, nil
		}),
		Parent:          "gpuWatchDog",
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	// TODO(crbug.com/958166): Use simply ChromeVideoWithSWDecoding() when HDR is launched.
	testing.AddFixture(&testing.Fixture{
		Name: "chromeVideoWithSWDecodingAndHDRScreen",
		Desc: "Similar to chromeVideoWithSWDecoding but also enalbing the HDR screen if present.",
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{
				chrome.ExtraArgs(chromeVModuleArgs...),
				chrome.ExtraArgs(chromeSuppressNotificationsArgs...),
				chrome.ExtraArgs("--disable-accelerated-video-decode"),
				chrome.EnableFeatures("UseHDRTransferFunction"),
			}, nil
		}),
		Parent:          "gpuWatchDog",
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name: "chromeCameraPerf",
		Desc: "Logged into a user session with camera tests-specific setting and without verbose logging that can affect the performance. This fixture should be used only for performance tests.",
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{
				chrome.ExtraArgs(chromeBypassPermissionsArgs...),
				chrome.ExtraArgs(chromeSuppressNotificationsArgs...),
			}, nil
		}),
		Parent:          "gpuWatchDog",
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name: "chromeFakeCameraPerf",
		Desc: "Logged into a user session with fake video/audio capture device (a.k.a. 'fake webcam', see https://webrtc.org/testing), without asking for user permission, and without verboselogging that can affect the performance. This fixture should be used only used for performance tests.",
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{
				chrome.ExtraArgs(chromeFakeWebcamArgs...),
				chrome.ExtraArgs(chromeSuppressNotificationsArgs...),
			}, nil
		}),
		Parent:          "gpuWatchDog",
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name: "chromeVideoWithClearHEVCHWDecoding",
		Desc: "Similar to chromeVideo fixture but also enables hardware accelerated HEVC decoding for clear content.",
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{
				chrome.ExtraArgs(chromeVModuleArgs...),
				chrome.ExtraArgs(chromeSuppressNotificationsArgs...),
				chrome.ExtraArgs(chromeUseClearHEVCHWDecodingArgs...),
			}, nil
		}),
		Parent:          "gpuWatchDog",
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name: "chromeVideoWithGuestLoginAndClearHEVCHWDecoding",
		Desc: "Similar to chromeVideo fixture but forcing login as a guest and enables hardware accelerated HEVC decoding for clear content.",
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{
				chrome.ExtraArgs(chromeVModuleArgs...),
				chrome.ExtraArgs(chromeSuppressNotificationsArgs...),
				chrome.ExtraArgs(chromeUseClearHEVCHWDecodingArgs...),
				chrome.GuestLogin(),
			}, nil
		}),
		Parent:          "gpuWatchDog",
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name: "chromeVideoWithDistinctiveIdentifier",
		Desc: "Similar to chromeVideo fixture but also allows a distinctive identifier which is needed for HWDRM.",
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{
				chrome.ExtraArgs(chromeVModuleArgs...),
				chrome.ExtraArgs(chromeSuppressNotificationsArgs...),
				chrome.ExtraArgs(chromeAllowDistinctiveIdentifierArgs...),
				chrome.ExtraArgs(chromeUseHWCodecsForSmallResolutions...),
				chrome.ExtraArgs(chromeBypassPermissionsArgs...),
			}, nil
		}),
		Parent:          "gpuWatchDog",
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	// We need to allow clear HEVC for DRM content because Shaka player needs that
	// to function properly for HEVC.
	// TODO(b/182171409): Remove this once Shaka player is updated to support HEVC
	// without this flag.
	testing.AddFixture(&testing.Fixture{
		Name: "chromeVideoWithClearHEVCHWDecodingAndDistinctiveIdentifier",
		Desc: "Similar to chromeVideo fixture but also allows a distinctive identifer and enables hardware accelerated HEVC decoding for clear content.",
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{
				chrome.ExtraArgs(chromeVModuleArgs...),
				chrome.ExtraArgs(chromeSuppressNotificationsArgs...),
				chrome.ExtraArgs(chromeUseClearHEVCHWDecodingArgs...),
				chrome.ExtraArgs(chromeAllowDistinctiveIdentifierArgs...),
			}, nil
		}),
		Parent:          "gpuWatchDog",
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
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

var chromeUseClearHEVCHWDecodingArgs = []string{
	// Enable playback of unencrypted HEVC content in Chrome.
	"--enable-clear-hevc-for-testing"}

var chromeAllowDistinctiveIdentifierArgs = []string{
	// Allows distinctive identifier with DRM playback when in dev mode. We don't
	// actually use RA for this, but it correlates to the same flag.
	"--allow-ra-in-dev-mode",
	// Prevents showing permission prompt and automatically grants permission to
	// allow a distinctive identifier for localhost which is where we server the
	// DRM content from in the test.
	"--unsafely-allow-protected-media-identifier-for-domain=127.0.0.1"}
