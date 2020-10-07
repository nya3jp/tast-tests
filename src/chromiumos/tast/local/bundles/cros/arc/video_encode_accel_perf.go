// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arc/c2e2etest"
	"chromiumos/tast/local/bundles/cros/arc/video"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/local/media/encoding"
	"chromiumos/tast/local/media/videotype"
	"chromiumos/tast/testing"
)

// Enable to cache the extracted raw video to speed up the test.
const veapCacheExtractedVideo = false

func init() {
	testing.AddTest(&testing.Test{
		Func:         VideoEncodeAccelPerf,
		Desc:         "Measures ARC++ and ARCVM hardware video encode performance by running the arcvideoencoder_test binary",
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
			ExtraData:         []string{video.Crowd1080P.Name},
			ExtraSoftwareDeps: []string{"android_p"},
			ExtraAttr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		}, {
			// TODO(b/140082257): enable once the virtio video encoder is ready
			Name: "h264_1080p_i420_vm",
			Val: encoding.TestOptions{
				Profile:     videotype.H264Prof,
				Params:      video.Crowd1080P,
				PixelFormat: videotype.I420,
			},
			ExtraData:         []string{video.Crowd1080P.Name},
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
	})
}

func VideoEncodeAccelPerf(ctx context.Context, s *testing.State) {
	video.RunARCPerfVideoTest(ctx, s, s.PreValue().(arc.PreData).ARC,
		s.Param().(encoding.TestOptions), veapCacheExtractedVideo)
}
