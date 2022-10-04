// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/audio"
	"chromiumos/tast/local/audio/crastestclient"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/cuj"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/mtbf/youtube"
	"chromiumos/tast/local/power"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         YoutubeMultiTab,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Plays YouTube video on multiple tabs concurrently, checks for significant frame drops and if the audio is being routed through expected device",
		Contacts:     []string{"ambalavanan.m.m@intel.com", "andrescj@google.com", "intel-chrome-system-automation-team@intel.com", "chromeos-gfx-video@google.com"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay(), hwdep.Speaker()),
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeGraphics",
		Timeout:      10 * time.Minute,
	})
}

// YoutubeMultiTab plays YouTube video on multiple tabs concurrently, checks for significant frame drops and if the audio is being routed through expected device.
func YoutubeMultiTab(ctx context.Context, s *testing.State) {
	// Ensure display on to record ui performance correctly.
	if err := power.TurnOnDisplay(ctx); err != nil {
		s.Fatal("Failed to turn on display: ", err)
	}

	var videoSource = youtube.VideoSrc{
		URL:     "https://www.youtube.com/watch?v=LXb3EKWsInQ",
		Title:   "COSTA RICA IN 4K 60fps HDR (ULTRA HD)",
		Quality: "1080p60",
	}

	// Give 5 seconds to cleanup other resources.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	// Setting the volume to low level.
	cras, err := audio.NewCras(ctx)
	if err != nil {
		s.Fatal("Failed to create Cras object: ", err)
	}

	vh, err := audio.NewVolumeHelper(ctx)
	if err != nil {
		s.Fatal("Failed to create the volumeHelper: ", err)
	}
	originalVolume, err := vh.GetVolume(ctx)
	if err != nil {
		s.Fatal("Failed to get volume: ", err)
	}
	testVol := 10
	s.Logf("Setting Output node volume to %d", testVol)
	if err := vh.SetVolume(ctx, testVol); err != nil {
		s.Fatalf("Failed to set output node volume to %d: %v", testVol, err)
	}
	defer vh.SetVolume(cleanupCtx, originalVolume)

	expectedAudioNode := "INTERNAL_SPEAKER"
	// Setting the active node to INTERNAL_SPEAKER if default node is set to some other node.
	if err := cras.SetActiveNodeByType(ctx, expectedAudioNode); err != nil {
		s.Fatalf("Failed to select active device %s: %v", expectedAudioNode, err)
	}
	wantDevName, wantDevType, err := cras.SelectedOutputDevice(ctx)
	if err != nil {
		s.Fatal("Failed to get the selected audio device: ", err)
	}
	if wantDevType != expectedAudioNode {
		s.Fatalf("Failed to set the audio node type: got %q; want %q", wantDevType, expectedAudioNode)
	}

	cr := s.FixtValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	kb, err := input.VirtualKeyboard(ctx)
	if err != nil {
		s.Fatal("Failed to open the keyboard: ", err)
	}
	defer kb.Close()

	ui := uiauto.New(tconn)

	var uiHandler cuj.UIActionHandler
	if uiHandler, err = cuj.NewClamshellActionHandler(ctx, tconn); err != nil {
		s.Fatal("Failed to create clamshell action handler: ", err)
	}
	defer uiHandler.Close()

	extendedDisplay := false
	var videoApps []*youtube.YtWeb
	for tabIdx := 0; tabIdx < 10; tabIdx++ {
		videoApp := youtube.NewYtWeb(cr.Browser(), tconn, kb, extendedDisplay, ui, uiHandler)
		defer videoApp.Close(cleanupCtx)
		if err := videoApp.OpenAndPlayVideo(videoSource)(ctx); err != nil {
			s.Fatalf("Failed to open %s: %v", videoSource.URL, err)
		}
		videoApps = append(videoApps, videoApp)
	}

	// Switch through all tabs and check for frame drops and if audio is routing through the expected audio node.
	for tabIdx := 0; tabIdx < 10; tabIdx++ {
		if err := uiHandler.SwitchToChromeTabByIndex(tabIdx)(ctx); err != nil {
			s.Fatal("Failed to switch tab: ", err)
		}

		if err := videoApps[tabIdx].PerformFrameDropsTest(ctx); err != nil {
			s.Fatal("Failed to play video without frame drops: ", err)
		}

		devName, err := crastestclient.FirstRunningDevice(ctx, audio.OutputStream)
		if err != nil {
			s.Fatal("Failed to detect running output device: ", err)
		}
		if wantDevName != devName {
			s.Fatalf("Failed to route the audio through expected audio node: got %q; want %q", devName, wantDevName)
		}
	}
}
