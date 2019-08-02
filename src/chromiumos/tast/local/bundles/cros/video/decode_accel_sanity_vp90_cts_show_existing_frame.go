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
		Func:         DecodeAccelSanityVP90CtsShowExistingFrame,
		Desc:         "Verify that the system doesn't crash when playing a VP9 video using the 'show existing frame' feature",
		Contacts:     []string{"deanliao@chromium.org", "chromeos-video-eng@google.com"},
		SoftwareDeps: []string{"chrome", caps.HWDecodeVP9},
		Data:         []string{"vda_sanity-vp90_2_17_show_existing_frame.vp9", "vda_sanity-vp90_2_17_show_existing_frame.vp9.json"},
	})
}

// DecodeAccelSanityVP90CtsShowExistingFrame runs the FlushAtEndOfStream test in the
// video_decode_accelerator_tests. The vda_sanity-vp90_2_17_show_existing_frame.vp9 video is used
// which makes use of the 'show existing frame' feature and comes from the Android CTS video
// repository. The test doesn't look at the decode result, but verifies system robustness when
// encountering the 'show existing frame' feature.
// TODO(crbug.com/900467): This test is failing on elm and hana due to driver issue.
func DecodeAccelSanityVP90CtsShowExistingFrame(ctx context.Context, s *testing.State) {
	decode.RunAccelVideoSanityTest(ctx, s, "vda_sanity-vp90_2_17_show_existing_frame.vp9")
}
