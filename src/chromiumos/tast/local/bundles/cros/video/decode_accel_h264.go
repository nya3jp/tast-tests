// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"

	"chromiumos/tast/local/bundles/cros/video/decode"
	"chromiumos/tast/local/bundles/cros/video/lib/caps"
	"chromiumos/tast/local/bundles/cros/video/lib/videotype"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DecodeAccelH264,
		Desc:         "Run Chrome video_decode_accelerator_unittest with a H264 video",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{caps.HWDecodeH264},
		Data:         decode.DataFiles(videotype.H264Prof, decode.AllocateBuffer),
	})
}

// DecodeAccelH264 runs video_decode_accelerator_unittest in ALLOCATE mode with test-25fps.h264.
func DecodeAccelH264(ctx context.Context, s *testing.State) {
	decode.RunAccelVideoTest(ctx, s, decode.Test25FPSH264, decode.AllocateBuffer)
}
