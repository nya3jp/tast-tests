// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"

	"chromiumos/tast/local/bundles/cros/video/encode"
	"chromiumos/tast/local/bundles/cros/video/lib/caps"
	"chromiumos/tast/local/bundles/cros/video/lib/videotype"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     EncodeAccelH264NV12DMABUF,
		Desc:     "Run Chrome video_encode_accelerator_unittest from NV12 raw frames to H264 stream with --native_input",
		Contacts: []string{"hiroh@chromium.org", "chromeos-video-eng@google.com"},
		Attr:     []string{"informational"},
		// Although the ability to android is unrelated to this test ability, we would like to run this test on ARC++ enabled boards.
		// TODO(hiroh): Remove "android" deps once Chrome VEAs and Chrome OS supports DMABUF-backed video frame on all boards.
		SoftwareDeps: []string{"chrome", "android", caps.HWEncodeH264},
		Data:         []string{encode.Bear192P.Name},
	})
}

// EncodeAccelH264NV12DMABUF runs video_encode_accelerator_unittest to encode H264 encoding with NV12 raw data, bear_320x192_40frames.nv12.yuv.
// The inputting VideoFrame on VEA::Encode() is DMABUF-backed one.
func EncodeAccelH264NV12DMABUF(ctx context.Context, s *testing.State) {
	encode.RunAllAccelVideoTests(ctx, s, encode.TestOptions{Profile: videotype.H264Prof, Params: encode.Bear192P, PixelFormat: videotype.NV12, InputMode: encode.DMABuf})
}
