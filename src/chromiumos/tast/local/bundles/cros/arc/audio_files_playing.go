// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arc/apputil/vlc"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/mtbf"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AudioFilesPlaying,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Play audio files via ARC++ app VLC player and verifies audio volume level is changed based on volume controls",
		Contacts:     []string{"ting.chen@cienet.com", "alfredyu@cienet.com", "cienet-development@googlegroups.com"},
		Attr:         []string{"group:mainline", "informational"},
		Data:         []string{"format_m4a.m4a", "format_mp3.mp3", "format_ogg.ogg", "format_wav.wav"},
		SoftwareDeps: []string{"chrome", "chrome_internal", "arc"},
		Fixture:      mtbf.ArcLoginReuseFixture,
		Timeout:      10 * time.Minute,
	})
}

// AudioFilesPlaying plays audio files via ARC++ app VLC player and verifies audio volume level is changed based on volume controls.
func AudioFilesPlaying(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*arc.PreData).Chrome
	a := s.FixtValue().(*arc.PreData).ARC

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	recorder, err := mtbf.NewRecorder(ctx)
	if err != nil {
		s.Fatal("Failed to start record performance: ", err)
	}
	defer recorder.Record(cleanupCtx, s.OutDir())

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to create the keyboard: ", err)
	}
	defer kb.Close()

	files := map[string]string{
		"m4a": "format_m4a.m4a",
		"mp3": "format_mp3.mp3",
		"ogg": "format_ogg.ogg",
		"wav": "format_wav.wav",
	}
	audioPath := filesapp.DownloadPath + "audios"
	if _, err := os.Stat(audioPath); os.IsNotExist(err) {
		s.Log("Create folder 'audios'")
		if err := os.Mkdir(audioPath, 0755); err != nil {
			s.Fatal("Failed to create audios folder: ", err)
		}
		defer os.Remove(audioPath)
	}
	for _, file := range files {
		audioFileLocation := filepath.Join(audioPath, file)
		if _, err := os.Stat(audioFileLocation); os.IsNotExist(err) {
			if err := fsutil.CopyFile(s.DataPath(file), audioFileLocation); err != nil {
				s.Fatalf("Failed to copy the test video to %s: %v", audioFileLocation, err)
			}
			defer os.Remove(audioFileLocation)
		}
	}

	vlcPlayer, err := vlc.NewVLCPlayer(ctx, cr, kb, tconn, a)
	if err != nil {
		s.Fatal("Failed to create VLC instance: ", err)
	}
	defer vlcPlayer.Close(cleanupCtx, cr, s.HasError, s.OutDir())

	if err := vlcPlayer.Launch(ctx); err != nil {
		s.Fatalf("Failed to launch app %q: %v", vlc.AppName, err)
	}

	if err := playAudioFiles(ctx, vlcPlayer, files); err != nil {
		s.Fatal("Failed to play audio: ", err)
	}
}

func playAudioFiles(ctx context.Context, vlcPlayer *vlc.Vlc, files map[string]string) error {
	testing.ContextLog(ctx, "Enter audio folder")
	if err := vlcPlayer.EnterAudioFolder(ctx); err != nil {
		testing.ContextLog(ctx, "Not entering audio folder or already in the folder")
	}

	for _, filename := range files {
		if err := vlcPlayer.PlayAudio(ctx, filename); err != nil {
			return err
		}

		testing.ContextLog(ctx, "Pause audio playing")
		if err := vlcPlayer.Pause(ctx); err != nil {
			return errors.Wrap(err, "failed to pause audio")
		}
	}
	return nil
}
