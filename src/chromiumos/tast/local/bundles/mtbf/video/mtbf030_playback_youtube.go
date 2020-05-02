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
		Func:         MTBF030PlaybackYoutube,
		Desc:         "Play youtube video",
		Contacts:     []string{"xliu@cienet.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"cros_video_decoder", "chrome", "chrome_internal"},
		Vars:         []string{"video.youtubeVideo"},
		Pre:          chrome.LoginReuse(),
	})
}

// MTBF030PlaybackYoutube plays mp4 video for 1080/720 resolution.
func MTBF030PlaybackYoutube(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	videoURL := common.GetVar(ctx, s, "video.youtubeVideo")
	conn, mtbferr := mtbfchrome.NewConn(ctx, cr, youtube.Add1SecondForURL(videoURL))
	if mtbferr != nil {
		s.Fatal(mtbferr)
	}
	defer conn.Close()
	s.Log("Document is ready")
	youtube.PlayVideo(ctx, conn)

	s.Log("Play for 5 more seconds")
	testing.Sleep(ctx, 5*time.Second)
}
