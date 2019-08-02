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
		Desc:         "Run Chrome video_decode_accelerator_tests FlushAtEndOfStream test on a VP9.1 video",
		Contacts:     []string{"deanliao@chromium.org", "chromeos-video-eng@google.com"},
		SoftwareDeps: []string{"chrome", caps.HWDecodeVP9},
		Data:         []string{"vda_sanity-bear_profile1.vp9", "vda_sanity-bear_profile1.vp9.json"},
	})
}

// DecodeAccelSanityVP91 runs FlushAtEndOfStream test in video_decode_accelerator_tests
// with vda_sanity-bear_profile1.vp9.
func DecodeAccelSanityVP91(ctx context.Context, s *testing.State) {
	decode.RunAccelVideoSanityTest(ctx, s, "vda_sanity-bear_profile1.vp9")
}
