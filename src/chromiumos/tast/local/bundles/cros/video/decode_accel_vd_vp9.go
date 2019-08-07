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
		Func:     DecodeAccelVDVP9,
		Desc:     "Runs Chrome video_decode_accelerator_tests with an VP9 video on a media::VideoDecoder",
		Contacts: []string{"acourbot@chromium.org", "dstaessens@chromium.org", "chromeos-video-eng@google.com"},
		Attr:     []string{"informational"},
		// TODO(b/137916185): Remove dependency on android capability. It's used here
		// to guarantee import-mode support, which is required by the new VD's.
		SoftwareDeps: []string{"android", "chrome", caps.HWDecodeVP9},
		Data:         []string{"test-25fps.vp9", "test-25fps.vp9.json"},
	})
}

// DecodeAccelVDVP9 runs the video_decode_accelerator_tests with test-25fps.vp9 against
// the new video decoders based on the media::VideoDecoder interface (see go/vd-migration).
func DecodeAccelVDVP9(ctx context.Context, s *testing.State) {
	decode.RunAccelVideoTest(ctx, s, "test-25fps.vp9", decode.VD)
}
