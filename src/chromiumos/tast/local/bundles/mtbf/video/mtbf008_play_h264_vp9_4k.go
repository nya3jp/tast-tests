// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/local/bundles/mtbf/video/media"
	"chromiumos/tast/local/chrome"
	mtbfchrome "chromiumos/tast/local/mtbf/chrome"
	"chromiumos/tast/local/mtbf/dom"
	"chromiumos/tast/testing"
)

const videoSelector = "video"

func init() {
	testing.AddTest(&testing.Test{
		Func:         MTBF008PlayH264VP94K,
		Desc:         "Playing H264 & VP9 30 FPS videos properly in 2K & 4K resolution",
		Contacts:     []string{"xliu@cienet.com"},
		SoftwareDeps: []string{"chrome", "chrome_internal", "cros_video_decoder", "vp9_sanity"},
		Pre:          chrome.LoginReuse(),
		Attr:         []string{"group:mainline", "informational"},
		Params: []testing.Param{{
			Name: "vp9",
			Val:  "VP9 DASH 30 FPS",
		}, {
			Name: "h264",
			Val:  "H264 DASH 30 FPS",
		}},
	})
}

// MTBF008PlayH264VP94K case tests if H264 & VP9 30 FPS videos play properly in 2K & 4K resolution.
func MTBF008PlayH264VP94K(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	const videoURL = "https://crosvideo.appspot.com/"
	videoType := s.Param().(string)

	conn, mtbferr := mtbfchrome.NewConn(ctx, cr, videoURL)
	if mtbferr != nil {
		s.Fatal(mtbferr)
	}
	defer conn.Close()
	defer conn.CloseTarget(ctx)
	s.Logf("%s video is now ready for 4K/2K playing", videoType)

	for _, quality := range []string{
		"2560x1440, 7075213 bits/s",
		"3840x2160, 23585388 bits/s",
	} {
		s.Logf("Change video quality to %s", quality)
		javascript := fmt.Sprintf(`((video, tracks) => {
			document.querySelector("#mpdList").value = Array.from(document.querySelector("#mpdList").options).find(o => o.label === video).value;
			document.querySelector("#videoTracks").value = tracks;
			loadStream();
		})("%s", "%s");`, videoType, quality)
		if err := conn.Exec(ctx, javascript); err != nil {
			s.Fatal(mtbferrors.New(mtbferrors.VideoPlayFailed, err, videoURL))
		}
		// Wait for video to change quality...
		if err := dom.WaitForReadyState(ctx, conn, videoSelector, 10*time.Second, 100*time.Millisecond); err != nil {
			s.Fatal(mtbferrors.New(mtbferrors.VideoReadyStatePoll, err))
		}
		s.Log("Verify frame drops is zero")
		if mtbferr := verifyFramedrops(ctx, conn, 10*time.Second); mtbferr != nil {
			s.Fatal(mtbferr)
		}
	}

	testing.Sleep(ctx, 5*time.Second)
}

// verifyFramedrops verifies that no frames are dropped every second during a given time duration.
func verifyFramedrops(ctx context.Context, conn *chrome.Conn, timeout time.Duration) error {
	return media.CheckFramedrops(ctx, conn, timeout, 30, videoSelector, getFramedrops)
}

// getFramedrops gets the number of frames dropped by executing  javascript code.
func getFramedrops(ctx context.Context, conn *chrome.Conn) (framedrops int, err error) {
	getFrameDropScript := "parseInt(document.querySelector('#droppedFramesDebug').innerText);"
	conn.Eval(ctx, getFrameDropScript, &framedrops)
	return
}
