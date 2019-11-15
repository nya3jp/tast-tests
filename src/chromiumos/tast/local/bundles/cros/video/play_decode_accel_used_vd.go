// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"

	"chromiumos/tast/local/bundles/cros/video/decode"
	"chromiumos/tast/local/bundles/cros/video/play"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/local/media/pre"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PlayDecodeAccelUsedVD,
		Desc:         "Verifies that video decode acceleration works in Chrome when using a media::VideoDecoder",
		Contacts:     []string{"akahuang@chromium.org", "dstaessens@chromium.org", "chromeos-video-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"cros_video_decoder", "chrome"},
		Data:         []string{"video.html", decode.ChromeMediaInternalsUtilsJSFile},
		Pre:          pre.ChromeVideoVD(),
		Params: []testing.Param{{
			Name: "h264",
			Val:  "720_h264.mp4",
			// "chrome_internal" is needed because H.264 is a proprietary codec.
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "chrome_internal"},
			ExtraData:         []string{"720_h264.mp4"},
		}, {
			Name:              "vp8",
			Val:               "720_vp8.webm",
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8},
			ExtraData:         []string{"720_vp8.webm"},
		}, {
			Name:              "vp9",
			Val:               "720_vp9.webm",
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
			ExtraData:         []string{"720_vp9.webm"},
		}},
	})
}

// PlayDecodeAccelUsedVD plays a video in the Chrome browser and checks if a
// media::VideoDecoder was used (see go/vd-migration).
func PlayDecodeAccelUsedVD(ctx context.Context, s *testing.State) {
	play.TestPlay(ctx, s, s.PreValue().(*chrome.Chrome),
		s.Param().(string), play.NormalVideo, play.VerifyHWAcceleratorUsed)
}
