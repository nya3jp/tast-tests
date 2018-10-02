// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"

	"chromiumos/tast/local/bundles/cros/video/playback"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PlaybackPerfH264,
		Desc:         "Measure video playback performance with/without HW acceleration",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome_login"},
		Data:         []string{"traffic-1920x1080-8005020218f6b86bfa978e550d04956e.mp4"},
	})
}

// PlaybackPerfH264 plays H264 1080p 30 fps video and measures the peformance values with/without
// HW decoding acceleration. The values are reported to performance dashboard.
func PlaybackPerfH264(ctx context.Context, s *testing.State) {
	playback.RunTest(ctx, s, "traffic-1920x1080-8005020218f6b86bfa978e550d04956e.mp4", "h264_1080p")
}
