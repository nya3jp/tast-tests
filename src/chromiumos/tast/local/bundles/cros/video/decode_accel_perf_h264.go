// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"

	"chromiumos/tast/local/bundles/cros/video/decode"
	"chromiumos/tast/local/bundles/cros/video/lib/caps"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DecodeAccelPerfH264,
		Desc:         "Runs Chrome video_decode_accelerator_perf_tests with an H.264 video",
		Contacts:     []string{"dstaessens@chromium.org", "chromeos-video-eng@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{caps.HWDecodeH264},
		Data:         []string{"test-25fps.h264", "test-25fps.h264.json"},
	})
}

// DecodeAccelPerfH264 runs the video_decode_accelerator_perf_tests with test-25fps.h264.
func DecodeAccelPerfH264(ctx context.Context, s *testing.State) {
	decode.RunAccelVideoPerfTest(ctx, s, "test-25fps.h264")
}
