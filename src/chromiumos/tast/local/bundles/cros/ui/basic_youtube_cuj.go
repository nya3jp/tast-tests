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
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         BasicYoutubeCUJ,
		Desc:         "Basic YouTube functionality check",
		Contacts:     []string{"ambalavanan.m.m@intel.com", "intel-chrome-system-automation-team@intel.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "loggedInAndKeepState",
	})
}

// BasicYoutubeCUJ plays YouTube video, performs basic user actions. Also checks for any frame drops and if the audio is routing through expected device.
func BasicYoutubeCUJ(ctx context.Context, s *testing.State) {
	expectedAudioNode := "INTERNAL_SPEAKER"
	var videoSource = videocuj.VideoSrc{
		"https://www.youtube.com/watch?v=LXb3EKWsInQ",
		"COSTA RICA IN 4K 60fps HDR (ULTRA HD)",
		"1440p60",
	}
	extendedDisplay := false
	cr := s.FixtValue().(cuj.FixtureData).Chrome

	// Give 5 seconds to cleanup other resources.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	// Mute the device to avoid noisiness.
	if err := crastestclient.Mute(ctx); err != nil {
		s.Fatal("Failed to mute: ", err)
	}
	defer crastestclient.Unmute(cleanupCtx)

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

	videoApp := videocuj.NewYtWeb(cr, tconn, kb, videoSource, extendedDisplay, ui, uiHandler)

	if err := videoApp.OpenAndPlayVideo(ctx); err != nil {
		s.Fatalf("Failed to open %s", videoSource)
	}
	defer videoApp.Close(cleanupCtx)

	if err := videoApp.PerformFrameDropsTest(ctx); err != nil {
		s.Error("Failed to play video without frame drops: ", err)
	}

	if err := videoApp.EnterFullscreen(ctx); err != nil {
		s.Fatalf("Failed to open %q", videoSource.Url)
	}

	if err := videoApp.PauseAndPlayVideo(ctx); err != nil {
		s.Error("Failed to pause and play video: ", err)
	}

	if err := videoApp.RestoreWindow(ctx); err != nil {
		s.Error("Failed to restore Youtube window: ", err)
	}

	if err := videoApp.MaximizeWindow(ctx); err != nil {
		s.Error("Failed to maximize Youtube window: ", err)
	}

	if err := videoApp.MinimizeWindow(ctx); err != nil {
		s.Error("Failed to minimize Youtube window: ", err)
	}

	cras, err := audio.NewCras(ctx)
	if err != nil {
		s.Error("Failed to create Cras object: ", err)
	}
	deviceName, deviceType, err := cras.SelectedOutputDevice(ctx)
	if err != nil {
		s.Error("Failed to get the selected audio device: ", err)
	}
	if deviceType != expectedAudioNode {
		s.Errorf("Failed to route the audio via expected node: want %q; got %q", expectedAudioNode, deviceType)
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
