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
		Func:         EncodeAccelVP8I420,
		Desc:         "Run Chrome video_encode_accelerator_unittest from I420 raw frames to VP8 stream",
		Contacts:     []string{"hiroh@chromium.org", "chromeos-video-eng@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome", caps.HWEncodeVP8},
		Data:         []string{encode.Bear192P.Name},
	})
}

// EncodeAccelVP8I420 runs video_encode_accelerator_unittest to test VP8 encoding with I420 raw data, bear_320x192_40frames.yuv.
func EncodeAccelVP8I420(ctx context.Context, s *testing.State) {
	encode.RunAllAccelVideoTests(ctx, s, encode.TestOptions{Profile: videotype.VP8Prof, Params: encode.Bear192P, PixelFormat: videotype.I420, InputMode: encode.SharedMemory})
}
