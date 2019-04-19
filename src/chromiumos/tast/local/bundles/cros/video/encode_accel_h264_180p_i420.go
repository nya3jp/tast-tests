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
		Func:         EncodeAccelH264180PI420,
		Desc:         "Run Chrome video_encode_accelerator_unittest from 180p I420 raw frames to H264 stream",
		Contacts:     []string{"hiroh@chromium.org", "chromeos-video-eng@google.com"},
		SoftwareDeps: []string{"chrome", caps.HWEncodeH264},
		Data:         []string{encode.Tulip180P.Name},
	})
}

// EncodeAccelH264180PI420 runs video_encode_accelerator_unittest to encode H264 encoding with 180p I420 raw data compressed in tulip2-320x180.webm.
func EncodeAccelH264180PI420(ctx context.Context, s *testing.State) {
	encode.RunAllAccelVideoTests(ctx, s, encode.TestOptions{Profile: videotype.H264Prof, Params: encode.Tulip180P, PixelFormat: videotype.I420, InputMode: encode.SharedMemory})
}
