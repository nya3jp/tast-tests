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
		Func:     SeekVD,
		Desc:     "Verifies that seeking works in Chrome when using a media::VideoDecoder, either with or without resolution changes",
		Contacts: []string{"acourbot@chromium.org", "chromeos-video-eng@google.com"},
		Attr:     []string{"group:mainline", "informational"},
		// "chrome_internal" is needed because H.264 is a proprietary codec.
		SoftwareDeps: []string{"cros_video_decoder", "chrome", "chrome_internal"},
		Pre:          pre.ChromeVideoVD(),
		Data:         []string{"video.html"},

		Params: []testing.Param{{
			Name:      "h264",
			Val:       "video_seek.mp4",
			ExtraData: []string{"video_seek.mp4"},
		}, {
			Name:      "switch_h264",
			Val:       "switch_1080p_720p.mp4",
			ExtraData: []string{"switch_1080p_720p.mp4"},
		}},
	})
}

// SeekVD plays a video in the Chrome browser and checks that it can safely be
// seeked into while using a media::VideoDecoder (see go/vd-migration).
func SeekVD(ctx context.Context, s *testing.State) {
	const numSeeks = 25
	play.TestSeek(ctx, s, s.PreValue().(*chrome.Chrome), s.Param().(string), numSeeks)
}
