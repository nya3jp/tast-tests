// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/video/playback"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PlaybackPerfH264,
		Desc:         "Measure video playback performance with/without HW acceleration",
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome_login", "chrome_internal"},
		Data:         []string{"traffic-1920x1080-8005020218f6b86bfa978e550d04956e.mp4"},
		// Default timeout (i.e. 2 minutes) is not enough for low-end devices.
		Timeout: 3 * time.Minute,
	})
}

// PlaybackPerfH264 plays H264 1080p 30 fps video and measures the peformance values with/without
// HW decoding acceleration. The values are reported to performance dashboard.
func PlaybackPerfH264(ctx context.Context, s *testing.State) {
	playback.RunTest(ctx, s, "traffic-1920x1080-8005020218f6b86bfa978e550d04956e.mp4", "h264_1080p")
}
