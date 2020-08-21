// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"
	"time"

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/local/chrome"
	mtbfchrome "chromiumos/tast/local/mtbf/chrome"
	"chromiumos/tast/local/mtbf/video/youtube"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MTBF017AspectRatio,
		Desc:         "Verify that we play movies in two different aspect ratios 4:3 and 16:9",
		Contacts:     []string{"xliu@cienet.com"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		Attr:         []string{"group:mainline", "informational"},
		Vars:         []string{"video.youtube16by9Video", "video.youtube4by3Video"},
		Pre:          chrome.LoginReuse(),
		Params: []testing.Param{{
			Name: "16by9",
			Val:  "video.youtube16by9Video",
		}, {
			Name: "4by3",
			Val:  "video.youtube4by3Video",
		}},
	})
}

// MTBF017AspectRatio case verifies videos with 16:9 and 4:3 aspect ratio can be played.
// Load Youtube video. Bring up StatsForNerds.
// Verify aspect ratio, pause and resume, fast forward, and rewind functionality.
// Verify full screen can be toggled while video is paused or playing.
func MTBF017AspectRatio(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	val := s.Param().(string)
	videoURL := s.RequiredVar(val)

	conn, mtbferr := mtbfchrome.NewConn(ctx, cr, youtube.Add1SecondForURL(videoURL))
	if mtbferr != nil {
		s.Fatal(mtbferr)
	}
	defer conn.Close()
	defer conn.CloseTarget(ctx)
	s.Log("Youtube video is now ready for playing")
	if mtbferr := youtube.WaitForReadyState(ctx, conn); mtbferr != nil {
		s.Fatal(mtbferr)
	}
	s.Log("Open stats for nerd")
	if mtbferr := youtube.OpenStatsForNerds(ctx, conn); mtbferr != nil {
		s.Fatal(mtbferr)
	}
	testing.Sleep(ctx, 1*time.Second)

	s.Log("Get aspect ration from stats for nerd")
	var videoFrame youtube.VideoFrame
	videoFrame, mtbferr = youtube.GetCurrentResolutionFromStatsForNerds(ctx, conn)
	if mtbferr != nil {
		s.Fatal(mtbferr)
	}
	if !is16by9(videoFrame.X, videoFrame.Y) && !is4by3(videoFrame.X, videoFrame.Y) {
		s.Fatal(mtbferrors.New(mtbferrors.VideoRatio, nil, videoFrame.X, videoFrame.Y))
	}

	s.Log("Verify pause and resume video")
	if mtbferr := youtube.PauseAndResumeWithoutDebug(ctx, conn); mtbferr != nil {
		s.Fatal(mtbferr)
	}

	s.Log("Verify fast forward, and rewind")
	if mtbferr := youtube.FastForward(ctx, conn); mtbferr != nil {
		s.Fatal(mtbferr)
	}
	testing.Sleep(ctx, 1*time.Second)
	if mtbferr := youtube.FastRewind(ctx, conn); mtbferr != nil {
		s.Fatal(mtbferr)
	}

	s.Log("Verify entering full screen while pause")
	if mtbferr := youtube.PauseVideo(ctx, conn); mtbferr != nil {
		s.Fatal(mtbferr)
	}
	toggleFullscreen(ctx, conn, s)

	s.Log("Verify entering full screen while playing")
	if mtbferr := youtube.PlayVideo(ctx, conn); mtbferr != nil {
		s.Fatal(mtbferr)
	}
	toggleFullscreen(ctx, conn, s)
}

// toggleFullscreen switches the video to full screen display and then back to normal display.
func toggleFullscreen(ctx context.Context, conn *chrome.Conn, s *testing.State) {
	if mtbferr := youtube.ToggleFullScreen(ctx, conn); mtbferr != nil {
		s.Fatal(mtbferr)
	}
	testing.Sleep(ctx, 2*time.Second)
	if mtbferr := youtube.ToggleFullScreen(ctx, conn); mtbferr != nil {
		s.Fatal(mtbferr)
	}
	testing.Sleep(ctx, 2*time.Second)
}

// findGCF is used to find the Greatest Common Factor to determine the aspect ratio of the video.
func findGCF(x, y int) int {
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

func is16by9(x, y int) bool {
	gcf := findGCF(x, y)
	if gcf == 0 {
		return false
	}
	return x/gcf == 16 && y/gcf == 9
}

func is4by3(x, y int) bool {
	gcf := findGCF(x, y)
	if gcf == 0 {
		return false
	}
	return x/gcf == 4 && y/gcf == 3
}
