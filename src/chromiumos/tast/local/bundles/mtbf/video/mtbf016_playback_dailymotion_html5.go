// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"
	"time"

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/local/bundles/mtbf/video/dailymotion"
	"chromiumos/tast/local/chrome"
	mtbfchrome "chromiumos/tast/local/mtbf/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MTBF016PlaybackDailymotionHTML5,
		Desc:         "PlaybackDailymotionHTML5: To test Dailymotion video in different resolution and fullscreen/ expand/ shrink functionality",
		Contacts:     []string{"xliu@cienet.com"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		Attr:         []string{"group:mainline", "informational"},
		Pre:          chrome.LoginReuse(),
	})
}

func MTBF016PlaybackDailymotionHTML5(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	url := "https://www.dailymotion.com/video/x7mi4l2?playlist=x6huns" // Dailymotion video without AD

	conn, mtbferr := mtbfchrome.NewConn(ctx, cr, url)
	if mtbferr != nil {
		s.Fatal(mtbferr)
	}
	defer conn.Close()
	defer conn.CloseTarget(ctx)
	s.Log("Document is ready")

	s.Log("Toggle fullscreen mode")
	dailymotion.ToggleFullScreen(ctx, conn)
	testing.Sleep(ctx, 5*time.Second)
	if err := dailymotion.IsPlaying(ctx, conn, 3*time.Second); err != nil {
		s.Fatal(mtbferrors.New(mtbferrors.VideoVeriPlay, err))
	}

	for _, quality := range []string{
		"1080p",
		"720p",
		"480p",
		"380p",
		"240p",
		"144p",
	} {
		s.Log("Change video quality to ", quality)
		if err := dailymotion.ChangeQuality(ctx, conn, dailymotion.Quality[quality]); err != nil {
			s.Fatal(mtbferrors.New(mtbferrors.VideoChgQuality2, err, quality))
		}
		testing.Sleep(ctx, 5*time.Second) // Wait for video to change quality...
		if err := dailymotion.IsPlaying(ctx, conn, 3*time.Second); err != nil {
			s.Fatal(mtbferrors.New(mtbferrors.VideoVeriPlay, err))
		}
	}

	dailymotion.ToggleFullScreen(ctx, conn)
	testing.Sleep(ctx, 5*time.Second)
	if err := dailymotion.IsPlaying(ctx, conn, 3*time.Second); err != nil {
		s.Fatal(mtbferrors.New(mtbferrors.VideoVeriPlay, err))
	}

	s.Log("Pause/ resume video")
	if err := dailymotion.PauseAndResume(ctx, conn); err != nil {
		s.Fatal(mtbferrors.New(mtbferrors.VideoPauseResume, err))
	}

	s.Log("Random seeking")
	if err := dailymotion.RandomSeek(ctx, conn, 5); err != nil {
		s.Fatal(mtbferrors.New(mtbferrors.VideoSeek, err))
	}
}
