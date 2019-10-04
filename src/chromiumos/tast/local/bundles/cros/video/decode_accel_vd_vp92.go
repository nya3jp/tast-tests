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
		Func:     DecodeAccelVDVP92,
		Desc:     "Runs Chrome video_decode_accelerator_tests with an VP9.2 video on a media::VideoDecoder",
		Contacts: []string{"acourbot@chromium.org", "dstaessens@chromium.org", "chromeos-video-eng@google.com"},
		// TODO(crbug.com/911754): reenable this test once HDR VP9.2 is implemented.
		Attr:         []string{"group:mainline", "disabled"},
		SoftwareDeps: []string{"cros_video_decoder", "chrome", caps.HWDecodeVP9_2},
		Data:         []string{"test-25fps.vp9_2", "test-25fps.vp9_2.json"},
	})
}

// DecodeAccelVDVP92 runs the video_decode_accelerator_tests with test-25fps.vp9_2 against
// the new video decoders based on the media::VideoDecoder interface (see go/vd-migration).
func DecodeAccelVDVP92(ctx context.Context, s *testing.State) {
	decode.RunAccelVideoTest(ctx, s, "test-25fps.vp9_2", decode.VD)
}
