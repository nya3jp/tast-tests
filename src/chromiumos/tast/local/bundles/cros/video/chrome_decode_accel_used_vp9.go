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
		Func:         ChromeDecodeAccelUsedVP9,
		Desc:         "Verifies that VP9 video decode acceleration works in Chrome.",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome_login", "autotest-capability:hw_dec_vp9_1080_30"},
		Data:         []string{"bear_vp9_320x240.webm", "video.html"},
	})
}

// ChromeDecodeAccelUsedVP9 plays bear_vp9_320x240.webm with Chrome and
// checks if video decode accelerator was used.
func ChromeDecodeAccelUsedVP9(ctx context.Context, s *testing.State) {
	play.ChromeDecodeAccelUsed(ctx, s, "bear_vp9_320x240.webm")
}
