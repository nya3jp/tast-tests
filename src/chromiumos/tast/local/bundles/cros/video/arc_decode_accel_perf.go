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
		Func:         ARCDecodeAccelPerf,
		Desc:         "Measures ARC++ hardware video decode performance by running the arcvideodecoder_test binary",
		Contacts:     []string{"johnylin@chromium.org", "chromeos-video-eng@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"android", "chrome"},
		Pre:          arc.Booted(),
		Params: []testing.Param{{
			Name:              "h264_240p",
			Val:               "test-25fps.h264",
			ExtraSoftwareDeps: []string{caps.HWDecodeH264},
			ExtraData:         []string{"test-25fps.h264", "test-25fps.h264.json"},
		}, {
			Name:              "vp8_240p",
			Val:               "test-25fps.vp8",
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8},
			ExtraData:         []string{"test-25fps.vp8", "test-25fps.vp8.json"},
		}, {
			Name:              "vp9_240p",
			Val:               "test-25fps.vp9",
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
			ExtraData:         []string{"test-25fps.vp9", "test-25fps.vp9.json"},
		}},
	})
}

func ARCDecodeAccelPerf(ctx context.Context, s *testing.State) {
	decode.RunARCVideoPerfTest(ctx, s, s.Param().(string))
}
