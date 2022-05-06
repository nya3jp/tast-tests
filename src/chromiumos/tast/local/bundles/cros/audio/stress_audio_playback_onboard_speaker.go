// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package audio

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/audio"
	"chromiumos/tast/local/audio/crastestclient"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type audioStressTestParams struct {
	stressDuration time.Duration
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         StressAudioPlaybackOnboardSpeaker,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies audio playback over onboard speaker for long duration",
		Contacts:     []string{"ambalavanan.m.m@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		HardwareDeps: hwdep.D(hwdep.Speaker()),
		Fixture:      "chromeLoggedIn",
		Params: []testing.Param{{
			Name: "bronze",
			Val: audioStressTestParams{
				stressDuration: 6 * time.Hour,
			},
			Timeout: 6*time.Hour + 10*time.Minute,
		}, {
			Name: "silver",
			Val: audioStressTestParams{
				stressDuration: 9 * time.Hour,
			},
			Timeout: 9*time.Hour + 10*time.Minute,
		}, {
			Name: "gold",
			Val: audioStressTestParams{
				stressDuration: 12 * time.Hour,
			},
			Timeout: 12*time.Hour + 10*time.Minute,
		}},
	})
}

// StressAudioPlaybackOnboardSpeaker plays audio file over onboard speaker for long duration.
func StressAudioPlaybackOnboardSpeaker(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 30*time.Second)
	defer cancel()

	cr := s.FixtValue().(*chrome.Chrome)
	testOpt := s.Param().(audioStressTestParams)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	defer func(ctx context.Context) {
		if err := closeAudioPlayer(ctx, tconn); err != nil {
			s.Error("Failed to close audio player at cleanup: ", err)
		}
	}(cleanupCtx)

	cras, err := audio.NewCras(ctx)
	if err != nil {
		s.Fatal("Failed to create Cras object: ", err)
	}

	var expectedAudioNode = "INTERNAL_SPEAKER"

	var deviceName, deviceType string
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		// Get current audio output device info.
		deviceName, deviceType, err = cras.SelectedOutputDevice(ctx)
		if err != nil {
			s.Fatal("Failed to get the selected audio device: ", err)
		}

		if deviceType != expectedAudioNode {
			if err := cras.SetActiveNodeByType(ctx, expectedAudioNode); err != nil {
				return errors.Wrapf(err, "failed to select active device %s", expectedAudioNode)
			}
			deviceName, deviceType, err = cras.SelectedOutputDevice(ctx)
			if err != nil {
				return errors.Wrap(err, "failed to get the selected audio device")
			}
			if deviceType != expectedAudioNode {
				return errors.Wrapf(err, "failed to set the audio node type: got %q; want %q", deviceType, expectedAudioNode)
			}
		}
		return nil
	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		s.Fatal("Failed to set audio node: ", err)
	}

	// Generate sine raw input file that lasts 1 hour.
	rawFileName := "AudioFile.raw"
	rawFilePath := filepath.Join(filesapp.DownloadPath, rawFileName)
	rawFile := audio.TestRawData{
		Path:          rawFilePath,
		BitsPerSample: 16,
		Channels:      2,
		Rate:          48000,
		Frequencies:   []int{440, 440},
		Volume:        0.05,
		Duration:      3600,
	}

	if err := audio.GenerateTestRawData(ctx, rawFile); err != nil {
		s.Fatal("Failed to generate audio test data: ", err)
	}
	defer os.Remove(rawFile.Path)

	wavFileName := "AudioFile.wav"
	wavFile := filepath.Join(filesapp.DownloadPath, wavFileName)
	if err := audio.ConvertRawToWav(ctx, rawFilePath, wavFile, 48000, 2); err != nil {
		s.Fatal("Failed to convert raw to wav: ", err)
	}
	defer os.Remove(wavFile)

	// Total stress test duration in seconds.
	durationInSecs := int(testOpt.stressDuration / time.Second)
	// Converting seconds to hour.
	iterHour := durationInSecs / 3600
	for i := 1; i <= iterHour; i++ {
		files, err := filesapp.Launch(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to launch the Files App: ", err)
		}
		defer files.Close(cleanupCtx)

		if err := files.OpenDownloads()(ctx); err != nil {
			s.Fatal("Failed to open Downloads folder in files app: ", err)
		}
		if err := files.OpenFile(wavFileName)(ctx); err != nil {
			s.Fatalf("Failed to open the audio file %q: %v", wavFileName, err)
		}

		endTime := time.Now().Add(time.Hour)
		s.Logf("Checking audio routing, test remaining time of %d/%d hour", i, iterHour)
		// Verify whether audio is routing through onboard-speaker or not.
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			devName, err := crastestclient.FirstRunningDevice(ctx, audio.OutputStream)
			if err != nil {
				return errors.Wrap(err, "failed to detect running output device")
			}
			if deviceName != devName {
				return errors.Wrapf(err, "unexpected audio node: got %q; want %q", devName, deviceName)
			}
			timeNow := time.Now()
			if timeNow.Before(endTime) {
				s.Logf("Audio is routing to %s, test remaining time: %v", expectedAudioNode, endTime.Sub(timeNow))
				return errors.New("audio is routing")
			}
			return nil
		}, &testing.PollOptions{Timeout: time.Hour + 10*time.Minute, Interval: 2 * time.Minute}); err != nil {
			s.Fatal("Timeout waiting for audio route on onboard-speaker: ", err)
		}

		// Close audio player window.
		if err := closeAudioPlayer(ctx, tconn); err != nil {
			s.Fatal("Failed to close audio player: ", err)
		}
	}
}

// closeAudioPlayer performs closing of audio player window.
func closeAudioPlayer(ctx context.Context, tconn *chrome.TestConn) error {
	ui := uiauto.New(tconn)
	nowPlayingText := nodewith.Name("Now playing").Role(role.StaticText)
	if err := ui.Exists(nowPlayingText)(ctx); err == nil {
		audioPlayerTitle := "Gallery - AudioFile.wav"
		audioPlayerWindow, err := ash.BringWindowToForeground(ctx, tconn, audioPlayerTitle)
		if err != nil {
			return errors.Wrap(err, "failed to bring audio player to foreground")
		}
		if err := audioPlayerWindow.CloseWindow(ctx, tconn); err != nil {
			return errors.Wrap(err, "failed to close autio player window")
		}
	}
	return nil
}
