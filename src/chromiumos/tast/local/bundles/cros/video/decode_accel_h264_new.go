// Copyright 2019 The Chromium OS Authors. All rights reserved.
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
		Func:     DecodeAccelH264New,
		Desc:     "Run Chrome video_decode_accelerator_tests with an H.264 video",
		Contacts: []string{"dstaessens@chromium.org", "chromeos-video-eng@google.com"},
		Attr:     []string{"informational"},
		// VDA unittest cannot run with IMPORT mode on devices where ARC++ is disabled. (cf. crbug.com/881729)
		SoftwareDeps: []string{"android", caps.HWDecodeH264},
		Data:         append(decode.DataFiles(videotype.H264Prof, decode.ImportBuffer), decode.Test25FPSH264.Name+".json"),
	})
}

// DecodeAccelH264New runs the video_decode_accelerator_tests with test-25fps.h264.
func DecodeAccelH264New(ctx context.Context, s *testing.State) {
	decode.RunAccelVideoTestNew(ctx, s, decode.Test25FPSH264.Name)
}
