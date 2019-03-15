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
		Func:         PlayDecodeAccelUsedMSEVP9,
		Desc:         "Verifies that VP9 video decode acceleration works when MSE is used",
		Contacts:     []string{"keiichiw@chromium.org", "chromeos-video-eng@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{caps.HWDecodeVP9, "chrome_login"},
		Pre:          pre.LoggedInVideo(),
		Data: append(
			play.MSEDataFiles(),
			"bear-320x240-video-only.vp9.webm",
			"bear-320x240-audio-only.opus.webm",
			"bear-320x240.vp9.mpd",
		),
	})
}

// PlayDecodeAccelUsedMSEVP9 plays a VP9 video stream by shaka player, which uses
// Media Source Extensions (MSE).
// After that, it checks if video decode accelerator was used.
func PlayDecodeAccelUsedMSEVP9(ctx context.Context, s *testing.State) {
	play.TestPlay(ctx, s, s.PreValue().(*chrome.Chrome),
		"bear-320x240.vp9.mpd", play.MSEVideo, play.CheckHistogram)
}
