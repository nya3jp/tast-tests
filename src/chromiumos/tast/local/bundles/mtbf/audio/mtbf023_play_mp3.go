// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package audio

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/mtbf/audio"
	"chromiumos/tast/local/mtbf/debug"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MTBF023PlayMp3,
		Desc:         "Play mp3 files from file app, pause and resume",
		Contacts:     []string{"xliu@cienet.com"},
		Attr:         []string{"group:mainline", "informational"},
		Pre:          chrome.LoginReuse(),
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		Data:         []string{"audio.mp3"},
	})
}

// MTBF023PlayMp3 play mp3 files from file app, pause and resume.
func MTBF023PlayMp3(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	s.Log("Preparing mp3 to play")
	audioFile := "audio.mp3"

	audioplayer, mtbferr := audio.NewPlayer(ctx, cr)
	if mtbferr != nil {
		s.Fatal(mtbferr)
	}
	defer audioplayer.Close(ctx)

	testing.Sleep(ctx, 5*time.Second)

	s.Log("Pause and play mp3")
	if mtbferr := audioplayer.StartToPlay(ctx, audioFile, s.DataPath(audioFile)); mtbferr != nil {
		debug.TakeScreenshot(ctx)
		s.Fatal(mtbferr)
	}
	testing.Sleep(ctx, time.Second)

	s.Logf("Pause %s", audioFile)
	if mtbferr := audioplayer.Pause(ctx); mtbferr != nil {
		debug.TakeScreenshot(ctx)
		debug.TakeScreenshot(ctx)
		s.Fatal(mtbferr)
	}
	testing.Sleep(ctx, time.Second)

	s.Log("Verify audio player is paused")
	if mtbferr := audioplayer.IsPausing(ctx, time.Second*3); mtbferr != nil {
		debug.TakeScreenshot(ctx)
		s.Fatal(mtbferr)
	}

	s.Logf("Play %s", audioFile)
	if mtbferr := audioplayer.Play(ctx); mtbferr != nil {
		debug.TakeScreenshot(ctx)
		s.Fatal(mtbferr)
	}
	testing.Sleep(ctx, time.Second)

	s.Log("Verify audio player is playing")
	if mtbferr := audioplayer.IsPlaying(ctx, time.Second*3); mtbferr != nil {
		debug.TakeScreenshot(ctx)
		s.Fatal(mtbferr)
	}

	testing.Sleep(ctx, 10*time.Second)
}
