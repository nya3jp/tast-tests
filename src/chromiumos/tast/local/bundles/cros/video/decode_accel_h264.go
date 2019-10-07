// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/video/decode"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/local/media/videotype"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DecodeAccelH264,
		Desc:         "Run Chrome video_decode_accelerator_unittest with an H.264 video",
		Contacts:     []string{"acourbot@chromium.org", "dstaessens@chromium.org", "chromeos-video-eng@google.com"},
		SoftwareDeps: []string{"chrome", caps.HWDecodeH264},
		Data:         decode.DataFiles(videotype.H264Prof),
		Timeout:      4 * time.Minute,
	})
}

// DecodeAccelH264 runs video_decode_accelerator_unittest in ALLOCATE mode with test-25fps.h264.
func DecodeAccelH264(ctx context.Context, s *testing.State) {
	decode.RunAllAccelVideoTest(ctx, s, decode.Test25FPSH264, decode.AllocateBuffer)
}
