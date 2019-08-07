// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"

	"chromiumos/tast/local/bundles/cros/video/decode"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DecodeAccelVP8,
		Desc:         "Run Chrome video_decode_accelerator_tests with a VP8 video",
		Contacts:     []string{"acourbot@chromium.org", "dstaessens@chromium.org", "chromeos-video-eng@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome", caps.HWDecodeVP8},
		Data:         []string{"test-25fps.vp8", "test-25fps.vp8.json"},
	})
}

// DecodeAccelVP8 runs the video_decode_accelerator_tests with test-25fps.vp8.
func DecodeAccelVP8(ctx context.Context, s *testing.State) {
	decode.RunAccelVideoTest(ctx, s, "test-25fps.vp8", decode.VDA)
}
