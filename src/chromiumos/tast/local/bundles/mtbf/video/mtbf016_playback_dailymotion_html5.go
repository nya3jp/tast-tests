// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/mtbf/video/common"
	"chromiumos/tast/local/bundles/mtbf/video/dailymotion"
	"chromiumos/tast/local/chrome"
	mtbfchrome "chromiumos/tast/local/mtbf/chrome"
	"chromiumos/tast/local/mtbf/debug"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MTBF016PlaybackDailymotionHTML5,
		Desc:         "PlaybackDailymotionHTML5: To test Dailymotion video in different resolution and fullscreen/ expand/ shrink functionality",
		Contacts:     []string{"xliu@cienet.com"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		Attr:         []string{"group:mainline", "informational"},
		Vars:         []string{"video.dailymotionVideo"},
		Pre:          chrome.LoginReuse(),
		Timeout:      10 * time.Minute,
	})
}

// MTBF016PlaybackDailymotionHTML5 test Dailymotion video in different resolution and fullscreen/ expand/ shrink functionality
func MTBF016PlaybackDailymotionHTML5(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	url := common.GetVar(ctx, s, "video.dailymotionVideo")

	conn, mtbferr := mtbfchrome.NewConn(ctx, cr, url)
	if mtbferr != nil {
		s.Fatal(mtbferr)
	}
	defer conn.Close()
	defer conn.CloseTarget(ctx)
	s.Log("Document is ready")

	s.Log("Toggle fullscreen mode")
	dailymotion.ToggleFullScreen(ctx, conn)

	// Wait for video to toggle full screen...
	if mtbferr := dailymotion.WaitForReadyState(ctx, conn); mtbferr != nil {
		debug.TakeScreenshot(ctx)
		s.Fatal(mtbferr)
	}
	testing.Sleep(ctx, 10*time.Second)
	dailymotion.SkipAD(ctx, conn)

	if mtbferr := dailymotion.IsPlaying(ctx, conn, 3*time.Second); mtbferr != nil {
		debug.TakeScreenshot(ctx)
		s.Fatal(mtbferr)
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
		if mtbferr := dailymotion.ChangeQuality(ctx, conn, dailymotion.Quality[quality]); mtbferr != nil {
			debug.TakeScreenshot(ctx)
			s.Fatal(mtbferr)
		}
		s.Log("Verify video is currently playing")
		testing.Sleep(ctx, 3*time.Second)
		if mtbferr := dailymotion.IsPlaying(ctx, conn, 3*time.Second); mtbferr != nil {
			debug.TakeScreenshot(ctx)
			s.Fatal(mtbferr)
		}
	}

	dailymotion.ToggleFullScreen(ctx, conn)
	// Wait for video to toggle full screen...
	if mtbferr := dailymotion.WaitForReadyState(ctx, conn); mtbferr != nil {
		debug.TakeScreenshot(ctx)
		s.Fatal(mtbferr)
	}

	s.Log("Pause/ resume video")
	if mtbferr := dailymotion.PauseAndResume(ctx, conn); mtbferr != nil {
		debug.TakeScreenshot(ctx)
		s.Fatal(mtbferr)
	}

	s.Log("Random seeking")
	if mtbferr := dailymotion.RandomSeek(ctx, conn, 5); mtbferr != nil {
		debug.TakeScreenshot(ctx)
		s.Fatal(mtbferr)
	}

	if mtbferr := dailymotion.IsPlaying(ctx, conn, 3*time.Second); mtbferr != nil {
		debug.TakeScreenshot(ctx)
		s.Fatal(mtbferr)
	}
}
