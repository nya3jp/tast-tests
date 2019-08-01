// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"

	"chromiumos/tast/local/bundles/cros/video/decode"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DecodeAccelPerfH2642160P60FPS,
		Desc:         "Runs video_decode_accelerator_perf_tests with an H.264 2160p@60fps video",
		Contacts:     []string{"dstaessens@chromium.org", "chromeos-video-eng@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome", caps.HWDecodeH264},
		Data:         []string{"2160p_60fps_600frames.h264", "2160p_60fps_600frames.h264.json"},
	})
}

// DecodeAccelPerfH2642160P60FPS runs the video_decode_accelerator_perf_tests with 2160p_60fps_600frames.h264.
func DecodeAccelPerfH2642160P60FPS(ctx context.Context, s *testing.State) {
	decode.RunAccelVideoPerfTest(ctx, s, "2160p_60fps_600frames.h264", decode.VDA)
}
