// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"

	"chromiumos/tast/local/bundles/cros/video/lib/caps"
	"chromiumos/tast/local/bundles/cros/video/lib/pre"
	"chromiumos/tast/local/bundles/cros/video/play"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     PlayDecodeAccelUsedMSEH264,
		Desc:     "Verifies that H264 video decode acceleration works when MSE is used",
		Contacts: []string{"keiichiw@chromium.org", "chromeos-video-eng@google.com"},
		Attr:     []string{"informational"},
		// "chrome_internal" is needed because H.264 is a proprietary codec.
		SoftwareDeps: []string{caps.HWDecodeH264, "chrome_login", "chrome_internal"},
		Pre:          pre.LoggedInVideo(),
		Data: append(
			play.MSEDataFiles(),
			"bear-320x240-video-only.h264.mp4",
			"bear-320x240-audio-only.aac.mp4",
			"bear-320x240.h264.mpd",
		),
	})
}

// PlayDecodeAccelUsedMSEH264 plays a H264 video stream by shaka player, which uses
// Media Source Extensions (MSE).
// After that, it checks if video decode accelerator was used.
func PlayDecodeAccelUsedMSEH264(ctx context.Context, s *testing.State) {
	play.TestPlay(ctx, s, s.PreValue().(*chrome.Chrome),
		"bear-320x240.h264.mpd", play.MSEVideo, play.CheckHistogram)
}
