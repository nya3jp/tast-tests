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
		Func: PlayDecodeAccelVideoSeekSwitchH264,
		Desc: "Verifies that H.264 resolution changing seek works in Chrome",
		Attr: []string{"informational"},
		// "chrome_internal" is needed because H.264 is a proprietary codec.
		SoftwareDeps: []string{caps.HWDecodeH264, "chrome_login", "chrome_internal"},
		Data:         []string{"switch_1080p_720p.mp4", "video.html"},
	})
}

func PlayDecodeAccelVideoSeekSwitchH264(ctx context.Context, s *testing.State) {
	play.TestSeek(ctx, s, "switch_1080p_720p.mp4", play.CheckHistogram)
}
