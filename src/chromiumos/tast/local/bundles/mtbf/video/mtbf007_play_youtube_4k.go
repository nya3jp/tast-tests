// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"
	"path/filepath"
	"time"

	"chromiumos/tast/local/chrome"
	mtbfchrome "chromiumos/tast/local/mtbf/chrome"
	"chromiumos/tast/local/mtbf/video/youtube"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MTBF007PlayYoutube4K,
		Desc:         "YouTube 2K & 4K videos play properly. Only run this test on devices supporting 2K & 4K videos",
		Contacts:     []string{"xliu@cienet.com"},
		SoftwareDeps: []string{"chrome", "chrome_internal", "cros_video_decoder"},
		Attr:         []string{"group:mainline", "informational"},
		Vars:         []string{"video.youtube4KVideo1", "video.youtube4KVideo2"},
		Pre:          chrome.LoginReuse(),
		Params: []testing.Param{{
			Name: "first_video",
			Val:  "video.youtube4KVideo1",
		}, {
			Name: "second_video",
			Val:  "video.youtube4KVideo2",
		}},
		Timeout: 3 * time.Minute,
	})
}

func MTBF007PlayYoutube4K(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	val := s.Param().(string)
	videoURL := s.RequiredVar(val)

	conn, mtbferr := mtbfchrome.NewConn(ctx, cr, youtube.Add1SecondForURL(videoURL))
	if mtbferr != nil {
		s.Fatal(mtbferr)
	}
	defer conn.Close()
	defer conn.CloseTarget(ctx)

	s.Log("Youtube video is now ready for 4K/2K playing")
	testing.Sleep(ctx, 5*time.Second)

	for _, quality := range []string{
		"4k",
		"2k",
	} {
		s.Log("Change video quality to ", quality)
		if mtbferr := youtube.ChangeQuality(ctx, conn, youtube.Quality[quality]); mtbferr != nil {
			s.Fatal(mtbferr)
		}
		s.Log("Verify video is currently playing")
		if mtbferr := youtube.IsPlaying(ctx, conn, 3*time.Second); mtbferr != nil {
			s.Fatal(mtbferr)
			path := filepath.Join(s.OutDir(), "screenshot-youtube-failed-playing.png")
			if err := screenshot.CaptureChrome(ctx, cr, path); err != nil {
				s.Log("Failed to capture screenshot: ", err)
			}
		}
	}

	s.Log("Open stats for nerd")
	if mtbferr := youtube.OpenStatsForNerds(ctx, conn); mtbferr != nil {
		s.Fatal(mtbferr)
	}

	s.Log("Verify frame drops is zero")
	if mtbferr := youtube.CheckFramedrops(ctx, conn, 20*time.Second); mtbferr != nil {
		s.Fatal(mtbferr)
	}

	testing.Sleep(ctx, 5*time.Second)
}
