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
		Func: PlayDecodeAccelVideoSeekSwitchVP8,
		Desc: "Verifies that VP8 resolution changing seek works in Chrome",
		Attr: []string{"informational"},
		SoftwareDeps: []string{caps.HWDecodeVP8, "chrome_login"},
		Data:         []string{"frame_size_change.webm", "video.html"},
	})
}

// TODO this test should not be performed on Nyan due to crbug.com/699260.

func PlayDecodeAccelVideoSeekSwitchVP8(ctx context.Context, s *testing.State) {
	play.TestSeek(ctx, s, "frame_size_change.webm", play.CheckHistogram)
}
