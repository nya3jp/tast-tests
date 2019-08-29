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

type params struct {
	videoName string
	videoDesc string
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         ARCPlaybackPerf,
		Desc:         "Measures video playback performance on ARC++ for H.264/VP8/VP9 1080p@30fps video",
		Contacts:     []string{"johnylin@chromium.org", "chromeos-video-eng@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"android", "chrome"},
		Data:         []string{arcplayback.APKName},
		Pre:          arc.Booted(),
		Params: []testing.Param{{
			Name: "h264_1080p_30fps",
			Val: params{
				videoName: "1080p_30fps_600frames.h264.mp4",
				videoDesc: "h264_1080p_30fps",
			},
			ExtraData: []string{"1080p_30fps_600frames.h264.mp4"},
		}, {
			Name: "vp8_1080p_30fps",
			Val: params{
				videoName: "1080p_30fps_600frames.vp8.webm",
				videoDesc: "vp8_1080p_30fps",
			},
			ExtraData: []string{"1080p_30fps_600frames.vp8.webm"},
		}, {
			Name: "vp9_1080p_30fps",
			Val: params{
				videoName: "1080p_30fps_600frames.vp9.webm",
				videoDesc: "vp9_1080p_30fps",
			},
			ExtraData: []string{"1080p_30fps_600frames.vp9.webm"},
		}},
	})
}

// ARCPlaybackPerf plays H.264/VP8/VP9 1080P 30 FPS video by APK on ARC++ and measures CPU usage.
func ARCPlaybackPerf(ctx context.Context, s *testing.State) {
	arcplayback.RunTest(ctx, s, s.PreValue().(arc.PreData).ARC, s.Param().(params).videoName, s.Param().(params).videoDesc)
}
