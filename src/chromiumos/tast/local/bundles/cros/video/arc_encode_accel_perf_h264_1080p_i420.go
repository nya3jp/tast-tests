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
		Func:         ARCEncodeAccelPerfH2641080PI420,
		Desc:         "Tests ARC e2e video encoding to measure the performance of H264 encoding for 1080p I420 video",
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		Contacts:     []string{"akahuang@chromium.org", "chromeos-video-eng@google.com"},
		SoftwareDeps: []string{"android", "chrome_login", caps.HWEncodeH264},
		Data:         []string{encode.Crowd1080P.Name},
		Pre:          arc.Booted(),
	})
}

// ARCEncodeAccelPerfH2641080PI420 runs ARC++ encoder e2e test to measure performance with 1080p I420 raw data.
func ARCEncodeAccelPerfH2641080PI420(ctx context.Context, s *testing.State) {
	encode.RunARCPerfVideoTest(ctx, s, s.PreValue().(arc.PreData).ARC, encode.TestOptions{
		Profile:     videotype.H264Prof,
		Params:      encode.Crowd1080P,
		PixelFormat: videotype.I420,
	})
}
