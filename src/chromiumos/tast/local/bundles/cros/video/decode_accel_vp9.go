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
		Func:         DecodeAccelVP9,
		Desc:         "Run Chrome video_decode_accelerator_unittest with a VP9 video",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{caps.HWDecodeVP9},
		Data:         decode.DataFiles(videotype.VP9Prof, decode.AllocateBuffer),
	})
}

// DecodeAccelVP9 runs video_decode_accelerator_unittest in ALLOCATE mode with test-25fps.vp9.
func DecodeAccelVP9(ctx context.Context, s *testing.State) {
	decode.RunAllAccelVideoTest(ctx, s, decode.Test25FPSVP9, decode.AllocateBuffer)
}
