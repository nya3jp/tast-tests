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
		Func:         DecodeAccelVP8ResolutionSwitch,
		Desc:         "Runs Chrome video_decode_accelerator_tests with a VP8 resolution switching video",
		Contacts:     []string{"dstaessens@chromium.org", "chromeos-video-eng@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome", caps.HWDecodeVP8},
		Data:         []string{"resolution_change_500frames.vp8.ivf", "resolution_change_500frames.vp8.ivf.json"},
	})
}

// DecodeAccelVP8ResolutionSwitch runs the video_decode_accelerator_tests with resolution_change_500frames.vp8.ivf.
func DecodeAccelVP8ResolutionSwitch(ctx context.Context, s *testing.State) {
	decode.RunAccelVideoTest(ctx, s, "resolution_change_500frames.vp8.ivf", decode.VDA)
}
