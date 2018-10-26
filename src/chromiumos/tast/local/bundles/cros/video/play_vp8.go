// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"

	"chromiumos/tast/local/bundles/cros/video/play"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PlayVP8,
		Desc:         "Checks VP8 video playback is working",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome_login"},
		Data:         []string{"bear_vp8_320x180.webm", "video.html"},
	})
}

// PlayVP8 plays bear_h264_320x180.mp4 with Chrome.
func PlayVP8(ctx context.Context, s *testing.State) {
	play.TestPlay(ctx, s, "bear_vp8_320x180.webm")
}
