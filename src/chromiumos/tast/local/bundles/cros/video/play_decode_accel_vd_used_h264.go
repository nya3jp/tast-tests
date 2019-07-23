// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"

	"chromiumos/tast/local/bundles/cros/video/play"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/local/media/pre"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     PlayDecodeAccelVDUsedH264,
		Desc:     "Verifies that H.264 video decode acceleration works in Chrome when using a media::VideoDecoder",
		Contacts: []string{"deanliao@chromium.org", "chromeos-video-eng@google.com"},
		Attr:     []string{"informational"},
		// "chrome_internal" is needed because H.264 is a proprietary codec.
		SoftwareDeps: []string{caps.HWDecodeH264, "chrome", "chrome_internal"},
		Data:         []string{"720_h264.mp4", "video.html"},
		Pre:          pre.ChromeVideoVD(),
	})
}

// PlayDecodeAccelVDUsedH264 plays 720_h264.mp4 with Chrome and checks if a
// media::VideoDecoder was used (see go/vd-migration).
func PlayDecodeAccelVDUsedH264(ctx context.Context, s *testing.State) {
	play.TestPlay(ctx, s, s.PreValue().(*chrome.Chrome),
		"720_h264.mp4", play.NormalVideo, play.CheckHistogram)
}
