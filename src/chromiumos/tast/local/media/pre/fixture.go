// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package pre

import (
	"context"
	"io/ioutil"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/crash"
	"chromiumos/tast/local/graphics"
	"chromiumos/tast/local/syslog"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name: "chromeVideo",
		Desc: "Logged into a user session with logging enabled",
		Impl: chrome.NewFixture(
			chromeVModuleArgs,
			chromeUseHWCodecsForSmallResolutions,
			chromeBypassPermissionsArgs,
			chromeSuppressNotificationsArgs,
		),
		Parent:          "gpuMonitor",
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	// Chrome has two said implementations: a "legacy" one and a Direct, VD-based on. Selecting one ore the other depends on the hardware and is ultimately determined by the overlays/ flags. Tests should be centered on what the users see, hence most of the testing should use chromeVideo, with a few test cases using this fixture.
	testing.AddFixture(&testing.Fixture{
		Name: "chromeAlternateVideoDecoder",
		Desc: "Logged into a user session with alternate hardware accelerated video decoder.",
		Impl: chrome.NewFixture(
			chromeVModuleArgs,
			chromeUseHWCodecsForSmallResolutions,
			chromeSuppressNotificationsArgs,
			chrome.EnableFeatures("UseAlternateVideoDecoderImplementation"),
		),
		Parent:          "gpuMonitor",
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name: "chromeVideoWithGuestLogin",
		Desc: "Similar to chromeVideo fixture but forcing login as a guest.",
		Impl: chrome.NewFixture(
			chromeVModuleArgs,
			chromeUseHWCodecsForSmallResolutions,
			chromeSuppressNotificationsArgs,
			chrome.GuestLogin(),
		),
		Parent:          "gpuMonitor",
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	// TODO(crbug.com/958166): Use simply ChromeVideo() when HDR is launched.
	testing.AddFixture(&testing.Fixture{
		Name: "chromeVideoWithHDRScreen",
		Desc: "Similar to chromeVideo fixture but enabling the HDR screen if present.",
		Impl: chrome.NewFixture(
			chromeVModuleArgs,
			chromeUseHWCodecsForSmallResolutions,
			chromeSuppressNotificationsArgs,
			chrome.EnableFeatures("UseHDRTransferFunction"),
		),
		Parent:          "gpuMonitor",
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name: "chromeCompositedVideo",
		Desc: "Similar to chromeVideo fixture but disabling hardware overlays entirely to force video to be composited.",
		Impl: chrome.NewFixture(
			chromeVModuleArgs,
			chromeUseHWCodecsForSmallResolutions,
			chromeSuppressNotificationsArgs,
			chrome.ExtraArgs("--enable-hardware-overlays=\"\""),
		),
		Parent:          "gpuMonitor",
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name: "chromeVideoWithFakeWebcam",
		Desc: "Similar to chromeVideo fixture but supplementing it with the use of a fake video/audio capture device (a.k.a. 'fake webcam'), see https://webrtc.org/testing/.",
		Impl: chrome.NewFixture(
			chromeVModuleArgs,
			chromeUseHWCodecsForSmallResolutions,
			chromeSuppressNotificationsArgs,
			chromeFakeWebcamArgs,
		),
		Parent:          "gpuMonitor",
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name: "chromeVideoWithFakeWebcamAndAlternateVideoDecoder",
		Desc: "Similar to chromeVideoWithFakeWebcam fixture but using the alternative video decoder.",
		Impl: chrome.NewFixture(
			chromeVModuleArgs,
			chromeUseHWCodecsForSmallResolutions,
			chromeSuppressNotificationsArgs,
			chromeFakeWebcamArgs,
			chrome.EnableFeatures("UseAlternateVideoDecoderImplementation"),
		),
		Parent:          "gpuMonitor",
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name: "chromeVideoWithFakeWebcamAndForceVP9ThreeTemporalLayers",
		Desc: "Similar to chromeVideoWithFakeWebcam fixture but forcing webrtc vp9 stream to be three temporal layers..",
		Impl: chrome.NewFixture(
			chromeVModuleArgs,
			chromeSuppressNotificationsArgs,
			chromeFakeWebcamArgs,
			chrome.ExtraArgs("--force-fieldtrials=WebRTC-SupportVP9SVC/EnabledByFlag_1SL3TL/"),
		),
		Parent:          "gpuMonitor",
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name: "chromeVideoWithFakeWebcamAndSWDecoding",
		Desc: "Similar to chromeVideoWithFakeWebcam fixture but hardware decoding disabled.",
		Impl: chrome.NewFixture(
			chromeVModuleArgs,
			chromeSuppressNotificationsArgs,
			chromeFakeWebcamArgs,
			chrome.ExtraArgs("--disable-accelerated-video-decode"),
		),
		Parent:          "gpuMonitor",
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name: "chromeVideoWithFakeWebcamAndSWEncoding",
		Desc: "Similar to chromeVideoWithFakeWebcam fixture but hardware encoding disabled.",
		Impl: chrome.NewFixture(
			chromeVModuleArgs,
			chromeSuppressNotificationsArgs,
			chromeFakeWebcamArgs,
			chrome.ExtraArgs("--disable-accelerated-video-encode"),
		),
		Parent:          "gpuMonitor",
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name: "chromeScreenCapture",
		Desc: "Logged into a user session with flag so that Chrome always picks the entire screen for getDisplayMedia(), bypassing the picker UI.",
		Impl: chrome.NewFixture(
			chromeSuppressNotificationsArgs,
			chrome.ExtraArgs(`--auto-select-desktop-capture-source=display`),
		),
		Parent:          "gpuMonitor",
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name: "chromeWindowCapture",
		Desc: "Logged into a user session with flag so that Chrome always picks the Chromium window for getDisplayMedia(), bypassing the picker UI.",
		Impl: chrome.NewFixture(
			chromeSuppressNotificationsArgs,
			chrome.ExtraArgs(`--auto-select-desktop-capture-source=Chrome`),
		),
		Parent:          "gpuMonitor",
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name: "chromeVideoWithSWDecoding",
		Desc: "Similar to chromeVideo fixture but making sure Chrome does not use any potential hardware accelerated decoding.",
		Impl: chrome.NewFixture(
			chromeSuppressNotificationsArgs,
			chrome.ExtraArgs("--disable-accelerated-video-decode"),
		),
		Parent:          "gpuMonitor",
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name: "chromeVideoWithSWDecodingAndLibGAV1",
		Desc: "Similar to chromeVideoWithSWDecoding fixture but enabling the use of LibGAV1 for AV1 decoding.",
		Impl: chrome.NewFixture(
			chromeVModuleArgs,
			chromeSuppressNotificationsArgs,
			chrome.ExtraArgs("--disable-accelerated-video-decode", "--enable-features=Gav1VideoDecoder"),
		),
		Parent:          "gpuMonitor",
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	// TODO(b/172217032): Remove these *HWAV1Decoding preconditions once the hardware av1 decoder feature is enabled by default.
	testing.AddFixture(&testing.Fixture{
		Name: "chromeVideoWithHWAV1Decoding",
		Desc: "Similar to chromeVideo fixture but also enables hardware accelerated av1 decoding.",
		Impl: chrome.NewFixture(
			chromeVModuleArgs,
			chromeUseHWCodecsForSmallResolutions,
			chromeSuppressNotificationsArgs,
			chrome.ExtraArgs("--enable-features=VaapiAV1Decoder"),
		),
		Parent:          "gpuMonitor",
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name: "chromeVideoWithGuestLoginAndHWAV1Decoding",
		Desc: "Similar to chromeVideoWithGuestLogin fixture but also enables hardware accelerated av1 decoding.",
		Impl: chrome.NewFixture(
			chromeVModuleArgs,
			chromeUseHWCodecsForSmallResolutions,
			chromeSuppressNotificationsArgs,
			chrome.GuestLogin(),
			chrome.ExtraArgs("--enable-features=VaapiAV1Decoder"),
		),
		Parent:          "gpuMonitor",
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	// TODO(crbug.com/958166): Use simply ChromeVideoWithSWDecoding() when HDR is launched.
	testing.AddFixture(&testing.Fixture{
		Name: "chromeVideoWithSWDecodingAndHDRScreen",
		Desc: "Similar to chromeVideoWithSWDecoding but also enalbing the HDR screen if present.",
		Impl: chrome.NewFixture(
			chromeVModuleArgs,
			chromeSuppressNotificationsArgs,
			chrome.ExtraArgs("--disable-accelerated-video-decode"),
			chrome.EnableFeatures("UseHDRTransferFunction"),
		),
		Parent:          "gpuMonitor",
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name: "chromeCameraPerf",
		Desc: "Logged into a user session with camera tests-specific setting and without verbose logging that can affect the performance. This fixture should be used only for performance tests.",
		Impl: chrome.NewFixture(
			chromeBypassPermissionsArgs,
			chromeSuppressNotificationsArgs,
		),
		Parent:          "gpuMonitor",
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name: "chromeFakeCameraPerf",
		Desc: "Logged into a user session with fake video/audio capture device (a.k.a. 'fake webcam', see https://webrtc.org/testing), without asking for user permission, and without verboselogging that can affect the performance. This fixture should be used only used for performance tests.",
		Impl: chrome.NewFixture(
			chromeFakeWebcamArgs,
			chromeSuppressNotificationsArgs,
		),
		Parent:          "gpuMonitor",
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:            "gpuMonitor",
		Desc:            "Check if there's any gpu crash file generated during the test.",
		Impl:            &gpuMonitorFixture{},
		PreTestTimeout:  5 * time.Second,
		PostTestTimeout: 5 * time.Second,
	})
}

type gpuMonitorFixture struct {
	hangPatterns []string
	postFunc     []func(ctx context.Context) error
}

func (f *gpuMonitorFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	f.hangPatterns = []string{
		"drm:i915_hangcheck_elapsed",
		"drm:i915_hangcheck_hung",
		"Hangcheck timer elapsed...",
		"drm/i915: Resetting chip after gpu hang",
	}
	return nil
}

func (f *gpuMonitorFixture) TearDown(ctx context.Context, s *testing.FixtState) {}

func (f *gpuMonitorFixture) Reset(ctx context.Context) error {
	return nil
}

func (f *gpuMonitorFixture) getGPUCrash() ([]string, error) {
	crashFiles, err := crash.GetCrashes(crash.DefaultDirs()...)
	if err != nil {
		return nil, err
	}
	// Filter the gpu related crash.
	var crashes []string
	for _, file := range crashFiles {
		if strings.HasSuffix(file, crash.GPUStateExt) {
			crashes = append(crashes, file)
		}
	}
	return crashes, nil
}

// checkNewCrashes checkes the difference between the oldCrashes and the current crashes. Return error if failed to retrieve current crashes or the list is mismatch.
func (f *gpuMonitorFixture) checkNewCrashes(ctx context.Context, oldCrashes []string) error {
	crashes, err := f.getGPUCrash()
	if err != nil {
		return err
	}

	// Check if there're new crash files got generated during the test.
	for _, crash := range crashes {
		found := false
		for _, preTestCrash := range oldCrashes {
			if preTestCrash == crash {
				found = true
				break
			}
		}
		if !found {
			return errors.Errorf("found gpu crash file: %s", crash)
		}
	}
	return nil
}

// checkNewHangs checks the oldHangLine with the current hangs in syslog.MessageFile. It returns error if failed to read the file or the lines are mismatch.
func (f *gpuMonitorFixture) checkNewHangs(ctx context.Context, oldHangLines map[string]bool) error {
	out, err := ioutil.ReadFile(syslog.MessageFile)
	if err != nil {
		return err
	}

	for _, line := range strings.Split(string(out), "\n") {
		for _, pattern := range f.hangPatterns {
			if strings.Contains(line, pattern) {
				if _, ok := oldHangLines[line]; !ok {
					return errors.New("detect gpu hang during test")
				}
			}
		}
	}
	return nil
}

func (f *gpuMonitorFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {
	f.postFunc = nil
	// Attempt flushing system logs every second instead of every 10 minutes.
	dirtyWritebackDuration, err := graphics.GetDirtyWritebackDuration()
	if err != nil {
		s.Log("Failed to set get dirty writeback duration: ", err)
	} else {
		if err := graphics.SetDirtyWritebackDuration(ctx, 1*time.Second); err != nil {
			f.postFunc = append(f.postFunc, func(ctx context.Context) error {
				s.Log("set back dirty writeback")
				return graphics.SetDirtyWritebackDuration(ctx, dirtyWritebackDuration)
			})
		}
	}

	// Record PreTest crashes.
	crashes, err := f.getGPUCrash()
	if err != nil {
		s.Log("Failed to get gpu crashes: ", err)
	} else {
		f.postFunc = append(f.postFunc, func(ctx context.Context) error {
			return f.checkNewCrashes(ctx, crashes)
		})
	}

	// Record PreTest GPU hangs.
	out, err := ioutil.ReadFile(syslog.MessageFile)
	if err != nil {
		s.Log("Failed to read message file: ", err)
	} else {
		hangLine := make(map[string]bool)
		for _, line := range strings.Split(string(out), "\n") {
			for _, pattern := range f.hangPatterns {
				if strings.Contains(line, pattern) {
					hangLine[line] = true
				}
			}
		}
		f.postFunc = append(f.postFunc, func(ctx context.Context) error {
			return f.checkNewHangs(ctx, hangLine)
		})
	}
}

func (f *gpuMonitorFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {
	var postErr error
	for i := len(f.postFunc) - 1; i >= 0; i-- {
		if err := f.postFunc[i](ctx); err != nil {
			postErr = errors.Wrap(postErr, err.Error())
		}
	}
	if postErr != nil {
		s.Error("PostTest failed: ", postErr)
	}
}

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
	// VA-API HW decoder and encoder might reject small resolutions for
	// performance (see crbug.com/1008491 and b/171041334).
	// Disable that for testing.
	"--disable-features=VaapiEnforceVideoMinMaxResolution")

var chromeFakeWebcamArgs = chrome.ExtraArgs(
	// Use a fake media capture device instead of live webcam(s)/microphone(s).
	"--use-fake-device-for-media-stream",
	// Avoid the need to grant camera/microphone permissions.
	"--use-fake-ui-for-media-stream")

var chromeBypassPermissionsArgs = chrome.ExtraArgs(
	// Avoid the need to grant camera/microphone permissions.
	"--use-fake-ui-for-media-stream")

var chromeSuppressNotificationsArgs = chrome.ExtraArgs(
	// Do not show message center notifications.
	"--suppress-message-center-popups")
