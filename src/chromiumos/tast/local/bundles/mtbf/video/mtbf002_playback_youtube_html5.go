// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"
	"time"

	"chromiumos/tast/common/mtbferrors"
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
		Pre:          chrome.LoggedIn(),
		Attr:         []string{"group:mainline", "informational"},
	})
}

func MTBF002PlaybackYoutubeHTML5(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	url := "https://www.youtube.com/watch?v=ZFvPLrKZywA" // YouTube video without AD.

	conn, err := mtbfchrome.NewConn(ctx, cr, youtube.Add1SecondForURL(url))
	if err != nil {
		s.Fatal("MTBF failed: ", err)
	}
	defer conn.Close()
	defer conn.CloseTarget(ctx)
	s.Log("Document is ready")

	s.Log("Toggle fullscreen mode")
	youtube.ToggleFullScreen(ctx, conn)
	if err = youtube.IsPlaying(ctx, conn, 5*time.Second); err != nil {
		s.Fatal(mtbferrors.New(mtbferrors.VideoNoPlay, err, url))
	}
	youtube.ToggleFullScreen(ctx, conn)
	if err = youtube.IsPlaying(ctx, conn, 5*time.Second); err != nil {
		s.Fatal(mtbferrors.New(mtbferrors.VideoNoPlay, err, url))
	}

	for _, quality := range []string{
		"1080p",
		"720p",
		"480p",
		"360p",
	} {
		s.Log("Change video quality to ", quality)
		if err = youtube.ChangeQuality(ctx, conn, youtube.Quality[quality]); err != nil {
			s.Error(mtbferrors.New(mtbferrors.VideoChgQuality, err, quality))
		}
		testing.Sleep(ctx, 2*time.Second) // Wait for video to change quality...
		if err = youtube.IsPlaying(ctx, conn, 3*time.Second); err != nil {
			s.Fatal(mtbferrors.New(mtbferrors.VideoNoPlay, err, url))
		}
	}

	s.Log("Pause/ resume video")
	if err = youtube.PauseAndResume(ctx, conn); err != nil {
		s.Error(mtbferrors.New(mtbferrors.VideoPauseResume, err))
	}

	s.Log("Random seeking")
	if err = youtube.RandomSeek(ctx, conn, 5); err != nil {
		s.Error(mtbferrors.New(mtbferrors.VideoSeek, err))
	}

	s.Log("Open stats for nerd")
	if err = youtube.OpenStatsForNerds(ctx, conn); err != nil {
		s.Error(mtbferrors.New(mtbferrors.VideoStatsNerd, err))
	}
	if err = youtube.IsPlaying(ctx, conn, 5*time.Second); err != nil {
		s.Fatal(mtbferrors.New(mtbferrors.VideoNoPlay, err, url))
	}
}
