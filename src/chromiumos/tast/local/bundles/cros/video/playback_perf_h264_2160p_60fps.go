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
		Func:     PlaybackPerfH2642160P60FPS,
		Desc:     "Measures video playback performance with/without HW acceleration for H264 2160p@60fps video",
		Contacts: []string{"hiroh@chromium.org", "chromeos-video-eng@google.com"},
		Attr:     []string{"group:crosbolt", "crosbolt_perbuild"},
		// "chrome_internal" is needed because H.264 is a proprietary codec.
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		Data:         []string{"2160p_60fps_600frames.h264.mp4"},
		// Default timeout (i.e. 2 minutes) is not enough for low-end devices.
		Timeout: 3 * time.Minute,
	})
}

// PlaybackPerfH2642160P60FPS plays H264 2160P 60 FPS video and measures the performance values with/without
// HW decoding acceleration. The values are reported to performance dashboard.
func PlaybackPerfH2642160P60FPS(ctx context.Context, s *testing.State) {
	playback.RunTest(ctx, s, "2160p_60fps_600frames.h264.mp4", "h264_2160p_60fps", playback.DefaultPerfDisabled, playback.VDA)
}
