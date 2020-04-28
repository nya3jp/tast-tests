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
	mtbfchrome "chromiumos/tast/local/mtbf/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MTBF002PlaybackYoutubeHTML5,
		Desc:         "Video Playback | Youtube HTML5 - To test YouTube video in different resolution and fullscreen / expand / shrink functionality",
		Contacts:     []string{"xliu@cienet.com"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		Attr:         []string{"group:mainline", "informational"},
		Pre:          chrome.LoginReuse(),
		Vars:         []string{"video.youtubeVideo"},
		Timeout:      4 * time.Minute,
	})
}

func MTBF002PlaybackYoutubeHTML5(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	url := common.GetVar(ctx, s, "video.youtubeVideo")

	conn, mtbferr := mtbfchrome.NewConn(ctx, cr, youtube.Add1SecondForURL(url))
	if mtbferr != nil {
		s.Fatal(mtbferr)
	}
	defer conn.Close()
	defer conn.CloseTarget(ctx)
	s.Log("Document is ready")

	s.Log("Toggle fullscreen mode")
	youtube.ToggleFullScreen(ctx, conn)
	if mtbferr := youtube.IsPlaying(ctx, conn, 5*time.Second); mtbferr != nil {
		s.Fatal(mtbferr)
	}
	youtube.ToggleFullScreen(ctx, conn)
	if mtbferr := youtube.IsPlaying(ctx, conn, 5*time.Second); mtbferr != nil {
		s.Fatal(mtbferr)
	}

	for _, quality := range []string{
		"1080p",
		"720p",
		"480p",
		"360p",
	} {
		s.Log("Change video quality to ", quality)
		if mtbferr := youtube.ChangeQuality(ctx, conn, youtube.Quality[quality]); mtbferr != nil {
			s.Fatal(mtbferr)
		}
		testing.Sleep(ctx, 2*time.Second) // Wait for video to change quality...
		if mtbferr := youtube.IsPlaying(ctx, conn, 3*time.Second); mtbferr != nil {
			s.Fatal(mtbferr)
		}
	}

	s.Log("Pause/ resume video")
	if mtbferr := youtube.PauseAndResume(ctx, conn); mtbferr != nil {
		s.Fatal(mtbferr)
	}

	s.Log("Random seeking")
	if mtbferr := youtube.RandomSeek(ctx, conn, 5); mtbferr != nil {
		s.Fatal(mtbferr)
	}

	s.Log("Open stats for nerd")
	if mtbferr := youtube.OpenStatsForNerds(ctx, conn); mtbferr != nil {
		s.Fatal(mtbferr)
	}
	if mtbferr := youtube.IsPlaying(ctx, conn, 5*time.Second); mtbferr != nil {
		s.Fatal(mtbferr)
	}
}
