// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/video/decode"
	"chromiumos/tast/local/bundles/cros/video/playback"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PlaybackVDPerfVP81080P60FPS,
		Desc:         "Measures video playback performance with/without HW acceleration for a VP8 1080p@60fps video using a media::VideoDecoder",
		Contacts:     []string{"akahuang@chromium.org", "dstaessens@chromium.org", "chromeos-video-eng@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"cros_video_decoder", "chrome"},
		Data:         []string{"1080p_60fps_600frames.vp8.webm", decode.ChromeMediaInternalsUtilsJSFile},
		// Default timeout (i.e. 2 minutes) is not enough for low-end devices.
		Timeout: 5 * time.Minute,
	})
}

// PlaybackVDPerfVP81080P60FPS plays a VP8 1080P 60FPS video and collects various performance
// metrics with HW video decode acceleration disabled/enabled, while using a media::VideoDecoder
// (see go/vd-migration). The values are reported to the performance dashboard.
func PlaybackVDPerfVP81080P60FPS(ctx context.Context, s *testing.State) {
	playback.RunTest(ctx, s, "1080p_60fps_600frames.vp8.webm", "vp8_1080p_60fps", playback.DefaultPerfDisabled, playback.VD)
}
