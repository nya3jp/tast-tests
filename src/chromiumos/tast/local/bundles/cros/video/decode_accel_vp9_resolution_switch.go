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
		Func:         DecodeAccelVP9ResolutionSwitch,
		Desc:         "Runs Chrome video_decode_accelerator_tests with a VP9 resolution switching video",
		Contacts:     []string{"dstaessens@chromium.org", "chromeos-video-eng@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome", caps.HWDecodeVP9},
		Data:         []string{"resolution_change_500frames.vp9.ivf", "resolution_change_500frames.vp9.ivf.json"},
	})
}

// DecodeAccelVP9ResolutionSwitch runs the video_decode_accelerator_tests with resolution_change_500frames.vp9.ivf.
func DecodeAccelVP9ResolutionSwitch(ctx context.Context, s *testing.State) {
	decode.RunAccelVideoTest(ctx, s, "resolution_change_500frames.vp9.ivf", decode.VDA)
}
