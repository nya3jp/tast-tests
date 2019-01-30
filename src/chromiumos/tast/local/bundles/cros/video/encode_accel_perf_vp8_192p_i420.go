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
		Func:         EncodeAccelPerfVP8192PI420,
		Desc:         "Runs Chrome video_encode_accelerator_unittest to measure the performance of VP8 encoding for 192p I420 video",
		Attr:         []string{"informational"},
		Contacts:     []string{"johnylin@chromium.org", "chromeos-video-eng@google.com"},
		SoftwareDeps: []string{caps.HWEncodeVP8},
		Data:         []string{encode.Bear192P.Name},
	})
}

// EncodeAccelPerfVP8192PI420 runs video_encode_accelerator_unittest to measure the performance of encoding VP8 with I420 raw data, bear_320x192_40frames.yuv.
func EncodeAccelPerfVP8192PI420(ctx context.Context, s *testing.State) {
	encode.RunAccelVideoPerfTest(ctx, s, encode.TestOptions{Profile: videotype.VP8Prof, Params: encode.Bear192P, PixelFormat: videotype.I420, InputMode: encode.SharedMemory})
}
