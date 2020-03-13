// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"
	"time"

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/local/bundles/mtbf/video/media"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

// videoPlayer is video element tag.
const videoPlayer = "video"

func init() {
	testing.AddTest(&testing.Test{
		Func:         MTBF003PlayVideoMp4,
		Desc:         "Play mp4 video files",
		Contacts:     []string{"xliu@cienet.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"cros_video_decoder", "chrome", "chrome_internal"},
		Pre:          chrome.LoggedIn(),
		Params: []testing.Param{{
			Name: "h264_720p",
			Val:  "https://storage.googleapis.com/chromiumos-test-assets-public/Shaka-Dash/720_60.mp4",
		}, {
			Name: "h264_1080p",
			Val:  "https://storage.googleapis.com/chromiumos-test-assets-public/AV-testing-files/(MP4(H.264),%201920%20x%201080,%20Stereo%2044KHz%20AAC).mp4",
		}},
	})
}

// MTBF003PlayVideoMp4 plays mp4 video for 1080/720 resolution.
func MTBF003PlayVideoMp4(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	videoURL := s.Param().(string)

	conn, err := cr.NewConn(ctx, videoURL)
	if err != nil {
		s.Fatal(mtbferrors.New(mtbferrors.Err3200, err, videoURL))
	}
	defer conn.Close()
	defer conn.CloseTarget(ctx)
	s.Log("Document is ready")

	s.Log("Play for 2 more seconds")
	testing.Sleep(ctx, 2*time.Second)

	s.Log("Pause / resume video")
	if err = media.VerifyPauseAndResumeElement(ctx, conn, videoPlayer); err != nil {
		s.Fatal(mtbferrors.New(mtbferrors.Err3310, err))
	}

	s.Log("Random seeking")
	if err = media.VerifyRandomSeekingElement(ctx, conn, 5, videoPlayer); err != nil {
		s.Fatal(mtbferrors.New(mtbferrors.Err3319, err))
	}

	if err = media.VerifyPlayingElement(ctx, conn, 5*time.Second, videoPlayer); err != nil {
		s.Fatal(mtbferrors.New(mtbferrors.Err3300, err, videoURL))
	}
}
