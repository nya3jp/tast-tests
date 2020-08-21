// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/mtbf/video/common"
	"chromiumos/tast/local/bundles/mtbf/video/youtube"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/mtbf/audio"
	mtbfchrome "chromiumos/tast/local/mtbf/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MTBF028AudioFocus,
		Desc:         "Plays youtube video and then plays m4a files",
		Contacts:     []string{"xliu@cienet.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "chrome_internal", "cros_video_decoder"},
		Data:         []string{"audio.m4a"},
		Vars:         []string{"video.youtubeVideo"},
		Pre:          chrome.LoginReuse(),
	})
}

// MTBF028AudioFocus case plays youtube video and .m4a audio file to verify their behavior.
// Load Youtube video and begin playing. Load .m4a audio file and begin playing. Verify Youtube video pauses.
// Start Youtube video playing again. Verify audio file pauses.
func MTBF028AudioFocus(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	videoURL := common.GetVar(ctx, s, "video.youtubeVideo")
	s.Log("Starting to jump to youtube url")
	conn, mtbferr := mtbfchrome.NewConn(ctx, cr, youtube.Add1SecondForURL(videoURL))
	if mtbferr != nil {
		s.Fatal(mtbferr)
	}
	defer conn.Close()
	defer conn.CloseTarget(ctx)

	s.Log("Youtube video is now ready for playing")
	testing.Sleep(ctx, 10*time.Second)

	s.Log("Preparing m4a to play")
	audioFile := "audio.m4a"

	audioplayer, mtbferr := audio.NewPlayer(ctx, cr)
	if mtbferr != nil {
		s.Fatal(mtbferr)
	}
	defer audioplayer.Close(ctx)

	testing.Sleep(ctx, 5*time.Second)

	if mtbferr := audioplayer.StartToPlay(ctx, audioFile, s.DataPath(audioFile)); mtbferr != nil {
		s.Fatal(mtbferr)
	}
	testing.Sleep(ctx, 5*time.Second)

	s.Log("Verify youtube has been paused")
	if mtbferr := youtube.IsPausing(ctx, conn, 3*time.Second); mtbferr != nil {
		s.Fatal(mtbferr)
	}

	testing.Sleep(ctx, 5*time.Second)

	s.Log("Play youtube and verify audio player paused")
	if mtbferr := youtube.PlayVideo(ctx, conn); mtbferr != nil {
		s.Fatal(mtbferr)
	}

	s.Log("Verify m4a has been paused")
	if mtbferr := audioplayer.IsPausing(ctx, time.Second*3); mtbferr != nil {
		s.Fatal(mtbferr)
	}

	testing.Sleep(ctx, 10*time.Second)
}
