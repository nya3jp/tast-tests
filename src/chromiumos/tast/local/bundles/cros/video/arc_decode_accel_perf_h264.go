// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/video/decode"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ARCDecodeAccelPerfH264,
		Desc:         "Runs arcvideodecoder_test on ARC++ to measure the performance with H.264 videos",
		Contacts:     []string{"johnylin@chromium.org", "chromeos-video-eng@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"android", "chrome", caps.HWDecodeH264},
		Data:         []string{"ArcMediaCodecTest.apk"},
		Pre:          arc.Booted(),
		Params: []testing.Param{{
			Name: "test_25fps",
			Val: params{
				videoName: "test-25fps.h264",
			},
			ExtraData: []string{"test-25fps.h264", "test-25fps.h264.json"},
		}},
	})
}

func ARCDecodeAccelPerfH264(ctx context.Context, s *testing.State) {
	decode.RunARCVideoPerfTest(ctx, s, s.Param().(params).videoName)
}
