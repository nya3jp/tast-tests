// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"

	"chromiumos/tast/local/bundles/cros/video/lib/caps"
	"chromiumos/tast/local/bundles/cros/video/play"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PlayDecodeAccelUsedMSEVP8,
		Desc:         "Verifies that VP8 video decode acceleration works when MSE is used",
		Contacts:     []string{"keiichiw@chromium.org", "chromeos-video-eng@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{caps.HWDecodeVP8, "chrome_login"},
		Data: append(
			play.MSEDataFiles(),
			"bear-320x240-video-only.vp8.webm",
			"bear-320x240-audio-only.vorbis.webm",
			"bear-320x240.vp8.mpd",
		),
	})
}

// PlayDecodeAccelUsedMSEVP8 plays a VP8 video stream by shaka player, which uses
// Media Source Extensions (MSE).
// After that, it checks if video decode accelerator was used.
func PlayDecodeAccelUsedMSEVP8(ctx context.Context, s *testing.State) {
	play.TestPlay(ctx, s, "bear-320x240.vp8.mpd", play.MSEVideo, play.CheckHistogram)
}
