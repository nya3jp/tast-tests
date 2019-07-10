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
		Func:         DecodeAccelPerfVP81080P60FPS,
		Desc:         "Runs video_decode_accelerator_perf_tests with a VP8 1080p@60fps video",
		Contacts:     []string{"dstaessens@chromium.org", "chromeos-video-eng@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome", caps.HWDecodeVP8},
		Data:         []string{"1080p_60fps_600frames.vp8.ivf", "1080p_60fps_600frames.vp8.ivf.json"},
	})
}

// DecodeAccelPerfVP81080P60FPS runs the video_decode_accelerator_perf_tests with 1080p_60fps_600frames.vp8.ivf.
func DecodeAccelPerfVP81080P60FPS(ctx context.Context, s *testing.State) {
	decode.RunAccelVideoPerfTest(ctx, s, "1080p_60fps_600frames.vp8.ivf")
}
