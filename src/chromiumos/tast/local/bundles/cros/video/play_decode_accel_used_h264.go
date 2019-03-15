// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"

	"chromiumos/tast/local/bundles/cros/video/lib/caps"
	"chromiumos/tast/local/bundles/cros/video/play"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     PlayDecodeAccelUsedH264,
		Desc:     "Verifies that H.264 video decode acceleration works in Chrome",
		Contacts: []string{"keiichiw@chromium.org", "chromeos-video-eng@google.com"},
		Attr:     []string{"informational"},
		// "chrome_internal" is needed because H.264 is a proprietary codec.
		SoftwareDeps: []string{caps.HWDecodeH264, "chrome_login", "chrome_internal"},
		Data:         []string{"bear-320x240.h264.mp4", "video.html"},
		Pre:          chrome.LoggedInVideo(),
	})
}

// PlayDecodeAccelUsedH264 plays bear-320x240.h264.mp4 with Chrome and
// checks if video decode accelerator was used.
func PlayDecodeAccelUsedH264(ctx context.Context, s *testing.State) {
	play.TestPlay(ctx, s, s.PreValue().(*chrome.Chrome),
		"bear-320x240.h264.mp4", play.NormalVideo, play.CheckHistogram)
}
