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
		Func:     DecodeAccelVDVP9ShowExistingFrame,
		Desc:     "Runs Chrome video_decode_accelerator_tests with a VP9 video that uses the show-existing-frame feature on a media::VideoDecoder (see go/vd-migration)",
		Contacts: []string{"akahuang@chromium.org", "dstaessens@chromium.org", "chromeos-video-eng@google.com"},
		// TODO(b/137916185): Remove dependency on android capability. It's used here
		// to guarantee import-mode support, which is required by the new VD's.
		Attr:         []string{"android", "informational"},
		SoftwareDeps: []string{"chrome", caps.HWDecodeVP9},
		Data:         []string{"vda_sanity-vp90_2_17_show_existing_frame.vp9", "vda_sanity-vp90_2_17_show_existing_frame.vp9.json"},
	})
}

// DecodeAccelVDVP9ShowExistingFrame runs the video_decode_accelerator_tests with
// vda_sanity-vp90_2_17_show_existing_frame.vp9. This video makes use of the VP9
// show-existing-frame feature and is used in Android CTS:
// https://android.googlesource.com/platform/cts/+/master/tests/tests/media/res/raw/vp90_2_17_show_existing_frame.vp9
func DecodeAccelVDVP9ShowExistingFrame(ctx context.Context, s *testing.State) {
	decode.RunAccelVideoTestNew(ctx, s, "vda_sanity-vp90_2_17_show_existing_frame.vp9", decode.VD)
}
