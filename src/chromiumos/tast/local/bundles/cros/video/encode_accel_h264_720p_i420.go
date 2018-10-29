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
		Func:         EncodeAccelH264720PI420,
		Desc:         "Run Chrome video_encode_accelerator_unittest from 720p I420 raw frames to H264 stream",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{caps.HWEncodeH264},
		Data:         []string{encode.Tulip720PI420.Name},
	})
}

// EncodeAccelH264720PI420 runs video_encode_accelerator_unittest to encode H264 encoding with 720p I420 raw data, tulip2-1280x720-1b95123232922fe0067869c74e19cd09.yuv.
func EncodeAccelH264720PI420(ctx context.Context, s *testing.State) {
	encode.RunAccelVideoTest(ctx, s, videotype.H264Prof, encode.Tulip720PI420)
}
