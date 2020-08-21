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

	audioplayer, mtbferr := audio.NewPlayer(ctx, cr)
	if mtbferr != nil {
		s.Fatal(mtbferr)
	}
	defer audioplayer.Close(ctx)

	if mtbferr := audioplayer.StartToPlay(ctx, audioFile, s.DataPath(audioFile)); mtbferr != nil {
		s.Fatal(mtbferr)
	}
	testing.Sleep(ctx, time.Second)

	if mtbferr := audioplayer.Pause(ctx); mtbferr != nil {
		s.Fatal(mtbferr)
	}
	testing.Sleep(ctx, time.Second)

	if mtbferr := audioplayer.IsPausing(ctx, time.Second*1); mtbferr != nil {
		s.Fatal(mtbferr)
	}

	if mtbferr := audioplayer.Play(ctx); mtbferr != nil {
		s.Fatal(mtbferr)
	}
	testing.Sleep(ctx, time.Second)

	s.Log("Verify audio player is playing")
	if mtbferr := audioplayer.IsPlaying(ctx, time.Second*1); mtbferr != nil {
		s.Fatal(mtbferr)
	}
	testing.Sleep(ctx, 3*time.Second)
}
