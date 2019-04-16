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
		Func:         PlaybackPerfVP81080P30FPS,
		Desc:         "Measures video playback performance with/without HW acceleration for VP8 1080p@30fps video",
		Contacts:     []string{"hiroh@chromium.org", "chromeos-video-eng@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome_login"},
		Data:         []string{"1080p_30fps_300frames.vp8.webm"},
		// Default timeout (i.e. 2 minutes) is not enough for low-end devices.
		Timeout: 3 * time.Minute,
	})
}

// PlaybackPerfVP81080P30FPS plays VP8 1080P 30 FPS video and measures the performance values with/without
// HW decoding acceleration. The values are reported to performance dashboard.
func PlaybackPerfVP81080P30FPS(ctx context.Context, s *testing.State) {
	playback.RunTest(ctx, s, "1080p_30fps_300frames.vp8.webm", "vp8_1080p_30fps", playback.DefaultPerfDisabled)
}
