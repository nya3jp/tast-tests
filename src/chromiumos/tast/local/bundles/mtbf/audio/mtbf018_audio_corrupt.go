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
		Func:         MTBF018AudioCorrupt,
		Desc:         "AudioHWCorruptedFile(MTBF018): Automated-test: audio_AudioCorruption",
		Contacts:     []string{"xliu@cienet.com"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		Attr:         []string{"group:mainline", "informational"},
		Pre:          chrome.LoginReuse(),
		Data:         []string{"corruptedAudio.mp3"},
	})
}

// MTBF018AudioCorrupt case verifies an attempt to play a corrupted audio/video file does not cause a crash.
func MTBF018AudioCorrupt(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	audioFile := "corruptedAudio.mp3"

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

	if mtbferr := audio.PlayFromDownloadsFolder(ctx, files, s.DataPath(audioFile), audioFile); err != nil {
		s.Fatal(mtbferr)
	}

	defer audio.ClickButton(ctx, tconn, "Close")
	testing.Sleep(ctx, 2*time.Second)

	s.Log("Verify audio player is paused")
	if mtbferr := audio.IsPausing(ctx, tconn, 5*time.Second); mtbferr != nil {
		s.Fatal(mtbferr)
	}
}
