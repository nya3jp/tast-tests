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
		Func: DecodeAccelSanity,
		Desc: "Verifies that the system doesn't crash when playing a VP9 video with unexpected VP9 profile1/2/3 features",
		Contacts: []string{
			"dstaessens@chromium.org",
			"chromeos-gfx-video@google.com",
			"chromeos-video-eng@google.com",
		},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome", caps.HWDecodeVP9},
		Params: []testing.Param{{
			Name:      "vp9_1",
			Val:       "vda_sanity-bear_profile1.vp9",
			ExtraData: []string{"vda_sanity-bear_profile1.vp9", "vda_sanity-bear_profile1.vp9.json"},
		}, {
			Name:      "vp9_2",
			Val:       "vda_sanity-bear_profile2.vp9",
			ExtraAttr: []string{"informational"},
			ExtraData: []string{"vda_sanity-bear_profile2.vp9", "vda_sanity-bear_profile2.vp9.json"},
			// The "vp9_sanity" SoftwareDeps is a whitelist used to filter out devices that are
			// known to be unstable when encountering unexpected features in a VP9 video stream.
			// The whitelist is used to avoid crashes on devices that are not expected to be fixed
			// soon, as device crashes affect all subsequent test runs. Currently RK3399 devices
			// may crash so they are excluded. See crbug.com/971032 for details.
			ExtraSoftwareDeps: []string{"vp9_sanity"},
		}, {
			Name:              "vp9_3",
			Val:               "vda_sanity-bear_profile3.vp9",
			ExtraAttr:         []string{"informational"},
			ExtraData:         []string{"vda_sanity-bear_profile3.vp9", "vda_sanity-bear_profile3.vp9.json"},
			ExtraSoftwareDeps: []string{"vp9_sanity"},
		}},
	})
}

// DecodeAccelSanity runs the FlushAtEndOfStream test in the video_decode_accelerator_tests. The
// vda_sanity-bear_profile{1,2,3}.vp9 video is used with metadata that incorrectly initializes the
// video decoder for VP9 profile0. The test doesn't look at the decode result, but verifies system
// robustness when encountering unexpected features.
func DecodeAccelSanity(ctx context.Context, s *testing.State) {
	decode.RunAccelVideoSanityTest(ctx, s, s.Param().(string))
}
