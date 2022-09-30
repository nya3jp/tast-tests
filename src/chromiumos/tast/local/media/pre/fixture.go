// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package pre

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/lacros/lacrosfixt"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name:     "chromeVideo",
		Desc:     "Logged into a user session with logging enabled",
		Contacts: []string{"chromeos-gfx-video@google.com"},
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{
				chrome.ExtraArgs(chromeVideoArgs...),
				chrome.ExtraArgs(chromeBypassPermissionsArgs...),
			}, nil
		}),
		Parent:          "gpuWatchDog",
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:     "chromeVideoOOPVD",
		Desc:     "Logged into a user session with logging and out-of-process video decoding enabled",
		Contacts: []string{"chromeos-gfx-video@google.com"},
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{
				chrome.ExtraArgs(chromeVideoArgs...),
				chrome.ExtraArgs(chromeBypassPermissionsArgs...),
				chrome.EnableFeatures("UseOutOfProcessVideoDecoding"),
			}, nil
		}),
		Parent:          "gpuWatchDog",
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:     "chromeVideoLacros",
		Desc:     "Logged into a user session with logging enabled (lacros)",
		Contacts: []string{"chromeos-gfx-video@google.com"},
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return lacrosfixt.NewConfig(lacrosfixt.ChromeOptions(
				chrome.ExtraArgs(chromeVideoArgs...),
				chrome.LacrosExtraArgs(chromeVideoArgs...),
				chrome.ExtraArgs(chromeBypassPermissionsArgs...),
				chrome.LacrosExtraArgs(chromeBypassPermissionsArgs...))).Opts()
		}),
		Parent:          "gpuWatchDog",
		SetUpTimeout:    chrome.LoginTimeout + 7*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:     "chromeVideoLacrosOOPVD",
		Desc:     "Logged into a user session with logging and out-of-process video decoding enabled (lacros)",
		Contacts: []string{"chromeos-gfx-video@google.com"},
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return lacrosfixt.NewConfig(lacrosfixt.ChromeOptions(
				chrome.ExtraArgs(chromeVideoArgs...),
				chrome.LacrosExtraArgs(chromeVideoArgs...),
				chrome.ExtraArgs(chromeBypassPermissionsArgs...),
				chrome.LacrosExtraArgs(chromeBypassPermissionsArgs...),
				chrome.EnableFeatures("UseOutOfProcessVideoDecoding"),
				chrome.LacrosEnableFeatures("UseOutOfProcessVideoDecoding"))).Opts()
		}),
		Parent:          "gpuWatchDog",
		SetUpTimeout:    chrome.LoginTimeout + 7*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:     "chromeCameraPerfLacros",
		Desc:     "Logged into a user session on Lacros without verbose logging that can affect the performance",
		Contacts: []string{"chromeos-camera-eng@google.com"},
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return lacrosfixt.NewConfig(lacrosfixt.ChromeOptions(
				chrome.ExtraArgs(chromeBypassPermissionsArgs...),
				chrome.LacrosExtraArgs(chromeBypassPermissionsArgs...),
				chrome.ExtraArgs(chromeSuppressNotificationsArgs...),
				chrome.LacrosExtraArgs(chromeSuppressNotificationsArgs...))).Opts()
		}),
		Parent:          "gpuWatchDog",
		SetUpTimeout:    chrome.LoginTimeout + 7*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	// Chrome has two said implementations: a "legacy" one and a Direct, VD-based on. Selecting one ore the other depends on the hardware and is ultimately determined by the overlays/ flags. Tests should be centered on what the users see, hence most of the testing should use chromeVideo, with a few test cases using this fixture.
	testing.AddFixture(&testing.Fixture{
		Name:     "chromeAlternateVideoDecoder",
		Desc:     "Logged into a user session with alternate hardware accelerated video decoder",
		Contacts: []string{"chromeos-gfx-video@google.com"},
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{
				chrome.ExtraArgs(chromeVideoArgs...),
				chrome.EnableFeatures("UseAlternateVideoDecoderImplementation"),
			}, nil
		}),
		Parent:          "gpuWatchDog",
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:     "chromeVideoWithGuestLogin",
		Desc:     "Similar to chromeVideo fixture but forcing login as a guest",
		Contacts: []string{"chromeos-gfx-video@google.com"},
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{
				chrome.ExtraArgs(chromeVideoArgs...),
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
		Name:     "chromeVideoWithHDRScreen",
		Desc:     "Similar to chromeVideo fixture but enabling the HDR screen if present",
		Contacts: []string{"chromeos-gfx-video@google.com"},
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{
				chrome.ExtraArgs(chromeVideoArgs...),
				chrome.EnableFeatures("UseHDRTransferFunction"),
			}, nil
		}),
		Parent:          "gpuWatchDog",
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:     "chromeCompositedVideo",
		Desc:     "Similar to chromeVideo fixture but disabling hardware overlays entirely to force video to be composited",
		Contacts: []string{"chromeos-gfx-video@google.com"},
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{
				chrome.ExtraArgs(chromeVideoArgs...),
				chrome.ExtraArgs("--enable-hardware-overlays=\"\""),
			}, nil
		}),
		Parent:          "gpuWatchDog",
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:     "chromeAshCompositedVideoLacros",
		Desc:     "Similar to chromeVideoLacros fixture but disabling hardware overlays in ash-chrome entirely to force video to be composited",
		Contacts: []string{"chromeos-gfx-video@google.com"},
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return lacrosfixt.NewConfig(lacrosfixt.ChromeOptions(
				chrome.ExtraArgs(chromeVideoArgs...),
				chrome.LacrosExtraArgs(chromeVideoArgs...),
				chrome.ExtraArgs("--enable-hardware-overlays=\"\""))).Opts()
		}),
		Parent:          "gpuWatchDog",
		SetUpTimeout:    chrome.LoginTimeout + 7*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:     "chromeLacrosCompositedVideoLacros",
		Desc:     "Similar to chromeVideoLacros fixture but disabling hardware overlays in lacros-chrome entirely to force video to be composited",
		Contacts: []string{"chromeos-gfx-video@google.com"},
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return lacrosfixt.NewConfig(lacrosfixt.ChromeOptions(
				chrome.ExtraArgs(chromeVideoArgs...),
				chrome.LacrosExtraArgs(chromeVideoArgs...),
				chrome.LacrosExtraArgs("--enable-hardware-overlays=\"\""))).Opts()
		}),
		Parent:          "gpuWatchDog",
		SetUpTimeout:    chrome.LoginTimeout + 7*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:     "chromeVideoWithFakeWebcam",
		Desc:     "Similar to chromeVideo fixture but supplementing it with the use of a fake video/audio capture device (a.k.a. 'fake webcam'), see https://webrtc.org/testing/",
		Contacts: []string{"chromeos-gfx-video@google.com"},
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{
				chrome.ExtraArgs(chromeVideoArgs...),
				chrome.ExtraArgs(chromeFakeWebcamArgs...),
			}, nil
		}),
		Parent:          "gpuWatchDog",
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:     "chromeVideoWithFakeWebcamAndAlternateVideoDecoder",
		Desc:     "Similar to chromeVideoWithFakeWebcam fixture but using the alternative video decoder",
		Contacts: []string{"chromeos-gfx-video@google.com"},
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{
				chrome.ExtraArgs(chromeVideoArgs...),
				chrome.ExtraArgs(chromeFakeWebcamArgs...),
				chrome.EnableFeatures("UseAlternateVideoDecoderImplementation"),
			}, nil
		}),
		Parent:          "gpuWatchDog",
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	// TODO(b/236546408): Remove once hardware variable bitrate encoding is enabled by default.
	testing.AddFixture(&testing.Fixture{
		Name:     "chromeVideoWithFakeWebcamAndHWVBREncoding",
		Desc:     "Similar to chromeVideoWebCam but enabling hardware VBR encoding",
		Contacts: []string{"chromeos-gfx-video@google.com"},
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{
				chrome.ExtraArgs(chromeVideoArgs...),
				chrome.ExtraArgs(chromeFakeWebcamArgs...),
				chrome.ExtraArgs("--enable-features=ChromeOSHWVBREncoding"),
			}, nil
		}),
		Parent:          "gpuWatchDog",
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:     "chromeVideoWithFakeWebcamAndSVCEnabled",
		Desc:     "Similar to chromeVideoWithFakeWebcam fixture but allowing use of the Web SVC API",
		Contacts: []string{"chromeos-gfx-video@google.com"},
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{
				chrome.ExtraArgs(chromeVideoArgs...),
				chrome.ExtraArgs(chromeFakeWebcamArgs...),
				chrome.ExtraArgs("--enable-blink-features=RTCSvcScalabilityMode"),
			}, nil
		}),
		Parent:          "gpuWatchDog",
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	// TODO(b/202926617): Remove once vp8 hardware temporal layer encoding is enabled by default.
	testing.AddFixture(&testing.Fixture{
		Name:     "chromeVideoWithFakeWebcamAndSVCEnabledWithHWVp8TemporalLayerEncoding",
		Desc:     "Similar to chromeVideoWithFakeWebcamAndSVCEnabled but enabling vp8 hardware temporal layer encoding",
		Contacts: []string{"chromeos-gfx-video@google.com"},
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{
				chrome.ExtraArgs(chromeVideoArgs...),
				chrome.ExtraArgs(chromeFakeWebcamArgs...),
				chrome.ExtraArgs("--enable-blink-features=RTCSvcScalabilityMode"),
				chrome.ExtraArgs("--enable-features=VaapiVp8TemporalLayerEncoding"),
			}, nil
		}),
		Parent:          "gpuWatchDog",
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:     "chromeVideoWithFakeWebcamAndSVCEnabledAndSWEncoding",
		Desc:     "Similar to chromeVideoWithFakeWebcam fixture but allowing use of the Web SVC API and disabling hardware encoding",
		Contacts: []string{"chromeos-gfx-video@google.com"},
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{
				chrome.ExtraArgs(chromeVideoArgs...),
				chrome.ExtraArgs(chromeFakeWebcamArgs...),
				chrome.ExtraArgs("--enable-blink-features=RTCSvcScalabilityMode"),
				chrome.ExtraArgs("--disable-accelerated-video-encode"),
			}, nil
		}),
		Parent:          "gpuWatchDog",
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:     "chromeVideoWithFakeWebcamAndEnableVaapiVideoMinResolution",
		Desc:     "Similar to chromeVideoWithFakeWebcam fixture but enable VaapiVideoMinResolutionForPerformance feature",
		Contacts: []string{"chromeos-gfx-video@google.com"},
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{
				chrome.ExtraArgs(chromeVideoArgsWithEnablingVaapiVEAMinResolution...),
				chrome.ExtraArgs(chromeFakeWebcamArgs...),
			}, nil
		}),
		Parent:          "gpuWatchDog",
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:     "chromeVideoWithFakeWebcamAndNoHwAcceleration",
		Desc:     "Similar to chromeVideoWithFakeWebcam fixture but with both hardware decoding and encoding disabled",
		Contacts: []string{"chromeos-gfx-video@google.com"},
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{
				chrome.ExtraArgs(chromeVideoArgs...),
				chrome.ExtraArgs(chromeFakeWebcamArgs...),
				chrome.ExtraArgs("--disable-accelerated-video-decode"),
				chrome.ExtraArgs("--disable-accelerated-video-encode"),
			}, nil
		}),
		Parent:          "gpuWatchDog",
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:     "chromeVideoWithFakeWebcamAndSWEncoding",
		Desc:     "Similar to chromeVideoWithFakeWebcam fixture but hardware encoding disabled",
		Contacts: []string{"chromeos-gfx-video@google.com"},
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{
				chrome.ExtraArgs(chromeVideoArgs...),
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
		Name:     "chromeVideoWithFakeWebcamAndGlobalVaapiLockDisabled",
		Desc:     "Similar to chromeVideoWithFakeWebcam fixture but the global VA-API lock is disabled if applicable",
		Contacts: []string{"chromeos-gfx-video@google.com"},
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{
				chrome.ExtraArgs(chromeVideoArgs...),
				chrome.ExtraArgs(chromeFakeWebcamArgs...),
				chrome.ExtraArgs("--disable-features=GlobalVaapiLock"),
			}, nil
		}),
		Parent:          "gpuWatchDog",
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:     "chromeScreenCapture",
		Desc:     "Logged into a user session with flag so that Chrome always picks the entire screen for getDisplayMedia(), bypassing the picker UI",
		Contacts: []string{"chromeos-gfx-video@google.com"},
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{
				chrome.ExtraArgs(chromeVideoArgs...),
				chrome.ExtraArgs(`--auto-select-desktop-capture-source=display`),
			}, nil
		}),
		Parent:          "gpuWatchDog",
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:     "chromeWindowCapture",
		Desc:     "Logged into a user session with flag so that Chrome always picks the Chromium window for getDisplayMedia(), bypassing the picker UI",
		Contacts: []string{"chromeos-gfx-video@google.com"},
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{
				chrome.ExtraArgs(chromeVideoArgs...),
				chrome.ExtraArgs(`--auto-select-desktop-capture-source=Chrome`),
			}, nil
		}),
		Parent:          "gpuWatchDog",
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:     "chromeTabCapture",
		Desc:     "Logged into a user session with flag so that Chrome always picks the current tab for getDisplayMedia(), bypassing the picker UI",
		Contacts: []string{"chromeos-gfx-video@google.com"},
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{
				chrome.ExtraArgs(chromeVideoArgs...),
				// Chrome automatically selects a tab page whose title contains "test".
				chrome.ExtraArgs("--auto-select-tab-capture-source-by-title=test"),
			}, nil
		}),
		Parent:          "gpuWatchDog",
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:     "chromeVideoWithSWDecoding",
		Desc:     "Similar to chromeVideo fixture but making sure Chrome does not use any potential hardware accelerated decoding",
		Contacts: []string{"chromeos-gfx-video@google.com"},
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{
				chrome.ExtraArgs(chromeVideoArgs...),
				chrome.ExtraArgs("--disable-accelerated-video-decode"),
			}, nil
		}),
		Parent:          "gpuWatchDog",
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:     "chromeVideoWithSWDecodingAndLibGAV1",
		Desc:     "Similar to chromeVideoWithSWDecoding fixture but enabling the use of LibGAV1 for AV1 decoding",
		Contacts: []string{"chromeos-gfx-video@google.com"},
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{
				chrome.ExtraArgs(chromeVideoArgs...),
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
		Name:     "chromeVideoWithHWAV1Decoding",
		Desc:     "Similar to chromeVideo fixture but also enables hardware accelerated av1 decoding",
		Contacts: []string{"chromeos-gfx-video@google.com"},
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{
				chrome.ExtraArgs(chromeVideoArgs...),
				chrome.ExtraArgs("--enable-features=VaapiAV1Decoder"),
			}, nil
		}),
		Parent:          "gpuWatchDog",
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:     "chromeVideoWithGuestLoginAndHWAV1Decoding",
		Desc:     "Similar to chromeVideoWithGuestLogin fixture but also enables hardware accelerated av1 decoding",
		Contacts: []string{"chromeos-gfx-video@google.com"},
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{
				chrome.ExtraArgs(chromeVideoArgs...),
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
		Name:     "chromeVideoWithSWDecodingAndHDRScreen",
		Desc:     "Similar to chromeVideoWithSWDecoding but also enalbing the HDR screen if present",
		Contacts: []string{"chromeos-gfx-video@google.com"},
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{
				chrome.ExtraArgs(chromeVideoArgs...),
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
		Name:     "chromeCameraPerf",
		Desc:     "Logged into a user session with camera tests-specific setting and without verbose logging that can affect the performance. This fixture should be used only for performance tests",
		Contacts: []string{"chromeos-camera-eng@google.com"},
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
		Name:     "chromeFakeCameraPerf",
		Desc:     "Logged into a user session with fake video/audio capture device (a.k.a. 'fake webcam', see https://webrtc.org/testing), without asking for user permission, and without verboselogging that can affect the performance. This fixture should be used only used for performance tests",
		Contacts: []string{"chromeos-camera-eng@google.com"},
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{
				chrome.ExtraArgs(chromeVideoArgs...),
				chrome.ExtraArgs(chromeFakeWebcamArgs...),
			}, nil
		}),
		Parent:          "gpuWatchDog",
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:     "chromeVideoWithDistinctiveIdentifier",
		Desc:     "Similar to chromeVideo fixture but also allows a distinctive identifier which is needed for HWDRM",
		Contacts: []string{"chromeos-gfx-video@google.com"},
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{
				chrome.ExtraArgs(chromeVideoArgs...),
				chrome.ExtraArgs(chromeBypassPermissionsArgs...),
				chrome.ExtraArgs(chromeAllowDistinctiveIdentifierArgs...),
			}, nil
		}),
		Parent:          "gpuWatchDog",
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:     "chromeWebCodecs",
		Desc:     "Similar to chromeVideo fixture but enabling using WebCodecs API",
		Contacts: []string{"chromeos-gfx-video@google.com"},
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{
				chrome.ExtraArgs(chromeVideoArgs...),
				chrome.ExtraArgs(chromeWebCodecsArgs...),
			}, nil
		}),
		Parent:          "gpuWatchDog",
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	// TODO(b/202926617): Remove once vp8 hardware temporal layer encoding is enabled by default.
	testing.AddFixture(&testing.Fixture{
		Name:     "chromeWebCodecsWithHWVp8TemporalLayerEncoding",
		Desc:     "Similar to chromeVideo fixture but enabling using WebCodecs API and vp8 hardware temporal layer encoding",
		Contacts: []string{"chromeos-gfx-video@google.com"},
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{
				chrome.ExtraArgs(chromeVideoArgs...),
				chrome.ExtraArgs(chromeWebCodecsArgs...),
				chrome.ExtraArgs("--enable-features=VaapiVp8TemporalLayerEncoding"),
			}, nil
		}),
		Parent:          "gpuWatchDog",
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	// TODO(b/236546408): Remove once hardware variable bitrate encoding is enabled by default.
	testing.AddFixture(&testing.Fixture{
		Name:     "chromeWebCodecsWithHWVBREncoding",
		Desc:     "Similar to chromeVideo fixture but enabling using WebCodecs API and hardware variable bitrate encoding",
		Contacts: []string{"chromeos-gfx-video@google.com"},
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{
				chrome.ExtraArgs(chromeVideoArgs...),
				chrome.ExtraArgs(chromeWebCodecsArgs...),
				chrome.ExtraArgs("--enable-features=ChromeOSHWVBREncoding"),
			}, nil
		}),
		Parent:          "gpuWatchDog",
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:     "chromeVideoWith1DecoderThreadsAndGlobalVaapiLockDisabled",
		Desc:     "Similar to chromeVideo fixture but max decoder threads is set to 1 and disable global VA-API lock",
		Contacts: []string{"chromeos-gfx-video@google.com"},
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{
				chrome.ExtraArgs(chromeVideoArgs...),
				chrome.ExtraArgs("--disable-features=GlobalVaapiLock"),
				chrome.ExtraArgs("--disable-features=LimitConcurrentDecoderInstances"),
				chrome.ExtraArgs("--max-chromeos-decoder-threads=1"),
			}, nil
		}),
		Parent:          "gpuWatchDog",
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:     "chromeVideoWith2DecoderThreadsAndGlobalVaapiLockDisabled",
		Desc:     "Similar to chromeVideo fixture but max decoder threads is set to 2 and disable global VA-API lock",
		Contacts: []string{"chromeos-gfx-video@google.com"},
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{
				chrome.ExtraArgs(chromeVideoArgs...),
				chrome.ExtraArgs("--disable-features=GlobalVaapiLock"),
				chrome.ExtraArgs("--disable-features=LimitConcurrentDecoderInstances"),
				chrome.ExtraArgs("--max-chromeos-decoder-threads=2"),
			}, nil
		}),
		Parent:          "gpuWatchDog",
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})
	testing.AddFixture(&testing.Fixture{
		Name:     "chromeVideoWith4DecoderThreadsAndGlobalVaapiLockDisabled",
		Desc:     "Similar to chromeVideo fixture but max decoder threads is set to 4 and disable global VA-API lock",
		Contacts: []string{"chromeos-gfx-video@google.com"},
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{
				chrome.ExtraArgs(chromeVideoArgs...),
				chrome.ExtraArgs("--disable-features=GlobalVaapiLock"),
				chrome.ExtraArgs("--disable-features=LimitConcurrentDecoderInstances"),
				chrome.ExtraArgs("--max-chromeos-decoder-threads=4"),
			}, nil
		}),
		Parent:          "gpuWatchDog",
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:     "chromeVideoWith9DecoderThreadsAndGlobalVaapiLockDisabled",
		Desc:     "Similar to chromeVideo fixture but max decoder threads is set to 9 and disable global VA-API lock",
		Contacts: []string{"chromeos-gfx-video@google.com"},
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{
				chrome.ExtraArgs(chromeVideoArgs...),
				chrome.ExtraArgs("--disable-features=GlobalVaapiLock"),
				chrome.ExtraArgs("--disable-features=LimitConcurrentDecoderInstances"),
				chrome.ExtraArgs("--max-chromeos-decoder-threads=9"),
			}, nil
		}),
		Parent:          "gpuWatchDog",
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:     "chromeVideoWith16DecoderThreadsAndGlobalVaapiLockDisabled",
		Desc:     "Similar to chromeVideo fixture but max decoder threads is set to 16 and disable global VA-API lock",
		Contacts: []string{"chromeos-gfx-video@google.com"},
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{
				chrome.ExtraArgs(chromeVideoArgs...),
				chrome.ExtraArgs("--disable-features=GlobalVaapiLock"),
				chrome.ExtraArgs("--disable-features=LimitConcurrentDecoderInstances"),
				chrome.ExtraArgs("--max-chromeos-decoder-threads=16"),
			}, nil
		}),
		Parent:          "gpuWatchDog",
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})
}

