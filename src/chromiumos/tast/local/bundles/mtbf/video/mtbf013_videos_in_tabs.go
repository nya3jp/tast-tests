// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/mtbf/video/common"
	"chromiumos/tast/local/bundles/mtbf/video/vimeo"
	"chromiumos/tast/local/bundles/mtbf/video/youtube"
	"chromiumos/tast/local/chrome"
	mtbfchrome "chromiumos/tast/local/mtbf/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MTBF013VideosInTabs,
		Desc:         "VideosInTabs(MTBF013): Play multiple videos in different tabs",
		Contacts:     []string{"xliu@cienet.com"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Pre:          chrome.LoginReuse(),
		Vars:         []string{"video.diffTabsVideo1", "video.diffTabsVideo3"},
	})
}

// MTBF013VideosInTabs case verifies that multiple videos in different tabs can be played at once.
// Set up the two Chrome tabs. Load one tab and start the video.
// Load the second tab and start the video. Verify both videos are playing.
func MTBF013VideosInTabs(ctx context.Context, s *testing.State) {
	url1 := common.GetVar(ctx, s, "video.diffTabsVideo1")
	url2 := common.GetVar(ctx, s, "video.diffTabsVideo3")
	cr := s.PreValue().(*chrome.Chrome)

	s.Log("Open video urls")
	conn1, mtbferr := mtbfchrome.NewConn(ctx, cr, youtube.Add1SecondForURL(url1))
	if mtbferr != nil {
		s.Fatal(mtbferr)
	}
	defer conn1.Close()
	defer conn1.CloseTarget(ctx)

	testing.Sleep(ctx, 3*time.Second)
	if mtbferr := youtube.PlayVideo(ctx, conn1); mtbferr != nil {
		s.Fatal(mtbferr)
	}

	conn2, mtbferr := mtbfchrome.NewConn(ctx, cr, url2)
	if mtbferr != nil {
		s.Fatal(mtbferr)
	}
	defer conn2.Close()
	defer conn2.CloseTarget(ctx)

	testing.Sleep(ctx, 3*time.Second)
	if mtbferr := vimeo.PlayVideo(ctx, conn2); mtbferr != nil {
		s.Fatal(mtbferr)
	}

	if mtbferr := youtube.IsPlaying(ctx, conn1, time.Second*5); mtbferr != nil {
		s.Fatal(mtbferr)
	}

	if mtbferr := vimeo.IsPlaying(ctx, conn2, time.Second*5); mtbferr != nil {
		s.Fatal(mtbferr)
	}
}
