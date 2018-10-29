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
		Func:         EncodeAccelH264360PI420,
		Desc:         "Run Chrome video_encode_accelerator_unittest from 360p I420 raw frames to H264 stream",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{caps.HWEncodeH264},
		Data:         []string{encode.YuvToWebm[encode.Tulip360PI420.Name]},
	})
}

// EncodeAccelH264360PI420 runs video_encode_accelerator_unittest to encode H264 encoding with 360p I420 raw data compressed in tulip2-640x360.webm.
func EncodeAccelH264360PI420(ctx context.Context, s *testing.State) {
	encode.RunAccelVideoTest(ctx, s, videotype.H264Prof, encode.Tulip360PI420)
}