var chromeVideoArgs = []string{
	// Enable verbose log messages for video components.
	"--vmodule=" + strings.Join([]string{
		"*/media/gpu/chromeos/*=2",
		"*/media/gpu/vaapi/*=2",
		"*/media/gpu/v4l2/*=2"}, ","),
	// The Renderer video stack might have a policy of not using hardware
	// accelerated decoding for certain small resolutions (see crbug.com/684792).
	// Disable that for testing.
	"--disable-features=ResolutionBasedDecoderPriority",
	// VA-API HW decoder and encoder might reject small resolutions for
	// performance (see crbug.com/1008491 and b/171041334).
	// Disable that for testing.
	"--disable-features=VaapiEnforceVideoMinMaxResolution",
	"--disable-features=VaapiVideoMinResolutionForPerformance",
	// Allow media autoplay. <video> tag won't automatically play upon loading the source unless this flag is set.
	"--autoplay-policy=no-user-gesture-required",
	// Do not show message center notifications.
	"--suppress-message-center-popups",
	// Make sure ARC++ is not running.
	"--arc-availability=none",
	// Disable firmware update to stop chrome from executing fwupd that restarts powerd.
	"--disable-features=FirmwareUpdaterApp",
}

var chromeVideoArgsWithEnablingVaapiVEAMinResolution = []string{
	// Enable verbose log messages for video components.
	"--vmodule=" + strings.Join([]string{
		"*/media/gpu/chromeos/*=2",
		"*/media/gpu/vaapi/*=2",
		"*/media/gpu/v4l2/*=2"}, ","),
	// The Renderer video stack might have a policy of not using hardware
	// accelerated decoding for certain small resolutions (see crbug.com/684792).
	// Disable that for testing.
	"--disable-features=ResolutionBasedDecoderPriority",
	// VA-API HW decoder and encoder might reject small resolutions for
	// performance (see crbug.com/1008491 and b/171041334).
	// Disable that for testing.
	"--disable-features=VaapiEnforceVideoMinMaxResolution",
	// Allow media autoplay. <video> tag won't automatically play upon loading the source unless this flag is set.
	"--autoplay-policy=no-user-gesture-required",
	// Do not show message center notifications.
	"--suppress-message-center-popups",
	// Make sure ARC++ is not running.
	"--arc-availability=none",
	// Disable firmware update to stop chrome from executing fwupd that restarts powerd.
	"--disable-features=FirmwareUpdaterApp",
}

var chromeBypassPermissionsArgs = []string{
	// Avoid the need to grant camera/microphone permissions.
	"--use-fake-ui-for-media-stream",
}

var chromeSuppressNotificationsArgs = []string{
	// Do not show message center notifications.
	"--suppress-message-center-popups"}

var chromeFakeWebcamArgs = []string{
	// Use a fake media capture device instead of live webcam(s)/microphone(s).
	"--use-fake-device-for-media-stream",
	// Avoid the need to grant camera/microphone permissions.
	"--use-fake-ui-for-media-stream"}

var chromeAllowDistinctiveIdentifierArgs = []string{
	// Allows distinctive identifier with DRM playback when in dev mode. We don't
	// actually use RA for this, but it correlates to the same flag.
	"--allow-ra-in-dev-mode",
	// Prevents showing permission prompt and automatically grants permission to
	// allow a distinctive identifier for localhost which is where we server the
	// DRM content from in the test.
	"--unsafely-allow-protected-media-identifier-for-domain=127.0.0.1"}

var chromeWebCodecsArgs = []string{
	"--enable-blink-features=WebCodecs",
}
