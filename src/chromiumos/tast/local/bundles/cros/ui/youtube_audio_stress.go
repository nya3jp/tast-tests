// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/audio"
	"chromiumos/tast/local/audio/crastestclient"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/bundles/cros/ui/videocuj"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/power"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type videoDuration struct {
	minutes int
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         YoutubeAudioStress,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Plays YouTube video streaming and checks if the audio is being routed through onboard speaker",
		Contacts: []string{
			"ambalavanan.m.m@intel.com",
			"andrescj@google.com",
			"intel-chrome-system-automation-team@intel.com",
			"chromeos-gfx-video@google.com",
		},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay(), hwdep.Speaker()),
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name:      "quick",
			Val:       videoDuration{minutes: 5},
			Timeout:   10 * time.Minute,
			Fixture:   "chromeLoggedIn",
			ExtraAttr: []string{"group:mainline", "informational"},
		}, {
			Name:    "bronze",
			Val:     videoDuration{minutes: 6 * 60}, // 6 hours.
			Timeout: 375 * time.Minute,
			Fixture: "chromeLoggedIn",
		}, {
			Name:    "silver",
			Val:     videoDuration{minutes: 9 * 60}, // 9 hours.
			Timeout: 555 * time.Minute,
			Fixture: "chromeLoggedIn",
		}, {
			Name:    "gold",
			Val:     videoDuration{minutes: 12 * 60}, // 12 hours.
			Timeout: 735 * time.Minute,
			Fixture: "chromeLoggedIn",
		},
		}})
}

// YoutubeAudioStress plays youtube video for long duration and verify audio routing
// through onboard speaker.
func YoutubeAudioStress(ctx context.Context, s *testing.State) {
	// Ensure display on to record ui performance correctly.
	if err := power.TurnOnDisplay(ctx); err != nil {
		s.Fatal("Failed to turn on display: ", err)
	}

	duration := s.Param().(videoDuration)
	cr := s.FixtValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	const (
		expectedAudioNode = "INTERNAL_SPEAKER"
		extendedDisplay   = false
		playingState      = 1 // Playing state of the Youtube player.
	)

	// Shorten deadline to leave time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	cras, err := audio.NewCras(ctx)
	if err != nil {
		s.Error("Failed to create Cras object: ", err)
	}

	vh, err := audio.NewVolumeHelper(ctx)
	if err != nil {
		s.Error("Failed to create the volumeHelper: ", err)
	}
	originalVolume, err := vh.GetVolume(ctx)
	if err != nil {
		s.Error("Failed to get volume: ", err)
	}
	// Setting the volume to low level.
	const testVol = 10
	if err := vh.SetVolume(ctx, testVol); err != nil {
		s.Errorf("Failed to set output node volume to %d: %v", testVol, err)
	}

	//Set back to original volume.
	defer vh.SetVolume(cleanupCtx, originalVolume)

	kb, err := input.VirtualKeyboard(ctx)
	if err != nil {
		s.Fatal("Failed to open the keyboard: ", err)
	}
	defer kb.Close()

	ui := uiauto.New(tconn)

	uiHandler, err := cuj.NewClamshellActionHandler(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to create clamshell action handler: ", err)
	}
	defer uiHandler.Close()

	var videoSource = videocuj.VideoSrc{
		URL:     "https://www.youtube.com/watch?v=g3PWYH1P7Oo",
		Title:   "12 Hours Of Tropical Coral Reef Fishes At Monterey Bay Aquarium | Littoral Relaxocean - YouTube",
		Quality: "1080p60",
	}
	// Create an instance of YtWeb to perform actions on youtube web.
	ytbWeb := videocuj.NewYtWeb(cr.Browser(), tconn, kb, videoSource, extendedDisplay, ui, uiHandler)
	defer ytbWeb.Close(cleanupCtx)

	if err := ytbWeb.OpenAndPlayVideo(ctx); err != nil {
		s.Fatalf("Failed to open %s: %v", videoSource.URL, err)
	}

	if err := ytbWeb.PerformFrameDropsTest(ctx); err != nil {
		s.Error("Failed to play video without frame drops: ", err)
	}

	// Setting the active node to INTERNAL_SPEAKER if default node is set to some other node.
	if err := cras.SetActiveNodeByType(ctx, expectedAudioNode); err != nil {
		s.Fatalf("Failed to select active device %q: %v", expectedAudioNode, err)
	}
	deviceName, deviceType, err := cras.SelectedOutputDevice(ctx)
	if err != nil {
		s.Fatal("Failed to get the selected audio device: ", err)
	}
	if deviceType != expectedAudioNode {
		s.Fatalf("Failed to set the audio node type: got %q; want %q", deviceType, expectedAudioNode)
	}

	devName, err := crastestclient.FirstRunningDevice(ctx, audio.OutputStream)
	if err != nil {
		s.Fatal("Failed to detect running output device: ", err)
	}

	if deviceName != devName {
		s.Fatalf("Failed to route the audio through expected audio node: got %q; want %q", devName, deviceName)
	}

	// videoPlaying verifies whether youtube video is playing or not.
	videoPlaying := func() error {
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			var playerState int
			if err := ytbWeb.YtWebConn().Eval(ctx, `document.getElementById('movie_player').getPlayerState()`, &playerState); err != nil {
				return errors.Wrap(err, "failed to get youtube player state")
			}
			if playerState != playingState {
				return errors.New("youtube video is not playing")
			}
			s.Log("Youtube video is playing")
			return nil
		}, &testing.PollOptions{Timeout: 40 * time.Second, Interval: 2 * time.Second}); err != nil {
			return err
		}
		return nil
	}

	startTime := time.Now().Unix()
	endTime := float64(duration.minutes * 60)

	if err := testing.Poll(ctx, func(c context.Context) error {
		if err := videoPlaying(); err != nil {
			return errors.Wrap(err, "failed to play video")
		}
		devName, err = crastestclient.FirstRunningDevice(ctx, audio.OutputStream)
		if err != nil {
			return errors.Wrap(err, "failed to detect running output device")
		}
		if deviceName != devName {
			return errors.Errorf("failed to route the audio through expected audio node: got %q; want %q", devName, deviceName)
		}
		elapsed := float64(time.Now().Unix() - startTime)
		if elapsed < endTime {
			s.Logf("Audio is routing to %s, test remaining time: %f/%f sec", expectedAudioNode, elapsed, endTime)
			return errors.New("audio is routing")
		}
		return nil
	}, &testing.PollOptions{Interval: 2 * time.Minute, Timeout: time.Duration(duration.minutes+10) * time.Minute}); err != nil {
		s.Fatalf("Failed to play Youtube through %s device: %v", expectedAudioNode, err)
	}
}
