// Copyright 2019 The Chromium OS Authors. All rights reserved.
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
		Func:         PlaybackVDPerfVP92160P60FPS,
		Desc:         "Measures video playback performance with/without HW acceleration for a VP9 2160p@60fps video using a media::VideoDecoder",
		Contacts:     []string{"akahuang@chromium.org", "dstaessens@chromium.org", "chromeos-video-eng@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{"2160p_60fps_600frames.vp9.webm"},
		// Default timeout (i.e. 2 minutes) is not enough for low-end devices.
		Timeout: 3 * time.Minute,
	})
}

// PlaybackVDPerfVP92160P60FPS plays a VP9 2160P 60FPS video and collects various performance
// metrics with HW video decode acceleration disabled/enabled, while using a media::VideoDecoder
// (see go/vd-migration). The values are reported to the performance dashboard.
func PlaybackVDPerfVP92160P60FPS(ctx context.Context, s *testing.State) {
	playback.RunTest(ctx, s, "2160p_60fps_600frames.vp9.webm", "vp9_2160p_60fps", playback.DefaultPerfDisabled, playback.VD)
}
