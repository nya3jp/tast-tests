// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/video/encode"
	"chromiumos/tast/local/bundles/cros/video/lib/caps"
	"chromiumos/tast/local/bundles/cros/video/lib/videotype"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         EncodeAccelH2641080PI420,
		Desc:         "Run Chrome video_encode_accelerator_unittest from 1080p I420 raw frames to H264 stream",
		Contacts:     []string{"hiroh@chromium.org", "chromeos-video-eng@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{caps.HWEncodeH264},
		Data:         []string{encode.Crowd1080P.Name},
		// This test takes up to 3 minutes when it passes. Allow 4 minutes with 1-minute buffer.
		Timeout: 4 * time.Minute,
	})
}

// EncodeAccelH2641080PI420 runs video_encode_accelerator_unittest to encode H264 encoding with 1080p I420 raw data compressed in crowd-1920x1080.webm.
func EncodeAccelH2641080PI420(ctx context.Context, s *testing.State) {
	encode.RunAllAccelVideoTests(ctx, s, encode.TestOptions{Profile: videotype.H264Prof, Params: encode.Crowd1080P, PixelFormat: videotype.I420, InputMode: encode.SharedMemory})
}
