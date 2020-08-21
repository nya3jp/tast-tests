// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/local/bundles/mtbf/video/common"
	"chromiumos/tast/local/bundles/mtbf/video/youtube"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MTBF030CloseChrome,
		Desc:         "Verify youtube video is stoped",
		Contacts:     []string{"xliu@cienet.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"cros_video_decoder", "chrome", "chrome_internal"},
		Vars:         []string{"video.youtubeVideo"},
		Pre:          chrome.LoginReuse(),
	})
}

// MTBF030CloseChrome close chrome
func MTBF030CloseChrome(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	videoURL := common.GetVar(ctx, s, "video.youtubeVideo")
	conn, err := cr.NewConnForTarget(ctx, chrome.MatchTargetURL(youtube.Add1SecondForURL(videoURL)))
	if err != nil {
		s.Fatal(mtbferrors.New(mtbferrors.ChromeExistTarget, err, videoURL))
	}
	defer conn.Close()
	defer conn.CloseTarget(ctx)
	s.Log("Chrome is now closed")
}
