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
	mtbfFilesapp "chromiumos/tast/local/mtbf/ui/filesapp"
	"chromiumos/tast/local/ui/filesapp"
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

	s.Log("Open the test API")
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal(mtbferrors.New(mtbferrors.ChromeTestConn, err))
	}
	defer tconn.Close()
	defer tconn.CloseTarget(ctx)

	s.Log("Open the Files App")
	files, mtbferr := mtbfFilesapp.Launch(ctx, tconn)
	if mtbferr != nil {
		s.Fatal(mtbferr)
	}
	defer filesapp.Close(ctx, tconn)

	audioplayer, mtbferr := player.New(s, audioFile, s.DataPath(audioFile))
	if mtbferr != nil {
		s.Fatal(mtbferr)
	}

	if mtbferr = audioplayer.StartToPlay(ctx, files); mtbferr != nil {
		s.Error(mtbferr)
	}
	defer player.ClickButton(ctx, tconn, "Close")

	testing.Sleep(ctx, 5*time.Second)

	s.Log("Pause and play m4a")
	if mtbferr = player.Pause(ctx, tconn); mtbferr != nil {
		s.Error(mtbferr)
	}

	testing.Sleep(ctx, 3*time.Second)
	if mtbferr = player.Play(ctx, tconn); mtbferr != nil {
		s.Error(mtbferr)
	}
	if mtbferr = player.IsPlaying(ctx, tconn, 5*time.Second); err != nil {
		s.Error(mtbferrors.New(mtbferrors.AudioPlayPause, mtbferr))
	}
	testing.Sleep(ctx, 10*time.Second)
}
