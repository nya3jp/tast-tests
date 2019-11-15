// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"

	"chromiumos/tast/local/bundles/cros/video/play"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/media/pre"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PlayVD,
		Desc:         "Checks whether video playback is working when using a media::VideoDecoder (see go/vd-migration)",
		Contacts:     []string{"dstaessens@chromium.org", "akahuang@chromium.org", "chromeos-video-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"cros_video_decoder", "chrome"},
		Data:         []string{"video.html"},
		Pre:          pre.ChromeVideoVD(),
		Params: []testing.Param{{
			Name:      "h264",
			Val:       "720_h264.mp4",
			ExtraData: []string{"720_h264.mp4"},
			// "chrome_internal" is needed because H.264 is a proprietary codec.
			ExtraSoftwareDeps: []string{"chrome_internal"},
		}, {
			Name:      "vp8",
			Val:       "720_vp8.webm",
			ExtraData: []string{"720_vp8.webm"},
		}, {
			Name:      "vp9",
			Val:       "720_vp9.webm",
			ExtraData: []string{"720_vp9.webm"},
		}},
	})
}

func PlayVD(ctx context.Context, s *testing.State) {
	play.TestPlay(ctx, s, s.PreValue().(*chrome.Chrome),
		s.Param().(string), play.NormalVideo, play.NoVerifyHWAcceleratorUsed)
}
