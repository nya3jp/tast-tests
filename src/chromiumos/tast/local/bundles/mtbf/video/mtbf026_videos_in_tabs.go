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
		Func:         MTBF026VideosInTabs,
		Desc:         "VideosInTabs(MTBF026): Play multiple videos in different tabs",
		Contacts:     []string{"xliu@cienet.com"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Pre:          chrome.LoginReuse(),
		Vars:         []string{"video.diffTabsVideo1", "video.diffTabsVideo2"},
	})
}

// MTBF026VideosInTabs case verifies that multiple videos in different tabs can be played at once.
// Set up the two Chrome tabs. Load one tab and start the video.
// Load the second tab and start the video. Verify both videos are playing.
func MTBF026VideosInTabs(ctx context.Context, s *testing.State) {
	url1, ok := s.Var("video.diffTabsVideo1")
	if !ok {
		s.Fatal(mtbferrors.New(mtbferrors.OSVarRead, nil, "video.diffTabsVideo1"))
	}

	url2, ok := s.Var("video.diffTabsVideo2")
	if !ok {
		s.Fatal(mtbferrors.New(mtbferrors.OSVarRead, nil, "video.diffTabsVideo2"))
	}

	cr := s.PreValue().(*chrome.Chrome)

	s.Log("Open video urls")
	conn1, err := mtbfchrome.NewConn(ctx, cr, youtube.Add1SecondForURL(url1))
	if err != nil {
		s.Fatal("MTBF failed: ", err)
	}
	defer conn1.Close()
	defer conn1.CloseTarget(ctx)

	testing.Sleep(ctx, 3*time.Second)
	if err := youtube.PlayVideo(ctx, conn1); err != nil {
		s.Fatal(mtbferrors.New(mtbferrors.ChromeExeJs, err, "OpenAndPlayVideo"))
	}

	conn2, err := mtbfchrome.NewConn(ctx, cr, youtube.Add1SecondForURL(url2))
	if err != nil {
		s.Fatal("MTBF failed: ", err)
	}
	defer conn2.Close()
	defer conn2.CloseTarget(ctx)

	testing.Sleep(ctx, 3*time.Second)
	if err := youtube.PlayVideo(ctx, conn2); err != nil {
		s.Fatal(mtbferrors.New(mtbferrors.ChromeExeJs, err, "OpenAndPlayVideo"))
	}

	if err := youtube.IsPlaying(ctx, conn1, time.Second*5); err != nil {
		s.Fatal("MTBF failed: ", err)
	}

	if err := youtube.IsPlaying(ctx, conn2, time.Second*5); err != nil {
		s.Fatal("MTBF failed: ", err)
	}
}
