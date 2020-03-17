// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package audio

import (
	"context"
	"time"

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/local/bundles/mtbf/audio/player"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/ui/filesapp"
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

func MTBF023PlayMp3(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	s.Log("Preparing mp3 to play")
	audioFile := "audio.mp3"

	s.Log("Open the test API")
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal(mtbferrors.New(mtbferrors.ChromeTestConn, err))
	}
	defer tconn.Close()
	defer tconn.CloseTarget(ctx)

	s.Log("Open the Files App")
	files, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Error(mtbferrors.New(mtbferrors.ChromeOpenApp, err, "Files App"))
	}
	defer filesapp.Close(ctx, tconn)

	audioplayer, mtbferr := player.New(s, audioFile, s.DataPath(audioFile))
	if mtbferr != nil {
		s.Fatal(mtbferr)
	}

	if mtbferr := audioplayer.StartToPlay(ctx, files); mtbferr != nil {
		s.Error(mtbferr)
	}
	defer player.ClickButton(ctx, tconn, "Close")

	testing.Sleep(ctx, 5*time.Second)

	s.Log("Pause and play mp3")
	if mtbferr = player.Pause(ctx, tconn); mtbferr != nil {
		s.Error(mtbferr)
	}

	testing.Sleep(ctx, 3*time.Second)
	if mtbferr = player.Play(ctx, tconn); mtbferr != nil {
		s.Error(mtbferr)
	}
	if err = player.IsPlaying(ctx, tconn, 5*time.Second); err != nil {
		s.Error(mtbferrors.New(mtbferrors.AudioPlayPause, err))
	}

	testing.Sleep(ctx, 10*time.Second)
}
