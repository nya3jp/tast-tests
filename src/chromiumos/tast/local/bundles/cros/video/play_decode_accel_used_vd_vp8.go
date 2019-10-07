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
		Func:         PlayDecodeAccelUsedVDVP8,
		Desc:         "Verifies that VP8 video decode acceleration works in Chrome when using a media::VideoDecoder",
		Contacts:     []string{"akahuang@chromium.org", "dstaessens@chromium.org", "chromeos-video-eng@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{caps.HWDecodeVP8, "chrome"},
		Data:         []string{"720_vp8.webm", "video.html"},
		Pre:          pre.ChromeVideoVD(),
	})
}

// PlayDecodeAccelUsedVDVP8 plays 720_vp8.webm with Chrome and checks if a
// media::VideoDecoder was used (see go/vd-migration).
func PlayDecodeAccelUsedVDVP8(ctx context.Context, s *testing.State) {
	play.TestPlay(ctx, s, s.PreValue().(*chrome.Chrome),
		"720_vp8.webm", play.NormalVideo, play.CheckHistogram)
}
