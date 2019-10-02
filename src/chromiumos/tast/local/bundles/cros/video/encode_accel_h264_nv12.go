// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/video/encode"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/local/media/videotype"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         EncodeAccelH264NV12,
		Desc:         "Run Chrome video_encode_accelerator_unittest from NV12 raw frames to H264 stream",
		Contacts:     []string{"akahuang@chromium.org", "chromeos-video-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", caps.HWEncodeH264},
		Data:         []string{encode.Bear192P.Name},
		Timeout:      4 * time.Minute,
	})
}

// EncodeAccelH264NV12 runs video_encode_accelerator_unittest to encode H264 encoding with NV12 raw data, bear_320x192_40frames.nv12.yuv.
func EncodeAccelH264NV12(ctx context.Context, s *testing.State) {
	encode.RunAllAccelVideoTests(ctx, s, encode.TestOptions{Profile: videotype.H264Prof, Params: encode.Bear192P, PixelFormat: videotype.NV12, InputMode: encode.SharedMemory})
}
