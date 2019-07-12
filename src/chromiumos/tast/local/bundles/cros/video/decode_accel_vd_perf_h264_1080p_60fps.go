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
		Func:     DecodeAccelVDPerfH2641080P60FPS,
		Desc:     "Runs video_decode_accelerator_perf_tests with an H.264 1080p@60fps video on a media::VideoDecoder",
		Contacts: []string{"dstaessens@chromium.org", "chromeos-video-eng@google.com"},
		Attr:     []string{"group:crosbolt", "crosbolt_perbuild"},
		// TODO(b/137916185): Remove dependency on android capability. It's used here
		// to guarantee import-mode support, which is required by the new VD's.
		SoftwareDeps: []string{"android", "chrome", caps.HWDecodeH264},
		Data:         []string{"1080p_60fps_600frames.h264", "1080p_60fps_600frames.h264.json"},
	})
}

// DecodeAccelVDPerfH2641080P60FPS runs the video_decode_accelerator_perf_tests with 1080p_60fps_600frames.h264
// against the new video decoders based on the media::VideoDecoder interface (see go/vd-migration).
func DecodeAccelVDPerfH2641080P60FPS(ctx context.Context, s *testing.State) {
	decode.RunAccelVideoPerfTest(ctx, s, "1080p_60fps_600frames.h264", decode.VD)
}
