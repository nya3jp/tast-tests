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
		Desc:         "Run Chrome video_decode_accelerator_tests FlushAtEndOfStream test on a VP9 video from Android CTS video repository which fails elm",
		Contacts:     []string{"deanliao@chromium.org", "chromeos-video-eng@google.com"},
		SoftwareDeps: []string{"chrome", caps.HWDecodeVP9},
		Data:         []string{"vda_sanity-vp90_2_17_show_existing_frame.vp9", "vda_sanity-vp90_2_17_show_existing_frame.vp9.json"},
	})
}

// DecodeAccelSanityVP90CtsShowExistingFrame runs FlushAtEndOfStream test in video_decode_accelerator_tests
// with vda_sanity-vp90_2_17_show_existing_frame.vp9.
// TODO(crbug.com/900467): This test is failing on elm and hana due to driver issue.
func DecodeAccelSanityVP90CtsShowExistingFrame(ctx context.Context, s *testing.State) {
	decode.RunAccelVideoSanityTest(ctx, s, "vda_sanity-vp90_2_17_show_existing_frame.vp9")
}
