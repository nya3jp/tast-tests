// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"

	"chromiumos/tast/local/bundles/cros/video/decode"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/local/media/videotype"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DecodeAccelH264New,
		Desc:         "Run Chrome video_decode_accelerator_tests with an H.264 video",
		Contacts:     []string{"acourbot@chromium.org", "dstaessens@chromium.org", "chromeos-video-eng@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome", caps.HWDecodeH264},
		Data:         decode.DataFiles(videotype.H264Prof),
	})
}

// DecodeAccelH264New runs the video_decode_accelerator_tests with test-25fps.h264.
// TODO(dstaessens): Drop the 'New' suffix when the old VDA tests have been deprecated.
func DecodeAccelH264New(ctx context.Context, s *testing.State) {
	decode.RunAccelVideoTestNew(ctx, s, decode.Test25FPSH264.Name, decode.VDA)
}
