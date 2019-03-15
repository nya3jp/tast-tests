// Copyright 2018 The Chromium OS Authors. All rights reserved.
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
		Func:     PlayH264,
		Desc:     "Checks H264 video playback is working",
		Contacts: []string{"keiichiw@chromium.org", "chromeos-video-eng@google.com"},
		Attr:     []string{"informational"},
		// "chrome_internal" is needed because H.264 is a proprietary codec.
		SoftwareDeps: []string{"chrome_login", "chrome_internal"},
		Pre:          pre.LoggedInVideo(),
		Data:         []string{"bear-320x240.h264.mp4", "video.html"},
	})
}

// PlayH264 plays bear-320x240.h264.mp4 with Chrome.
func PlayH264(ctx context.Context, s *testing.State) {
	play.TestPlay(ctx, s, s.PreValue().(*chrome.Chrome),
		"bear-320x240.h264.mp4", play.NormalVideo, play.NoCheckHistogram)
}
