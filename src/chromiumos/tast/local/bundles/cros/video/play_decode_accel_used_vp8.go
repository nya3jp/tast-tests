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
		Func:         PlayDecodeAccelUsedVP8,
		Desc:         "Verifies that VP8 video decode acceleration works in Chrome",
		Contacts:     []string{"deanliao@chromium.org", "chromeos-video-eng@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{caps.HWDecodeVP8, "chrome"},
		Pre:          pre.ChromeVideo(),
		Data:         []string{"bear-320x240.vp8.webm", "video.html"},
	})
}

// PlayDecodeAccelUsedVP8 plays bear-320x240.vp8.webm with Chrome and
// checks if video decode accelerator was used.
func PlayDecodeAccelUsedVP8(ctx context.Context, s *testing.State) {
	play.TestPlay(ctx, s, s.PreValue().(*chrome.Chrome),
		"bear-320x240.vp8.webm", play.NormalVideo, play.CheckHistogram)
}
