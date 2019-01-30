// Copyright 2019 The Chromium OS Authors. All rights reserved.
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
		Func:         EncodeAccelPerfH2641080PI420,
		Desc:         "Run Chrome video_encode_accelerator_unittest to measure the performance of H264 encoding for 1080p I420 video",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{caps.HWEncodeH264},
		Data:         []string{encode.Crowd1080P.Name},
	})
}

// EncodeAccelPerfH2641080PI420 runs video_encode_accelerator_unittest to measure the performance of encoding H264 with 1080p I420 raw data compressed in crowd-1920x1080.webm.
func EncodeAccelPerfH2641080PI420(ctx context.Context, s *testing.State) {
	encode.RunAccelVideoPerfTest(ctx, s, encode.TestOptions{Profile: videotype.H264Prof, Params: encode.Crowd1080P, PixelFormat: videotype.I420, InputMode: encode.SharedMemory})
}
