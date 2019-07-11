// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"

	"chromiumos/tast/local/bundles/cros/video/lib/pre"
	"chromiumos/tast/local/bundles/cros/video/play"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     Play,
		Desc:     "Checks video playback is working",
		Contacts: []string{"deanliao@chromium.org", "chromeos-video-eng@google.com"},
		Attr:     []string{"informational"},
		// "chrome_internal" is needed because H.264 is a proprietary codec.
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		Pre:          pre.ChromeVideo(),
		Data:         []string{"video.html"},
		Params: []testing.Param{{
			Name:      "h264",
			ExtraData: []string{"720_h264.mp4"},
			Val:       "720_h264.mp4",
		}, {
			Name:      "vp8",
			ExtraData: []string{"720_vp8.webm"},
			Val:       "720_vp8.webm",
		}, {
			Name:      "vp9",
			ExtraData: []string{"720_vp9.webm"},
			Val:       "720_vp9.webm",
		}},
	})
}

func Play(ctx context.Context, s *testing.State) {
	play.TestPlay(ctx, s, s.PreValue().(*chrome.Chrome),
		s.Param().(string), play.NormalVideo, play.NoCheckHistogram)
}
