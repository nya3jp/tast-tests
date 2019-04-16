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
		Func:         PlaybackPerfAV1720P30FPS,
		Desc:         "Measures video playback performance with/without HW acceleration for AV1 720p@30fps video",
		Contacts:     []string{"hiroh@chromium.org", "chromeos-video-eng@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome_login"},
		Data:         []string{"720p_30fps_300frames.av1.mp4"},
		// Default timeout (i.e. 2 minutes) is not enough for low-end devices.
		Timeout: 3 * time.Minute,
	})
}

func PlaybackPerfAV1720P30FPS(ctx context.Context, s *testing.State) {
	playback.RunTest(ctx, s, "720p_30fps_300frames.av1.mp4", "av1_720p_30fps", false)
}
