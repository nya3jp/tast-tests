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
		Func:         PlaybackPerfVP91080P30FPS,
		Desc:         "Measures video playback performance with/without HW acceleration for VP9 1080p@30fps video",
		Contacts:     []string{"hiroh@chromium.org", "chromeos-video-eng@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome_login"},
		Data:         []string{"1080p_30fps_300frames.vp9.webm"},
		// Default timeout (i.e. 2 minutes) is not enough for low-end devices.
		Timeout: 3 * time.Minute,
	})
}

// PlaybackPerfVP91080P30FPS plays VP9 1080P 30 FPS video and measures the performance values with/without
// HW decoding acceleration, and default playback. The values are reported to performance dashboard.
func PlaybackPerfVP91080P30FPS(ctx context.Context, s *testing.State) {
	playback.RunTest(ctx, s, "1080p_30fps_300frames.vp9.webm", "vp9_1080p_30fps", true /* recordDefault */)
}
