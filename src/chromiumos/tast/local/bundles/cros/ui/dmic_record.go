// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/audio"
	"chromiumos/tast/local/audio/crastestclient"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/bundles/cros/ui/videocuj"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DMICRecord,
		Desc:         "DMIC Record via commands",
		Contacts:     []string{"ambalavanan.m.m@intel.com", "intel-chrome-system-automation-team@intel.com"},
		HardwareDeps: hwdep.D(hwdep.Microphone(), hwdep.Speaker()),
		Fixture:      "chromeLoggedIn",
	})
}

func DMICRecord(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}
	kb, err := input.VirtualKeyboard(ctx)
	if err != nil {
		s.Fatal("Failed to open the keyboard: ", err)
	}
	defer kb.Close()

	const (
		recordTime        = 30
		audioRate         = 48000
		recWavFileName    = "audio.wav"
		extendedDisplay   = false
		audioChannel      = 2
		cmdTimeout        = time.Minute
		expectedAudioNode = "INTERNAL_SPEAKER"
	)

	uiHandler, err := cuj.NewClamshellActionHandler(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to create clamshell action handler: ", err)
	}
	defer uiHandler.Close()

	// Shorten deadline to leave time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	var videoSource = videocuj.VideoSrc{
		URL:     "https://www.youtube.com/watch?v=LXb3EKWsInQ",
		Title:   "COSTA RICA IN 4K 60fps HDR (ULTRA HD)",
		Quality: "1080p60",
	}

	ui := uiauto.New(tconn)
	// Create an instance of YtWeb to perform actions on youtube web.
	ytbWeb := videocuj.NewYtWeb(cr.Browser(), tconn, kb, videoSource, extendedDisplay, ui, uiHandler)
	defer ytbWeb.Close(cleanupCtx)

	if err := ytbWeb.OpenAndPlayVideo(ctx); err != nil {
		s.Fatalf("Failed to open %s: %v", videoSource.URL, err)
	}

	if err := ytbWeb.Play()(ctx); err != nil {
		s.Fatal("Failed to play the video: ", err)
	}

	cras, err := audio.NewCras(ctx)
	if err != nil {
		s.Fatal("Failed to create Cras object: ", err)
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

	args := []string{"-r", strconv.Itoa(audioRate), "-c", strconv.Itoa(audioChannel),
		filepath.Join(filesapp.DownloadPath, recWavFileName), "trim", "0", "30"}

	cmd := testexec.CommandContext(ctx, "rec", args...)
	s.Logf("Recording audio using: %s", cmd)
	if err := cmd.Run(testexec.DumpLogOnError); err != nil {
		s.Fatalf("Failed to execute %q: %v", cmd, err)
	}

	files, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to launch the Files App: ", err)
	}
	defer func(ctx context.Context) {
		files.Close(ctx)
		if err := os.Remove(filepath.Join(filesapp.DownloadPath, recWavFileName)); err != nil {
			s.Errorf("Failed to delete file %q: %v", recWavFileName, err)
		}
		if err = kb.Accel(ctx, "Ctrl+W"); err != nil {
			s.Error("Failed to close Audio player: ", err)
		}
	}(ctx)

	if err := files.OpenDownloads()(ctx); err != nil {
		s.Fatal("Failed to open Downloads folder in files app: ", err)
	}
	if err := files.OpenFile(recWavFileName)(ctx); err != nil {
		s.Fatalf("Failed to open the audio file %q: %v", recWavFileName, err)
	}

	// Sample time for the audio to play for 5 seconds.
	if err := testing.Sleep(ctx, 5*time.Second); err != nil {
		s.Fatal("Failed to sleep: ", err)
	}
	devName, err = crastestclient.FirstRunningDevice(ctx, audio.OutputStream)
	if err != nil {
		s.Fatal("Failed to detect running output device: ", err)
	}
	if deviceName != devName {
		s.Fatalf("Failed to route the audio through expected audio node: got %q; want %q", devName, deviceName)
	}
}
