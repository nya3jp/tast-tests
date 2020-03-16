// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"
	"path/filepath"
	"time"

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/local/bundles/mtbf/video/youtube"
	"chromiumos/tast/local/chrome"
	mtbfchrome "chromiumos/tast/local/mtbf/chrome"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MTBF007PlayYoutube4K,
		Desc:         "YouTube 2K & 4K videos play properly. Only run this test on devices supporting 2K & 4K videos",
		Contacts:     []string{"xliu@cienet.com"},
		SoftwareDeps: []string{"chrome", "chrome_internal", "cros_video_decoder"},
		Pre:          chrome.LoginReuse(),
		Attr:         []string{"group:mainline", "informational"},
		Params: []testing.Param{{
			Name: "first_video",
			Val:  "https://www.youtube.com/watch?v=suWsd372pQE",
		}, {
			Name: "second_video",
			Val:  "http://www.youtube.com/watch?v=PcXOnoSN0RE",
		}},
	})
}

func MTBF007PlayYoutube4K(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	videoURL := s.Param().(string)

	conn, err := mtbfchrome.NewConn(ctx, cr, youtube.Add1SecondForURL(videoURL))
	if err != nil {
		s.Fatal("MTBF failed: ", err)
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
		if err = youtube.ChangeQuality(ctx, conn, youtube.Quality[quality]); err != nil {
			s.Error(mtbferrors.New(mtbferrors.VideoChgQuality, err, quality))
		}
		testing.Sleep(ctx, 20*time.Second) // Wait for video to change quality...
		if err = youtube.IsPlaying(ctx, conn, 3*time.Second); err != nil {
			s.Error(mtbferrors.New(mtbferrors.VideoNoPlay, err, videoURL))
			path := filepath.Join(s.OutDir(), "screenshot-youtube-failed-playing.png")
			if err := screenshot.CaptureChrome(ctx, cr, path); err != nil {
				s.Log("Failed to capture screenshot: ", err)
			}
		}
	}

	s.Log("Open stats for nerd")
	if err = youtube.OpenStatsForNerds(ctx, conn); err != nil {
		s.Error(mtbferrors.New(mtbferrors.VideoStatsNerd, err))
	}

	s.Log("Verify frame drops is zero")
	if mtbferr := youtube.CheckFramedrops(ctx, conn, 20*time.Second); mtbferr != nil {
		s.Error(mtbferr)
	}

	testing.Sleep(ctx, 5*time.Second)
}
