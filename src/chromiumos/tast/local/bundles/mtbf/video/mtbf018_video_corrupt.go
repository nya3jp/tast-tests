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
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MTBF018VideoCorrupt,
		Desc:         "VideoHWCorruptedFile(MTBF018): Automated-test: video_VideoCorruption",
		Contacts:     []string{"xliu@cienet.com"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		Attr:         []string{"group:mainline", "informational"},
		Pre:          chrome.LoginReuse(),
		Data:         []string{"2.mp4"},
	})
}

// MTBF018VideoCorrupt case verifies an attempt to play a corrupted audio/video file does not cause a crash.
func MTBF018VideoCorrupt(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	s.Log("Preparing video to play")
	videoFile := "2.mp4"

	videoplayer, err := player.NewVideoPlayer(ctx, cr)
	if err != nil {
		s.Fatal("MTBF failed: ", err)
	}
	defer videoplayer.Close(ctx)

	s.Log("Open the test API")
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal(mtbferrors.New(mtbferrors.ChromeTestConn, err))
	}
	defer tconn.Close()

	if mtbferr := videoplayer.StartToPlay(ctx, videoFile, s.DataPath(videoFile)); mtbferr != nil {
		s.Error(mtbferr)
	}

	testing.Sleep(ctx, time.Second)

	if err := player.VerifyVideoPausing(ctx, tconn, time.Second*5); err != nil {
		s.Error(mtbferrors.New(mtbferrors.VideoPlayPause, err))
	}
}
