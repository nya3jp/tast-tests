// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package audio

import (
	"context"
	"time"

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/mtbf/audio"
	mtbfFilesapp "chromiumos/tast/local/mtbf/ui/filesapp"
	"chromiumos/tast/local/ui/filesapp"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MTBF025PlayPcmWav,
		Desc:         "PlayPcmWav(MTBF025): Play pcm files",
		Contacts:     []string{"xliu@cienet.com"},
		Attr:         []string{"group:mainline", "informational"},
		Pre:          chrome.LoginReuse(),
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		Data:         []string{"GLASS.wav"},
	})
}

// MTBF025PlayPcmWav play pcm files
func MTBF025PlayPcmWav(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	s.Log("Preparing PCM to play")
	audioFile := "GLASS.wav"

	s.Log("Open the test API")
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal(mtbferrors.New(mtbferrors.ChromeTestConn, err))
	}
	defer tconn.Close()

	s.Log("Open the Files App")
	files, mtbferr := mtbfFilesapp.Launch(ctx, tconn)
	if mtbferr != nil {
		s.Fatal(mtbferr)
	}
	defer filesapp.Close(ctx, tconn)

	if mtbferr := audio.PlayFromDownloadsFolder(ctx, files, s.DataPath(audioFile), audioFile); mtbferr != nil {
		s.Fatal(mtbferr)
	}
	defer audio.ClickButton(ctx, tconn, "Close")

	testing.Sleep(ctx, time.Second)

	s.Log("Pause and play PCM")
	if err := files.WaitForElement(ctx, "button", "Pause", time.Minute); err != nil {
		s.Fatal(mtbferrors.New(mtbferrors.AudioWaitPauseButton, err))
	}

	if mtbferr = audio.Pause(ctx, tconn); mtbferr != nil {
		s.Fatal(mtbferr)
	}
	testing.Sleep(ctx, time.Second)
	s.Log("Verify audio player is paused")
	if mtbferr := audio.IsPausing(ctx, tconn, 3*time.Second); mtbferr != nil {
		s.Fatal(mtbferr)
	}

	if err := files.WaitForElement(ctx, "button", "Play", time.Minute); err != nil {
		s.Fatal(mtbferrors.New(mtbferrors.AudioWaitPlayButton, err))
	}

	if mtbferr = audio.Play(ctx, tconn); mtbferr != nil {
		s.Fatal(mtbferr)
	}
	testing.Sleep(ctx, time.Second)
	s.Log("Verify audio player is playing")
	if mtbferr = audio.IsPlaying(ctx, tconn, 3*time.Second); mtbferr != nil {
		s.Fatal(mtbferr)
	}

	testing.Sleep(ctx, 3*time.Second)
}
