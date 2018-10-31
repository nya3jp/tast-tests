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
		Func:         ChromeDecodeAccelUsedVP8MSE,
		Desc:         "Verifies that VP8 video decode acceleration works for MSE videos in Chrome.",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome_login", "autotest-capability:hw_dec_vp8_1080_30"},
	})
}

// ChromeDecodeAccelUsedVP8MSE plays VP8 video on http://crosvideo.appspot.com and
// checks if video decode accelerator was used.
func ChromeDecodeAccelUsedVP8MSE(ctx context.Context, s *testing.State) {
	play.CrosVideo(ctx, s, "vp8")
}
