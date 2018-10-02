// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"

	"chromiumos/tast/local/bundles/cros/video/enc"
	"chromiumos/tast/local/bundles/cros/video/lib/caps"
	"chromiumos/tast/local/bundles/cros/video/lib/videotype"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         EncAccelH264I420,
		Desc:         "Run Chrome video_encode_accelerator_unittest from I420 raw frames to H264 stream",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{caps.HWEncodeH264},
		Data:         []string{enc.BearI420.Name},
	})
}

// EncAccelH264I420 runs encode_accelerator_unittest to encode H264 encoding with I420 raw data, bear_320x192_40frames.yuv.
func EncAccelH264I420(ctx context.Context, s *testing.State) {
	enc.RunAccelVideoTest(ctx, s, videotype.H264Prof, enc.BearI420)
}
