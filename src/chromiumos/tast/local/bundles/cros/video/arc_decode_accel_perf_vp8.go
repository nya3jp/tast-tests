// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/video/decode"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ARCDecodeAccelPerfVP8,
		Desc:         "Runs c2 e2e tests on ARC++ to measure the performance with VP8 videos",
		Contacts:     []string{"johnylin@chromium.org", "chromeos-video-eng@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"android_both", "chrome", caps.HWDecodeVP8},
		Data:         []string{"C2E2ETest.apk"},
		Pre:          arc.Booted(),
		Timeout:      time.Duration(decode.PerfTestRuntimeSec) * time.Second,
		Params: []testing.Param{{
			Name: "test_25fps",
			Val: params{
				videoName: "test-25fps.vp8",
			},
			ExtraData: []string{"test-25fps.vp8", "test-25fps.vp8.json"},
		}, {
			Name: "1080p_60fps_600frames",
			Val: params{
				videoName: "1080p_60fps_600frames.vp8.ivf",
			},
			ExtraData: []string{"1080p_60fps_600frames.vp8.ivf", "1080p_60fps_600frames.vp8.ivf.json"},
		}, {
			Name: "1080p_30fps_300frames",
			Val: params{
				videoName: "1080p_30fps_300frames.vp8.ivf",
			},
			ExtraData: []string{"1080p_30fps_300frames.vp8.ivf", "1080p_30fps_300frames.vp8.ivf.json"},
		}, {
			Name: "2160p_60fps_600frames",
			Val: params{
				videoName: "2160p_60fps_600frames.vp8.ivf",
			},
			ExtraData: []string{"2160p_60fps_600frames.vp8.ivf", "2160p_60fps_600frames.vp8.ivf.json"},
		}, {
			Name: "2160p_30fps_300frames",
			Val: params{
				videoName: "2160p_30fps_300frames.vp8.ivf",
			},
			ExtraData: []string{"2160p_30fps_300frames.vp8.ivf", "2160p_30fps_300frames.vp8.ivf.json"},
		}},
	})
}

func ARCDecodeAccelPerfVP8(ctx context.Context, s *testing.State) {
	decode.RunARCVideoPerfTest(ctx, s, s.Param().(params).videoName)
}
