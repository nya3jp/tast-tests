// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/video/encode"
	"chromiumos/tast/local/bundles/cros/video/lib/caps"
	"chromiumos/tast/local/bundles/cros/video/lib/videotype"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ARCEncodeAccelH264I420,
		Desc:         "Tests ARC e2e video encoding from I420 raw frames to an H.264 stream",
		Contacts:     []string{"akahuang@chromium.org", "chromeos-video-eng@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android", "chrome_login", caps.HWEncodeH264},
		Data:         []string{encode.Bear192P.Name},
		Pre:          arc.Booted(),
	})
}

// ARCEncodeAccelH264I420 runs ARC++ encoder e2e test to encode H264 encoding with I420 raw data, bear_320x192_40frames.yuv.
func ARCEncodeAccelH264I420(ctx context.Context, s *testing.State) {
	pd := s.PreValue().(arc.PreData)
	encode.RunARCVideoTest(ctx, s, pd.Chrome, pd.ARC, encode.TestOptions{
		Profile:     videotype.H264Prof,
		Params:      encode.Bear192P,
		PixelFormat: videotype.I420,
	})
}
