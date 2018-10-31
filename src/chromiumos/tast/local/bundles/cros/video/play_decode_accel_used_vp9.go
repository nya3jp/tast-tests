// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"

	"chromiumos/tast/local/bundles/cros/video/lib/caps"
	"chromiumos/tast/local/bundles/cros/video/play"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PlayDecodeAccelUsedVP9,
		Desc:         "Verifies that VP9 video decode acceleration works in Chrome",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{caps.HWDecodeVP9, "chrome_login"},
		Data:         []string{"bear_vp9_320x240.webm", "video.html"},
	})
}

// PlayDecodeAccelUsedVP9 plays bear_vp9_320x240.webm with Chrome and
// checks if video decode accelerator was used.
func PlayDecodeAccelUsedVP9(ctx context.Context, s *testing.State) {
	play.TestPlay(ctx, s, "bear_vp9_320x240.webm", play.CheckHistogram)
}
