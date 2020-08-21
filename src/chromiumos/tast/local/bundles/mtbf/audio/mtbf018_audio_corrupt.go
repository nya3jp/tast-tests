// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package audio

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/mtbf/audio"
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
	s.Log("Preparing audio to play")
	audioFile := "corruptedAudio.mp3"

	audioplayer, mtbferr := audio.NewPlayer(ctx, cr)
	if mtbferr != nil {
		s.Fatal(mtbferr)
	}
	defer audioplayer.Close(ctx)

	if mtbferr := audioplayer.StartToPlay(ctx, audioFile, s.DataPath(audioFile)); mtbferr != nil {
		s.Fatal(mtbferr)
	}

	testing.Sleep(ctx, time.Second)

	if mtbferr := audioplayer.IsPausing(ctx, time.Second*5); mtbferr != nil {
		s.Fatal(mtbferr)
	}
}
