// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"

	"chromiumos/tast/local/bundles/cros/video/encode"
	"chromiumos/tast/local/bundles/cros/video/lib/caps"
	"chromiumos/tast/local/bundles/cros/video/lib/videotype"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         EncodeAccelPerfVP82160PI420,
		Desc:         "Runs Chrome video_encode_accelerator_unittest to measure the performance of VP8 encoding for 2160p I420 video",
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		Contacts:     []string{"johnylin@chromium.org", "chromeos-video-eng@google.com"},
		SoftwareDeps: []string{"chrome", caps.HWEncodeVP8_4K},
		Data:         []string{encode.Crowd2160P.Name},
	})
}

func EncodeAccelPerfVP82160PI420(ctx context.Context, s *testing.State) {
	encode.RunAccelVideoPerfTest(ctx, s, encode.TestOptions{
		Profile:     videotype.VP8Prof,
		Params:      encode.Crowd2160P,
		PixelFormat: videotype.I420,
		InputMode:   encode.SharedMemory,
	})
}
