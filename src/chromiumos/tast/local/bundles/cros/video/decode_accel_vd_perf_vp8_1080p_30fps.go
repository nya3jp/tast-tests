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
		Func:     DecodeAccelVDPerfVP81080P30FPS,
		Desc:     "Runs video_decode_accelerator_perf_tests with a VP8 1080p@30fps video on a media::VideoDecoder (see go/vd-migration)",
		Contacts: []string{"dstaessens@chromium.org", "chromeos-video-eng@google.com"},
		Attr:     []string{"group:crosbolt", "crosbolt_perbuild"},
		// TODO(b/137916185): Remove dependency on android capability. It's used here
		// to guarantee import-mode support, which is required by the new VD's.
		SoftwareDeps: []string{"android", "chrome", caps.HWDecodeVP8},
		Data:         []string{"1080p_30fps_300frames.vp8.ivf", "1080p_30fps_300frames.vp8.ivf.json"},
	})
}

func DecodeAccelVDPerfVP81080P30FPS(ctx context.Context, s *testing.State) {
	decode.RunAccelVideoPerfTest(ctx, s, "1080p_30fps_300frames.vp8.ivf", decode.VD)
}
