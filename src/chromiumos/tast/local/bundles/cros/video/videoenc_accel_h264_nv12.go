// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"

	"chromiumos/tast/local/bundles/cros/video/lib/caps"
	"chromiumos/tast/local/bundles/cros/video/lib/videotype"
	"chromiumos/tast/local/bundles/cros/video/videoenc"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VideoencAccelH264NV12,
		Desc:         "Run Chrome video_encode_accelerator_unittest from NV12 raw frames to H264 stream",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{caps.HWEncodeH264},
		Data:         []string{videoenc.BearNV12.Name},
	})
}

// VideoencAccelH264NV12 runs encode_accelerator_unittest to encode H264 encoding with NV12 raw data, bear_320x192_40frames.nv12.yuv.
func VideoencAccelH264NV12(ctx context.Context, s *testing.State) {
	videoenc.RunTest(ctx, s, videotype.H264, videoenc.BearNV12)
}
