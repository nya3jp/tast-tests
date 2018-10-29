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
		Func:         EncodeAccelH264180PI420,
		Desc:         "Run Chrome video_encode_accelerator_unittest from 180p I420 raw frames to H264 stream",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{caps.HWEncodeH264},
		Data:         []string{encode.YuvToWebm[encode.Tulip180PI420.Name]},
	})
}

// EncodeAccelH264180PI420 runs video_encode_accelerator_unittest to encode H264 encoding with 180p I420 raw data, tulip2-320x180-55be7124b3aec1b72bfb57f433297193.yuv".
func EncodeAccelH264180PI420(ctx context.Context, s *testing.State) {
	encode.RunAccelVideoTest(ctx, s, videotype.H264Prof, encode.Tulip180PI420)
}
