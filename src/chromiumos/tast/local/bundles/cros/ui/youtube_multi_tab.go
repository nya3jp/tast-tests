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
	"chromiumos/tast/local/bundles/cros/ui/cuj/volume"
	"chromiumos/tast/local/bundles/cros/ui/videocuj"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/power"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         YoutubeMultiTab,
		Desc:         "Plays YouTube video on multiple tabs, checks for any frame drops and if the audio is routing through expected device",
		Contacts:     []string{"ambalavanan.m.m@intel.com", "intel-chrome-system-automation-team@intel.com"},
		Attr:         []string{"group:mainline", "informational"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeGraphics",
		Timeout:      10 * time.Minute,
	})
}

// YoutubeMultiTab plays YouTube video on multiple tabs, checks for any frame drops and if the audio is routing through expected device.
func YoutubeMultiTab(ctx context.Context, s *testing.State) {
	// Ensure display on to record ui performance correctly.
	if err := power.TurnOnDisplay(ctx); err != nil {
		s.Fatal("Failed to turn on display: ", err)
	}

	var videoSource = videocuj.VideoSrc{
		"https://www.youtube.com/watch?v=LXb3EKWsInQ",
		"COSTA RICA IN 4K 60fps HDR (ULTRA HD)",
		"1080p60",
	}

	// Give 5 seconds to cleanup other resources.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	// Setting the volume to low level.
	cras, err := audio.NewCras(ctx)
	if err != nil {
		s.Error("Failed to create Cras object: ", err)
	}

	vh, err := volume.NewVolumeHelper(ctx)
	if err != nil {
		s.Error("Failed to create the volumeHelper: ", err)
	}
	originalVolume, err := vh.GetVolume(ctx)
	testVol := 10
	s.Logf("Setting Output node volume to %d", testVol)
	if err := vh.SetVolume(ctx, testVol); err != nil {
		s.Errorf("Failed to set output node volume to %d: %v", testVol, err)
	}
	defer vh.SetVolume(cleanupCtx, originalVolume)

	deviceName, deviceType, err := cras.SelectedOutputDevice(ctx)
	if err != nil {
		s.Error("Failed to get the selected audio device: ", err)
	}
	expectedAudioNode := "INTERNAL_SPEAKER"
	if deviceType != expectedAudioNode {
		s.Errorf("Failed to route the audio via expected node: want %q; got %q", expectedAudioNode, deviceType)
	}

	s.Logf("Selected audio device name: %s", deviceName)
	s.Logf("Selected audio device type: %s", deviceType)

	cr := s.FixtValue().(*chrome.Chrome)
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

	var uiHandler cuj.UIActionHandler
	if uiHandler, err = cuj.NewClamshellActionHandler(ctx, tconn); err != nil {
		s.Fatal("Failed to create clamshell action handler: ", err)
	}
	defer uiHandler.Close()

	extendedDisplay := false
	tabs := 10
	videoApps := make([]*videocuj.YtWeb, 0, tabs)
	for tabIdx := 0; tabIdx < tabs; tabIdx++ {
		videoApp := videocuj.NewYtWeb(cr, tconn, kb, videoSource, extendedDisplay, ui, uiHandler)

		if err := videoApp.OpenAndPlayVideo(ctx); err != nil {
			s.Fatalf("Failed to open %s: %v", videoSource.URL, err)
		}
		// screenshot.Capture(ctx, filepath.Join(s.OutDir(), fmt.Sprintf("SS_%d.png", i)))
		videoApps = append(videoApps, videoApp)
	}
	defer func() {
		for tabIdx := 0; tabIdx < tabs; tabIdx++ {
			videoApps[tabIdx].Close(cleanupCtx)
		}
	}()

	// Switch through all tabs and check for frame drops and if audio is routing through the expected audio node.
	for tabIdx := 0; tabIdx < tabs; tabIdx++ {
		if err := uiHandler.SwitchToChromeTabByIndex(tabIdx)(ctx); err != nil {
			s.Error("Failed to switch tab")
		}

		if err := videoApps[tabIdx].PerformFrameDropsTest(ctx); err != nil {
			s.Error("Failed to play video without frame drops: ", err)
		}

		devName, err := crastestclient.FirstRunningDevice(ctx, audio.OutputStream)
		if err != nil {
			s.Fatal("Failed to detect running output device: ", err)
		}
		if deviceName != devName {
			s.Fatalf("Failed to route the audio through expected audio node: got %q; want %q", devName, deviceName)
		}
	}
}
