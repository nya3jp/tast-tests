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
		Func:         DecodeAccelSanityVP91,
		Desc:         "Verify that the system doesn't crash when playing a VP9 video with unexpected VP9 profile1 features",
		Contacts:     []string{"deanliao@chromium.org", "chromeos-video-eng@google.com"},
		SoftwareDeps: []string{"chrome", caps.HWDecodeVP9},
		Data:         []string{"vda_sanity-bear_profile1.vp9", "vda_sanity-bear_profile1.vp9.json"},
	})
}

// DecodeAccelSanityVP91 runs the FlushAtEndOfStream test in the video_decode_accelerator_tests.
// The vda_sanity-bear_profile1.vp9 video is used with metadata that incorrectly initializes the
// video decoder for VP9 profile0. The test doesn't look at the decode result, but verifies system
// robustness when encountering unexpected features.
func DecodeAccelSanityVP91(ctx context.Context, s *testing.State) {
	decode.RunAccelVideoSanityTest(ctx, s, "vda_sanity-bear_profile1.vp9")
}
