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
		Func:     PlaybackVDPerfH2641080P60FPS,
		Desc:     "Measures video playback performance with/without HW acceleration for H264 1080p@60fps video using a media::VideoDecoder",
		Contacts: []string{"akahuang@chromium.org", "dstaessens@chromium.org", "chromeos-video-eng@google.com"},
		Attr:     []string{"group:crosbolt", "crosbolt_perbuild"},
		// "chrome_internal" is needed because H.264 is a proprietary codec.
		// TODO(b/137916185): Remove dependency on android capability. It's used here
		// to guarantee import-mode support, which is required by the new VD's.
		SoftwareDeps: []string{"android", "chrome", "chrome_internal"},
		Data:         []string{"1080p_60fps_600frames.h264.mp4"},
		// Default timeout (i.e. 2 minutes) is not enough for low-end devices.
		Timeout: 5 * time.Minute,
	})
}

// PlaybackVDPerfH2641080P60FPS plays an H264 1080P 60FPS video and collects various performance
// metrics with HW video decode acceleration disabled/enabled, while using a media::VideoDecoder
// (see go/vd-migration). The values are reported to the performance dashboard.
func PlaybackVDPerfH2641080P60FPS(ctx context.Context, s *testing.State) {
	playback.RunTest(ctx, s, "1080p_60fps_600frames.h264.mp4", "h264_1080p_60fps", playback.DefaultPerfDisabled, playback.VD)
}
