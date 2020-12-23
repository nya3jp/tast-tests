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
	"chromiumos/tast/local/media/videotype"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

// Enable to cache the extracted raw video to speed up the test.
const veapCacheExtractedVideo = false

func init() {
	testing.AddTest(&testing.Test{
		Func:         VideoEncodeAccelPerf,
		Desc:         "Measures ARC++ and ARCVM hardware video encode performance by running the arcvideoencoder_test binary",
		Contacts:     []string{"dstaessens@chromium.org", "chromeos-video-eng@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		Data:         []string{c2e2etest.X86ApkName, c2e2etest.ArmApkName},
		SoftwareDeps: []string{"chrome", caps.HWEncodeH264},
		HardwareDeps: hwdep.D(hwdep.SkipOnPlatform(video.EncoderBlocklist...)),
		Fixture:      "arcBooted", // TODO(akahuang): Implement new precondition to boot ARC and enable verbose at chromium.
		Timeout:      5 * time.Minute,
		Params: []testing.Param{{
			Name: "h264_1080p_i420",
			Val: video.EncodeTestOptions{
				Profile:     videotype.H264Prof,
				Params:      video.Crowd1080P,
				PixelFormat: videotype.I420,
			},
			ExtraData:         []string{video.Crowd1080P.Name},
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name: "h264_1080p_i420_vm",
			Val: video.EncodeTestOptions{
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
	video.RunARCPerfVideoTest(ctx, s, s.FixtValue().(*arc.PreData).ARC,
		s.Param().(video.EncodeTestOptions), veapCacheExtractedVideo)
}
