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
		Func:         PlaybackPerfVP82160P60FPS,
		Desc:         "Measures video playback performance with/without HW acceleration for VP8 2160p@60fps video",
		Contacts:     []string{"hiroh@chromium.org", "chromeos-video-eng@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{"2160p_60fps_600frames.vp8.webm"},
		// Default timeout (i.e. 2 minutes) is not enough for low-end devices.
		Timeout: 5 * time.Minute,
	})
}

// PlaybackPerfVP82160P60FPS plays VP8 2160P 60 FPS video and measures the performance values with/without
// HW decoding acceleration. The values are reported to performance dashboard.
func PlaybackPerfVP82160P60FPS(ctx context.Context, s *testing.State) {
	playback.RunTest(ctx, s, "2160p_60fps_600frames.vp8.webm", "vp8_2160p_60fps", playback.DefaultPerfDisabled, playback.VDA)
}
