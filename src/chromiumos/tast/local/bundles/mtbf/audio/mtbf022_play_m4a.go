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
		Func:         MTBF022PlayM4a,
		Desc:         "Play m4a files",
		Contacts:     []string{"xliu@cienet.com"},
		Attr:         []string{"group:mainline", "informational"},
		Pre:          chrome.LoginReuse(),
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		Data:         []string{"audio.m4a"},
	})
}

// MTBF022PlayM4a plays m4a audio file.
func MTBF022PlayM4a(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	s.Log("Preparing m4a to play")
	audioFile := "audio.m4a"

	audioplayer, mtbferr := audio.NewPlayer(ctx, cr)
	if mtbferr != nil {
		s.Fatal(mtbferr)
	}
	defer audioplayer.Close(ctx)

	if mtbferr := audioplayer.StartToPlay(ctx, audioFile, s.DataPath(audioFile)); mtbferr != nil {
		s.Fatal(mtbferr)
	}
	testing.Sleep(ctx, time.Second*5)

	if mtbferr := audioplayer.Pause(ctx); mtbferr != nil {
		s.Fatal(mtbferr)
	}
	testing.Sleep(ctx, time.Second)

	if mtbferr := audioplayer.IsPausing(ctx, time.Second*3); mtbferr != nil {
		s.Fatal(mtbferr)
	}

	if mtbferr := audioplayer.Play(ctx); mtbferr != nil {
		s.Fatal(mtbferr)
	}
	testing.Sleep(ctx, time.Second)

	if mtbferr := audioplayer.IsPlaying(ctx, time.Second*5); mtbferr != nil {
		s.Fatal(mtbferr)
	}

	testing.Sleep(ctx, time.Second*10)
}
