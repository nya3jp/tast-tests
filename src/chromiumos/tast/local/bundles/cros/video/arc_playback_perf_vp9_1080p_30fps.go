// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/video/arcplayback"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ARCPlaybackPerfVP91080P30FPS,
		Desc:         "Measures video playback performance on ARC++ w/ HW acceleration for VP9 1080p@30fps video",
		Contacts:     []string{"johnylin@chromium.org", "chromeos-video-eng@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android", "chrome", caps.HWDecodeVP9},
		Data:         []string{"arc_video_test.apk", "1080p_30fps_600frames.vp9.webm"},
		Pre:          arc.Booted(),
	})
}

// ARCPlaybackPerfVP91080P30FPS plays VP9 1080P 30 FPS video by APK on ARC++ and measures CPU usage.
func ARCPlaybackPerfVP91080P30FPS(ctx context.Context, s *testing.State) {
	arcplayback.RunTest(ctx, s, s.PreValue().(arc.PreData).ARC, "1080p_30fps_600frames.vp9.webm", "vp9_1080p_30fps")
}
