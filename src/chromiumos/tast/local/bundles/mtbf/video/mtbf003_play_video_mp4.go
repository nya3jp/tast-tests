// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/mtbf/video/media"
	"chromiumos/tast/local/chrome"
	mtbfchrome "chromiumos/tast/local/mtbf/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MTBF003PlayVideoMp4,
		Desc:         "Play mp4 video files with pause / resume / seek",
		Contacts:     []string{"xliu@cienet.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"cros_video_decoder", "chrome", "chrome_internal"},
		Pre:          chrome.LoginReuse(),
		Params: []testing.Param{{
			Name: "h264_720p",
			Val:  "https://storage.googleapis.com/chromiumos-test-assets-public/Shaka-Dash/720_60.mp4",
		}, {
			Name: "h264_1080p",
			Val:  "https://storage.googleapis.com/chromiumos-test-assets-public/AV-testing-files/(MP4(H.264),%201920%20x%201080,%20Stereo%2044KHz%20AAC).mp4",
		}},
	})
}

func MTBF003PlayVideoMp4(ctx context.Context, s *testing.State) {
	// videoPlayer is video element tag.
	const videoPlayer = "video"

	cr := s.PreValue().(*chrome.Chrome)
	videoURL := s.Param().(string)

	conn, mtbferr := mtbfchrome.NewConn(ctx, cr, videoURL)
	if mtbferr != nil {
		s.Fatal(mtbferr)
	}
	defer conn.Close()
	defer conn.CloseTarget(ctx)
	s.Log("Document is ready")

	s.Log("Play for 2 more seconds")
	testing.Sleep(ctx, 2*time.Second)

	s.Log("Pause / resume video")
	if mtbferr := media.PauseAndResume(ctx, conn, videoPlayer); mtbferr != nil {
		s.Fatal(mtbferr)
	}

	s.Log("Random seeking")
	if mtbferr := media.RandomSeek(ctx, conn, 5, videoPlayer); mtbferr != nil {
		s.Fatal(mtbferr)
	}

	if mtbferr := media.IsPlaying(ctx, conn, 5*time.Second, videoPlayer); mtbferr != nil {
		s.Fatal(mtbferr)
	}
}
