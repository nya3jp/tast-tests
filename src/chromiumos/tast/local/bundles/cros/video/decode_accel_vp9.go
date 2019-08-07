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
		Func:         DecodeAccelVP9,
		Desc:         "Run Chrome video_decode_accelerator_tests with a VP9 video",
		Contacts:     []string{"acourbot@chromium.org", "dstaessens@chromium.org", "chromeos-video-eng@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome", caps.HWDecodeVP9},
		Data:         []string{"test-25fps.vp9", "test-25fps.vp9.json"},
	})
}

// DecodeAccelVP9 runs the video_decode_accelerator_tests with test-25fps.vp9.
func DecodeAccelVP9(ctx context.Context, s *testing.State) {
	decode.RunAccelVideoTest(ctx, s, "test-25fps.vp9", decode.VDA)
}
