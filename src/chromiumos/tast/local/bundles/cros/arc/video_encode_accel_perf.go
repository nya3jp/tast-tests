// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/c2e2etest"
	"chromiumos/tast/local/bundles/cros/arc/video"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/local/media/encoding"
	"chromiumos/tast/local/media/videotype"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VideoEncodeAccelPerf,
		Desc:         "Measures ARC++ and ARCM hardware video encode performance by running the arcvideoencoder_test binary",
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		Contacts:     []string{"dstaessens@chromium.org", "chromeos-video-eng@google.com"},
		Data:         []string{c2e2etest.X86ApkName, c2e2etest.ArmApkName},
		SoftwareDeps: []string{"chrome", caps.HWEncodeH264},
		Pre:          arc.Booted(), // TODO(akahuang): Implement new precondition to boot ARC and enable verbose at chromium.
		Timeout:      4 * time.Minute,
		Params: []testing.Param{{
			Name: "h264_1080p_i420",
			Val: encoding.TestOptions{
				Profile:     videotype.H264Prof,
				Params:      video.Crowd1080P,
				PixelFormat: videotype.I420,
			},
			ExtraSoftwareDeps: []string{"android_p"},
			ExtraData:         []string{video.Crowd1080P.Name},
		}, {
			Name: "h264_1080p_i420_vm",
			Val: encoding.TestOptions{
				Profile:     videotype.H264Prof,
				Params:      video.Crowd1080P,
				PixelFormat: videotype.I420,
			},
			ExtraSoftwareDeps: []string{"android_vm"},
			ExtraData:         []string{video.Crowd1080P.Name},
		}},
	})
}

func VideoEncodeAccelPerf(ctx context.Context, s *testing.State) {
	video.RunARCPerfVideoTest(ctx, s, s.PreValue().(arc.PreData).ARC, s.Param().(encoding.TestOptions))
}
