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
		Func:         EncodeAccelPerfH264I420,
		Desc:         "Run Chrome video_encode_accelerator_unittest to measure the performance of H264 encoding for raw I420 video",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{caps.HWEncodeH264},
		Data:         []string{encode.Bear192P.Name},
	})
}

// EncodeAccelPerfH264I420 runs video_encode_accelerator_unittest to measure the performance of encoding H264 with I420 raw data, bear_320x192_40frames.yuv.
func EncodeAccelPerfH264I420(ctx context.Context, s *testing.State) {
	encode.RunAccelVideoPerfTest(ctx, s, encode.TestOptions{Profile: videotype.H264Prof, Params: encode.Bear192P, PixelFormat: videotype.I420, InputMode: encode.SharedMemory})
}
