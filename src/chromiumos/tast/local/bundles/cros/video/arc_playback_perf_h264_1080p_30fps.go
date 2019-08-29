// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/video/arcplayback"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ARCPlaybackPerfH2641080P30FPS,
		Desc:         "Measures video playback performance on ARC++ for H.264 1080p@30fps video",
		Contacts:     []string{"johnylin@chromium.org", "chromeos-video-eng@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"android", "chrome"},
		Data:         []string{arcplayback.APKName, "1080p_30fps_600frames.h264.mp4"},
		Pre:          arc.Booted(),
	})
}

// ARCPlaybackPerfH2641080P30FPS plays H.264 1080P 30 FPS video by APK on ARC++ and measures CPU usage.
func ARCPlaybackPerfH2641080P30FPS(ctx context.Context, s *testing.State) {
	arcplayback.RunTest(ctx, s, s.PreValue().(arc.PreData).ARC, "1080p_30fps_600frames.h264.mp4", "h264_1080p_30fps")
}
