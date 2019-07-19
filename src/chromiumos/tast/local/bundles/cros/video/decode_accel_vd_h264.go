// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"

	"chromiumos/tast/local/bundles/cros/video/decode"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/local/media/videotype"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     DecodeAccelVDH264,
		Desc:     "Runs Chrome video_decode_accelerator_tests with an H.264 video on a VideoDecoder (see crbug.com/952730)",
		Contacts: []string{"acourbot@chromium.org", "dstaessens@chromium.org", "chromeos-video-eng@google.com"},
		Attr:     []string{"informational"},
		// The android capability guarantees import-mode support, which is a requirment for the new VD's.
		SoftwareDeps: []string{"android", "chrome", caps.HWDecodeH264},
		Data:         decode.DataFiles(videotype.H264Prof),
	})
}

// DecodeAccelVDH264 runs the video_decode_accelerator_tests with test-25fps.h264
// against the new video decoders based on the VideoDecoder interface.
func DecodeAccelVDH264(ctx context.Context, s *testing.State) {
	decode.RunAccelVideoTestNew(ctx, s, decode.Test25FPSH264.Name, decode.VD)
}
