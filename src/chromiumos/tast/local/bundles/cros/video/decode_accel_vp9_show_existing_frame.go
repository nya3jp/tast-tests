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
		Func:         DecodeAccelVP9ShowExistingFrame,
		Desc:         "Runs Chrome video_decode_accelerator_tests with a VP9 video that uses the show-existing-frame feature",
		Contacts:     []string{"dstaessens@chromium.org", "chromeos-video-eng@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome", caps.HWDecodeVP9},
		Data:         []string{"vda_sanity-vp90_2_17_show_existing_frame.vp9", "vda_sanity-vp90_2_17_show_existing_frame.vp9.json"},
	})
}

// DecodeAccelVP9ShowExistingFrame runs the video_decode_accelerator_tests with
// vda_sanity-vp90_2_17_show_existing_frame.vp9. This video makes use of the VP9
// show-existing-frame feature and is used in Android CTS:
// https://android.googlesource.com/platform/cts/+/master/tests/tests/media/res/raw/vp90_2_17_show_existing_frame.vp9
func DecodeAccelVP9ShowExistingFrame(ctx context.Context, s *testing.State) {
	decode.RunAccelVideoTest(ctx, s, "vda_sanity-vp90_2_17_show_existing_frame.vp9", decode.VDA)
}
