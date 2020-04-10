// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"
	"time"

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/local/bundles/mtbf/video/player"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MTBF025PlayPcmMov,
		Desc:         "PlayPcmMov(MTBF025): Play pcm files",
		Contacts:     []string{"xliu@cienet.com"},
		Attr:         []string{"group:mainline", "informational"},
		Pre:          chrome.LoginReuse(),
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		Data:         []string{"intelinsideA_pcm_s16be.mov", "intelinsideA_pcm_s24be.mov"},
		Params: []testing.Param{{
			Name: "pcms16be",
			Val:  "intelinsideA_pcm_s16be.mov",
		}, {
			Name: "pcms24be",
			Val:  "intelinsideA_pcm_s24be.mov",
		}},
	})
}

// MTBF025PlayPcmMov case verifies the varies music format files can be played and paused.
func MTBF025PlayPcmMov(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	s.Log("Preparing pcm to play")
	pcmFile := s.Param().(string)

	videoplayer, mtbferr := player.NewVideoPlayer(ctx, cr)
	if mtbferr != nil {
		s.Fatal(mtbferr)
	}
	defer videoplayer.Close(ctx)

	if mtbferr := videoplayer.StartToPlay(ctx, pcmFile, s.DataPath(pcmFile)); mtbferr != nil {
		s.Fatal(mtbferr)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kb.Close()

	if err := kb.Accel(ctx, "Alt+="); err != nil {
		s.Fatal(mtbferrors.New(mtbferrors.ChromeKeyPress, err, "Alt+="))
	}

	testing.Sleep(ctx, time.Second)

	if mtbferr := videoplayer.Pause(ctx); mtbferr != nil {
		s.Fatal(mtbferr)
	}

	testing.Sleep(ctx, time.Second)

	if mtbferr := videoplayer.Play(ctx); mtbferr != nil {
		s.Fatal(mtbferr)
	}

	testing.Sleep(ctx, 3*time.Second)

	if err := kb.Accel(ctx, "Alt+="); err != nil {
		s.Fatal(mtbferrors.New(mtbferrors.ChromeKeyPress, err, "Alt+="))
	}
}
