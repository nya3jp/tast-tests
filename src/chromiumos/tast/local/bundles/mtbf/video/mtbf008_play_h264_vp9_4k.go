// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/local/chrome"
	mtbfchrome "chromiumos/tast/local/mtbf/chrome"
	"chromiumos/tast/local/mtbf/dom"
	"chromiumos/tast/testing"
)

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

	conn, err := mtbfchrome.NewConn(ctx, cr, videoURL)
	if err != nil {
		s.Fatal("MTBF failed: ", err)
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
		if err = conn.Exec(ctx, javascript); err != nil {
			s.Error(mtbferrors.New(mtbferrors.VideoNoPlay, err, videoURL))
		}
		testing.Sleep(ctx, 10*time.Second) // Wait for video to change quality and keeps playing...
		s.Log("Verify frame drops is zero")
		if mtbferr := verifyFramedrops(ctx, conn, 10*time.Second); mtbferr != nil {
			s.Error(mtbferr)
		}
	}

	testing.Sleep(ctx, 5*time.Second)
}

// verifyFramedrops verifies that no frames are dropped every second during a given time duration.
func verifyFramedrops(ctx context.Context, conn *chrome.Conn, timeout time.Duration) error {
	currentTime, err := dom.GetElementCurrentTime(ctx, conn, "video")
	if err != nil {
		return mtbferrors.New(mtbferrors.VideoGetTime, err)
	}
	var interval, time time.Duration = 1 * time.Second, 0

	totalFramedrops := 0
	for time <= timeout {
		if err = testing.Sleep(ctx, interval); err != nil {
			return err
		}
		time += interval

		framedrops, err := getFramedrops(ctx, conn)
		if err != nil {
			return mtbferrors.New(mtbferrors.VideoGetFrmDrop, err)
		}
		totalFramedrops = framedrops
	}

	const (
		minorFrameDropThreshold  = 1
		majorFrameDropThreshold  = 2
		severeFrameDropThreshold = 24
	)

	if totalFramedrops >= severeFrameDropThreshold {
		return mtbferrors.New(mtbferrors.VideoSevereFramedrops, nil, currentTime, totalFramedrops, timeout.Seconds())
	} else if totalFramedrops >= majorFrameDropThreshold {
		return mtbferrors.New(mtbferrors.VideoMajorFramedrops, nil, currentTime, totalFramedrops, timeout.Seconds())
	} else if totalFramedrops >= minorFrameDropThreshold {
		return mtbferrors.New(mtbferrors.VideoMinorFramedrops, nil, currentTime, totalFramedrops, timeout.Seconds())
	}

	return nil
}

// getFramedrops gets the number of frames dropped by executing  javascript code.
func getFramedrops(ctx context.Context, conn *chrome.Conn) (framedrops int, err error) {
	getFrameDropScript := "parseInt(document.querySelector('#droppedFramesDebug').innerText);"
	conn.Eval(ctx, getFrameDropScript, &framedrops)
	return
}
