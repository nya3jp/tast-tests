// Copyright 2018 The Chromium OS Authors. All rights reserved.
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
		Func:     PlayDecodeAccelUsedMSEH264,
		Desc:     "Verifies that H264 video decode acceleration works when MSE is used",
		Contacts: []string{"deanliao@chromium.org", "chromeos-video-eng@google.com"},
		// "chrome_internal" is needed because H.264 is a proprietary codec.
		SoftwareDeps: []string{caps.HWDecodeH264, "chrome", "chrome_internal"},
		Pre:          pre.ChromeVideo(),
		Data: append(
			play.MSEDataFiles(),
			"bear-320x240-video-only.h264.mp4",
			"bear-320x240-audio-only.aac.mp4",
			"bear-320x240.h264.mpd",
		),
		// Marked informational due to flakiness on ToT.
		// TODO(crbug.com/1008317): Promote to critical again.
		Attr: []string{"group:mainline", "informational"},
	})
}

// PlayDecodeAccelUsedMSEH264 plays a H264 video stream by shaka player, which uses
// Media Source Extensions (MSE).
// After that, it checks if video decode accelerator was used.
func PlayDecodeAccelUsedMSEH264(ctx context.Context, s *testing.State) {
	play.TestPlay(ctx, s, s.PreValue().(*chrome.Chrome),
		"bear-320x240.h264.mpd", play.MSEVideo, play.CheckHistogram)
}
