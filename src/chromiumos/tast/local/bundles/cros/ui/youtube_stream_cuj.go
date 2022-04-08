// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/audio"
	"chromiumos/tast/local/audio/crastestclient"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/bundles/cros/ui/videocuj"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/power"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         YoutubeStreamCUJ,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Plays YouTube video of different quality and checks for any frame drops and if the audio is routing through expected device",
		Contacts:     []string{"ambalavanan.m.m@intel.com", "andrescj@google.com", "intel-chrome-system-automation-team@intel.com", "chromeos-gfx-video@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay(), hwdep.Speaker()),
		SoftwareDeps: []string{"chrome"},
		Fixture:      "loggedInAndKeepState",
		Params: []testing.Param{{
			Name: "1080p30",
			Val: videocuj.VideoSrc{
				URL:     "https://www.youtube.com/watch?v=Zv11L-ZfrSg",
				Title:   "Ultimate Wild Animals Collection in 8K ULTRA HD / 8K TV",
				Quality: "1080p",
			},
		}, {
			Name: "1080p60",
			Val: videocuj.VideoSrc{
				URL:     "https://www.youtube.com/watch?v=LXb3EKWsInQ",
				Title:   "COSTA RICA IN 4K 60fps HDR (ULTRA HD)",
				Quality: "1080p60",
			},
		}, {
			Name: "1440p30",
			Val: videocuj.VideoSrc{
				URL:     "https://www.youtube.com/watch?v=Zv11L-ZfrSg",
				Title:   "Ultimate Wild Animals Collection in 8K ULTRA HD / 8K TV",
				Quality: "1440p",
			},
		}, {
			Name: "1440p60",
			Val: videocuj.VideoSrc{
				URL:     "https://www.youtube.com/watch?v=LXb3EKWsInQ",
				Title:   "COSTA RICA IN 4K 60fps HDR (ULTRA HD)",
				Quality: "1440p60",
			},
		}},
	})
}

// YoutubeStreamCUJ plays YouTube video of different quality and checks for any frame drops and if the audio is routing through expected device.
func YoutubeStreamCUJ(ctx context.Context, s *testing.State) {
	// Ensure display on to record ui performance correctly.
	if err := power.TurnOnDisplay(ctx); err != nil {
		s.Fatal("Failed to turn on display: ", err)
	}
	expectedAudioNode := "INTERNAL_SPEAKER"
	var videoSource = s.Param().(videocuj.VideoSrc)

	// Give 5 seconds to cleanup other resources.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	// Setting the volume to low level.
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
	testVol := 10
	s.Logf("Setting Output node volume to %d", testVol)
	if err := vh.SetVolume(ctx, testVol); err != nil {
		s.Errorf("Failed to set output node volume to %d: %v", testVol, err)
	}
	defer vh.SetVolume(cleanupCtx, originalVolume)

	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to open the keyboard: ", err)
	}
	defer kb.Close()

	ui := uiauto.New(tconn)

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		s.Fatal("Failed to ensure clamshell mode: ", err)
	}
	defer cleanup(cleanupCtx)
	var uiHandler cuj.UIActionHandler
	if uiHandler, err = cuj.NewClamshellActionHandler(ctx, tconn); err != nil {
		s.Fatal("Failed to create clamshell action handler: ", err)
	}
	defer uiHandler.Close()

	extendedDisplay := false
	videoApp := videocuj.NewYtWeb(cr.Browser(), tconn, kb, videoSource, extendedDisplay, ui, uiHandler)

	if err := videoApp.OpenAndPlayVideo(ctx); err != nil {
		s.Fatalf("Failed to open %s: %v", videoSource.URL, err)
	}
	defer videoApp.Close(cleanupCtx)

	if err := videoApp.PerformFrameDropsTest(ctx); err != nil {
		s.Error("Failed to play video without frame drops: ", err)
	}

	deviceName, deviceType, err := cras.SelectedOutputDevice(ctx)
	if err != nil {
		s.Error("Failed to get the selected audio device: ", err)
	}
	if deviceType != expectedAudioNode {
		s.Logf("%s audio node is not selected, selecting it", expectedAudioNode)
		if err := cras.SetActiveNodeByType(ctx, expectedAudioNode); err != nil {
			s.Fatalf("Failed to select active device %s: %v", expectedAudioNode, err)
		}
		deviceName, deviceType, err = cras.SelectedOutputDevice(ctx)
		if err != nil {
			s.Error("Failed to get the selected audio device: ", err)
		}
		if deviceType != expectedAudioNode {
			s.Fatalf("Failed to set the audio node type: got %q; want %q", deviceType, expectedAudioNode)
		}
	}

	s.Logf("Selected audio device name: %s", deviceName)
	s.Logf("Selected audio device type: %s", deviceType)

	devName, err := crastestclient.FirstRunningDevice(ctx, audio.OutputStream)
	if err != nil {
		s.Fatal("Failed to detect running output device: ", err)
	}

	if deviceName != devName {
		s.Fatalf("Failed to route the audio through expected audio node: got %q; want %q", devName, deviceName)
	}
}
