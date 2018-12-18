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
		Func:         ArcEncodeAccelH264I420,
		Desc:         "Run ARC video encoding e2e test from I420 raw frames to H264 stream",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{caps.HWEncodeH264},
		Data:         []string{encode.Bear192P.Name},
	})
}

// ArcEncodeAccelH264I420 runs ARC++ encoder e2e2 test to encode H264 encoding with I420 raw data, bear_320x192_40frames.yuv.
func ArcEncodeAccelH264I420(ctx context.Context, s *testing.State) {
	encode.RunArcVideoTest(ctx, s, encode.TestOptions{Profile: videotype.H264Prof, Params: encode.Bear192P, PixelFormat: videotype.I420})
}
