// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/video/c2e2etest"
	"chromiumos/tast/local/bundles/cros/video/encode"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/local/media/videotype"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ARCEncodeAccelPerf,
		Desc:         "Measures ARC++ hardware video encode performance by running the arcvideoencoder_test binary",
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		Contacts:     []string{"dstaessens@chromium.org", "chromeos-video-eng@google.com"},
		Data:         []string{c2e2etest.X86ApkName, c2e2etest.ArmApkName},
		SoftwareDeps: []string{"android_p", "chrome"},
		Pre:          arc.Booted(), // TODO(akahuang): Implement new precondition to boot ARC and enable verbose at chromium.
		Timeout:      4 * time.Minute,
		Params: []testing.Param{{
			Name: "h264_1080p_i420",
			Val: encode.TestOptions{
				Profile:     videotype.H264Prof,
				Params:      encode.Crowd1080P,
				PixelFormat: videotype.I420,
			},
			ExtraSoftwareDeps: []string{caps.HWEncodeH264},
			ExtraData:         []string{encode.Crowd1080P.Name},
		}},
	})
}

func ARCEncodeAccelPerf(ctx context.Context, s *testing.State) {
	encode.RunARCPerfVideoTest(ctx, s, s.PreValue().(arc.PreData).ARC, s.Param().(encode.TestOptions))
}
