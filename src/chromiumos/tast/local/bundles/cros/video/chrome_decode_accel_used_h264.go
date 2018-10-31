// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"

	"chromiumos/tast/local/bundles/cros/video/lib/play"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ChromeDecodeAccelUsedH264,
		Desc:         "Verifies that H.264 video decode acceleration works in Chrome.",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome_login", "autotest-capability:hw_dec_h264_1080_30"},
		Data:         []string{"bear_h264_320x180.mp4", "video.html"},
	})
}

// ChromeDecodeAccelUsedH264 plays bear_h264_320x180.mp4 with Chrome and
// checks if video decode accelerator was used.
func ChromeDecodeAccelUsedH264(ctx context.Context, s *testing.State) {
	play.ChromeDecodeAccelUsed(ctx, s, "bear_h264_320x180.mp4")
}
