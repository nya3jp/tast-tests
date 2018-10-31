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
		Func:         ChromeDecodeAccelUsedVP8,
		Desc:         "Verifies that VP8 video decode acceleration works in Chrome.",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome_login", "autotest-capability:hw_dec_vp8_1080_30"},
		Data:         []string{"bear_vp8_320x180.webm", "video.html"},
	})
}

// ChromeDecodeAccelUsedVP8 plays bear_vp8_320x180.webm with Chrome and
// checks if video decode accelerator was used.
func ChromeDecodeAccelUsedVP8(ctx context.Context, s *testing.State) {
	play.ChromeDecodeAccelUsed(ctx, s, "bear_vp8_320x180.webm")
}
