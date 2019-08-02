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
		Func:     DecodeAccelSanityVP92,
		Desc:     "Run Chrome video_decode_accelerator_tests FlushAtEndOfStream test on a VP9.2 video",
		Contacts: []string{"deanliao@chromium.org", "chromeos-video-eng@google.com"},
		Attr:     []string{"informational"},
		// "vp9_sanity" is a whitelist of devices that stay alive playing unsupported VP9 profile stream.
		// Currently RK3399 devices may crash playing the VP9 profile 2 stream, so they are excluded.
		// See crbug.com/971032 for detail.
		SoftwareDeps: []string{"chrome", caps.HWDecodeVP9, "vp9_sanity"},
		Data:         []string{"vda_sanity-bear_profile2.vp9", "vda_sanity-bear_profile2.vp9.json"},
	})
}

// DecodeAccelSanityVP92 runs FlushAtEndOfStream test in video_decode_accelerator_tests
// with vda_sanity-bear_profile2.vp9
func DecodeAccelSanityVP92(ctx context.Context, s *testing.State) {
	decode.RunAccelVideoSanityTest(ctx, s, "vda_sanity-bear_profile2.vp9")
}
