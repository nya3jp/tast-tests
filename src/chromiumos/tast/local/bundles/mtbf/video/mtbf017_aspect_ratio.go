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
		Func:         MTBF017AspectRatio,
		Desc:         "Verify that we play movies in two different aspect ratios 4:3 and 16:9",
		Contacts:     []string{"xliu@cienet.com"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		Pre:          chrome.LoginReuse(),
		Attr:         []string{"group:mainline", "informational"},
		Params: []testing.Param{{
			Name: "16by9",
			Val:  "https://www.youtube.com/watch?v=WUgvvPRH7Oc",
		}, {
			Name: "4by3",
			Val:  "https://www.youtube.com/watch?v=mM5_T-F1Yn4",
		}},
	})
}

// MTBF017AspectRatio case verifies videos with 16:9 and 4:3 aspect ratio can be played.
// Load Youtube video. Bring up StatsForNerds.
// Verify aspect ratio, pause and resume, fast forward, and rewind functionality.
// Verify full screen can be toggled while video is paused or playing.
func MTBF017AspectRatio(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	videoURL := s.Param().(string)

	conn, err := mtbfchrome.NewConn(ctx, cr, youtube.Add1SecondForURL(videoURL))
	if err != nil {
		s.Fatal("MTBF failed: ", err)
	}
	defer conn.Close()
	defer conn.CloseTarget(ctx)
	s.Log("Youtube video is now ready for playing")
	s.Log("Open stats for nerd")
	if err = youtube.OpenStatsForNerds(ctx, conn); err != nil {
		s.Error(mtbferrors.New(mtbferrors.VideoStatsNerd, err))
	}
	testing.Sleep(ctx, 1*time.Second)

	s.Log("Get aspect ration from stats for nerd")
	var videoFrame youtube.VideoFrame
	videoFrame, err = youtube.GetCurrentResolutionFromStatsForNerds(ctx, conn)
	if err != nil {
		s.Error(mtbferrors.New(mtbferrors.VideoGetRatio, err))
	}
	if !is16by9(videoFrame.X, videoFrame.Y) && !is4by3(videoFrame.X, videoFrame.Y) {
		s.Error(mtbferrors.New(mtbferrors.VideoRatio, nil, videoFrame.X, videoFrame.Y))
	}

	s.Log("Verify pause and resume video")
	if err = youtube.PauseAndResume(ctx, conn); err != nil {
		s.Error(mtbferrors.New(mtbferrors.VideoPauseResume, err))
	}

	s.Log("Verify fast forward, and rewind")
	if err = youtube.FastForward(ctx, conn); err != nil {
		s.Error(mtbferrors.New(mtbferrors.VideoFastFwd, err))
	}
	testing.Sleep(ctx, 1*time.Second)
	if err = youtube.FastRewind(ctx, conn); err != nil {
		s.Error(mtbferrors.New(mtbferrors.VideoFastRwd, err))
	}

	s.Log("Verify entering full screen while pause")
	if err = youtube.PauseVideo(ctx, conn); err != nil {
		s.Error(mtbferrors.New(mtbferrors.VideoNoPause, err, "Youtube"))
	}
	toggleFullscreen(ctx, conn, s)

	s.Log("Verify entering full screen while playing")
	if err = youtube.PlayVideo(ctx, conn); err != nil {
		s.Error(mtbferrors.New(mtbferrors.VideoNoPlay, err, "Youtube"))
	}
	toggleFullscreen(ctx, conn, s)
}

// toggleFullscreen switches the video to full screen display and then back to normal display.
func toggleFullscreen(ctx context.Context, conn *chrome.Conn, s *testing.State) {
	var err error
	if err = youtube.ToggleFullScreen(ctx, conn); err != nil {
		s.Error(mtbferrors.New(mtbferrors.VideoEnterFullSc, err))
	}
	testing.Sleep(ctx, 2*time.Second)
	if err = youtube.ToggleFullScreen(ctx, conn); err != nil {
		s.Error(mtbferrors.New(mtbferrors.VideoEnterFullSc, err))
	}
	testing.Sleep(ctx, 2*time.Second)
}

// findGCF is used to find the Greatest Common Factor to determine the aspect ratio of the video.
func findGCF(x int, y int) int {
	start := x
	if y > x {
		start = y
	}
	for i := start; i > 0; i-- {
		if y%i == 0 && x%i == 0 {
			return i
		}
	}
	return 0
}

func is16by9(x int, y int) bool {
	gcf := findGCF(x, y)
	if gcf == 0 {
		return false
	}
	return x/gcf == 16 && y/gcf == 9
}

func is4by3(x int, y int) bool {
	gcf := findGCF(x, y)
	if gcf == 0 {
		return false
	}
	return x/gcf == 4 && y/gcf == 3
}
