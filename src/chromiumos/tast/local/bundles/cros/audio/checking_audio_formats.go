// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package audio

import (
	"context"
	"os"
	"path"
	"regexp"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/audio"
	"chromiumos/tast/local/audio/crastestclient"
	"chromiumos/tast/local/bundles/cros/audio/audionode"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CheckingAudioFormats,
		Desc:         "Verifies supported audio file formats",
		Contacts:     []string{"pathan.jilani@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		HardwareDeps: hwdep.D(hwdep.Speaker()),
		Data:         []string{"audio.flac", "audio.m4a", "audio.ogg", "audio.wav"},
		Fixture:      "chromeLoggedIn",
	})
}

func CheckingAudioFormats(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	const (
		expectedAudioNode = "INTERNAL_SPEAKER"
		downloadsPath     = filesapp.DownloadPath
	)

	audioFiles := []string{"audio.flac", "audio.m4a", "audio.ogg", "audio.wav"}
	audioFileRe := regexp.MustCompile(`^audio.(wav|m4a|ogg|flac)$`)

	for _, file := range audioFiles {
		if err := fsutil.CopyFile(s.DataPath(file), path.Join(downloadsPath, file)); err != nil {
			s.Fatalf("Failed to copy %q file to %q: %v", file, downloadsPath, err)
		}
	}

	defer func(context.Context) {
		for _, file := range audioFiles {
			if err := os.Remove(path.Join(downloadsPath, file)); err != nil {
				s.Fatalf("Failed to remove %q file: %v", file, err)
			}
		}
	}(cleanupCtx)

	kb, err := input.VirtualKeyboard(ctx)
	if err != nil {
		s.Fatal("Failed to create keyboard eventwriter: ", err)
	}
	defer kb.Close()

	files, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to launch the Files App: ", err)
	}

	if err := files.OpenDownloads()(ctx); err != nil {
		s.Fatal("Failed to open Downloads folder in files app: ", err)
	}
	defer files.Close(cleanupCtx)

	for _, file := range audioFiles {
		if !audioFileRe.MatchString(file) {
			s.Fatalf("Unknown audio file format, want %q; got %q", audioFileRe, file)
		}

		if err := files.OpenFile(file)(ctx); err != nil {
			s.Fatalf("Failed to open the audio file %q: %v", file, err)
		}

		// Sample time for the audio to play for 5 seconds.
		if err := testing.Sleep(ctx, 5*time.Second); err != nil {
			s.Fatal("Error while waiting during sample time: ", err)
		}

		audioDeviceName, err := audionode.SetAudioNode(ctx, expectedAudioNode)
		if err != nil {
			s.Fatal("Failed to set the Audio node: ", err)
		}

		devName, err := crastestclient.FirstRunningDevice(ctx, audio.OutputStream)
		if err != nil {
			s.Fatal("Failed to detect running output device: ", err)
		}

		if audioDeviceName != devName {
			s.Fatalf("Failed to route the audio through expected audio node: got %q; want %q", devName, audioDeviceName)
		}

		// Closing the audio player.
		if kb.Accel(ctx, "Ctrl+W"); err != nil {
			s.Error("Failed to close Audio player: ", err)
		}
	}
}
