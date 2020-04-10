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
	"chromiumos/tast/local/mtbf/audio"
	mtbfchrome "chromiumos/tast/local/mtbf/chrome"
	mtbfFilesapp "chromiumos/tast/local/mtbf/ui/filesapp"
	"chromiumos/tast/local/ui/filesapp"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MTBF028AudioFocus,
		Desc:         "Plays youtube video and then plays m4a files",
		Contacts:     []string{"xliu@cienet.com"},
		Attr:         []string{"group:mainline", "informational"},
		Pre:          chrome.LoginReuse(),
		SoftwareDeps: []string{"chrome", "chrome_internal", "cros_video_decoder"},
		Data:         []string{"audio.m4a"},
	})
}

// MTBF028AudioFocus case plays youtube video and .m4a audio file to verify their behavior.
// Load Youtube video and begin playing. Load .m4a audio file and begin playing. Verify Youtube video pauses.
// Start Youtube video playing again. Verify audio file pauses.
func MTBF028AudioFocus(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	const videoURL = "https://www.youtube.com/watch?v=txTqtm58AqM"
	s.Log("Starting to jump to youtube url")
	conn, err := mtbfchrome.NewConn(ctx, cr, youtube.Add1SecondForURL(videoURL))
	if err != nil {
		s.Fatal("MTBF failed: ", err)
	}
	defer conn.Close()
	defer conn.CloseTarget(ctx)

	s.Log("Youtube video is now ready for playing")
	testing.Sleep(ctx, 10*time.Second)

	s.Log("Preparing m4a to play")
	audioFile := "audio.m4a"

	s.Log("Open the test API")
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal(mtbferrors.New(mtbferrors.ChromeTestConn, err))
	}
	defer tconn.Close()
	defer tconn.CloseTarget(ctx)

	s.Log("Open the Files App")
	files, mtbferr := mtbfFilesapp.Launch(ctx, tconn)
	if mtbferr != nil {
		s.Fatal(mtbferr)
	}
	defer filesapp.Close(ctx, tconn)

	if mtbferr := audio.PlayFromDownloadsFolder(ctx, files, s.DataPath(audioFile), audioFile); mtbferr != nil {
		s.Fatal(mtbferr)
	}
	defer audio.ClickButton(ctx, tconn, "Close")

	testing.Sleep(ctx, 5*time.Second)

	s.Log("Verify youtube has been paused")
	if err := youtube.IsPausing(ctx, conn, 3*time.Second); err != nil {
		s.Error(mtbferrors.New(mtbferrors.VideoYTPause, err))
	}

	testing.Sleep(ctx, 5*time.Second)

	s.Log("Play youtube and verify audio player paused")
	if err := youtube.PlayVideo(ctx, conn); err != nil {
		s.Error(mtbferrors.New(mtbferrors.VideoNotPlay, nil, videoURL))
	}

	s.Log("Verify m4a has been paused")
	if mtbferr = audio.IsPausing(ctx, tconn, 3*time.Second); mtbferr != nil {
		s.Error(mtbferr)
	}

	testing.Sleep(ctx, 10*time.Second)
}
