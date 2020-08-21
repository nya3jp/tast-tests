// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/mtbf/video"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MTBF018VideoCorrupt,
		Desc:         "VideoHWCorruptedFile(MTBF018): Automated-test: video_VideoCorruption",
		Contacts:     []string{"xliu@cienet.com"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		Attr:         []string{"group:mainline", "informational"},
		Pre:          chrome.LoginReuse(),
		Data:         []string{"corruptedVideo.mp4"},
	})
}

// MTBF018VideoCorrupt case verifies an attempt to play a corrupted audio/video file does not cause a crash.
func MTBF018VideoCorrupt(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	s.Log("Preparing video to play")
	videoFile := "corruptedVideo.mp4"

	videoplayer, mtbferr := video.NewPlayer(ctx, cr)
	if mtbferr != nil {
		s.Fatal(mtbferr)
	}
	defer videoplayer.Close(ctx)

	if mtbferr := videoplayer.StartToPlay(ctx, videoFile, s.DataPath(videoFile)); mtbferr != nil {
		s.Fatal(mtbferr)
	}

	testing.Sleep(ctx, time.Second)

	if mtbferr := videoplayer.IsPausing(ctx, time.Second*5); mtbferr != nil {
		s.Fatal(mtbferr)
	}
}
