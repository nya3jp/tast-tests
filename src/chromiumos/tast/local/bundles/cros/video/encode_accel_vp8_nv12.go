// Copyright 2018 The Chromium OS Authors. All rights reserved.
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
		Func:         EncodeAccelVP8NV12,
		Desc:         "Run Chrome video_encode_accelerator_unittest from NV12 raw frames to VP8 stream",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{caps.HWEncodeVP8},
		Data:         []string{encode.BearNV12.Name},
	})
}

// EncodeAccelVP8NV12 runs video_encode_accelerator_unittest to test VP8 encoding with NV12 raw data, bear_320x192_40frames.nv12.yuv.
func EncodeAccelVP8NV12(ctx context.Context, s *testing.State) {
	encode.RunAccelVideoTest(ctx, s, videotype.VP8Prof, encode.BearNV12)
}
