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
		Func:         DecodeAccelPerfVP91080P30FPS,
		Desc:         "Runs video_decode_accelerator_perf_tests with a VP9 1080p@30fps video",
		Contacts:     []string{"dstaessens@chromium.org", "chromeos-video-eng@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome", caps.HWDecodeVP9},
		Data:         []string{"1080p_30fps_300frames.vp9.ivf", "1080p_30fps_300frames.vp9.ivf.json"},
	})
}

// DecodeAccelPerfVP91080P30FPS runs the video_decode_accelerator_perf_tests with 1080p_30fps_300frames.vp9.ivf.
func DecodeAccelPerfVP91080P30FPS(ctx context.Context, s *testing.State) {
	decode.RunAccelVideoPerfTest(ctx, s, "1080p_30fps_300frames.vp9.ivf", decode.VDA)
}
