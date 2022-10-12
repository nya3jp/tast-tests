// Copyright 2021 The ChromiumOS Authors
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
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CheckingAudioFormats,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies supported audio file formats",
		Contacts:     []string{"pathan.jilani@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		HardwareDeps: hwdep.D(hwdep.Speaker()),
		Data:         []string{"audio.flac", "audio.m4a", "audio.ogg", "audio.wav", "audio.mp3", "audio.5.1.mp3"},
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

	downloadsPath, err := cryptohome.DownloadsPath(ctx, cr.NormalizedUser())
	if err != nil {
		s.Fatal("Failed to get user's Download path: ", err)
	}

	audioFiles := []string{"audio.flac", "audio.m4a", "audio.ogg", "audio.wav", "audio.mp3", "audio.5.1.mp3"}
	audioFileRe := regexp.MustCompile(`^audio.(wav|m4a|ogg|flac|mp3|5.1.mp3)$`)

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

		_, err := crastestclient.FirstRunningDevice(ctx, audio.OutputStream)
		if err != nil {
			s.Fatal("Failed to detect running audio stream: ", err)
		}

		// Closing the audio player.
		if kb.Accel(ctx, "Ctrl+W"); err != nil {
			s.Error("Failed to close Audio player: ", err)
		}
	}
}
