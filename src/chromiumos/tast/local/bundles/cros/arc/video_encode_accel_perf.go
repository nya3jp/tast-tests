// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arc/c2e2etest"
	"chromiumos/tast/local/bundles/cros/arc/video"
	"chromiumos/tast/local/media/videotype"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

// Enable to cache the extracted raw video to speed up the test.
const veapCacheExtractedVideo = false

func init() {
	testing.AddTest(&testing.Test{
		Func:         VideoEncodeAccelPerf,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Measures ARC++ and ARCVM hardware video encode performance by running the arcvideoencoder_test binary",
		Contacts:     []string{"dstaessens@chromium.org", "chromeos-video-eng@google.com"},
		Data:         []string{c2e2etest.X86ApkName, c2e2etest.ArmApkName},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "arcBooted", // TODO(akahuang): Implement new precondition to boot ARC and enable verbose at chromium.
		Timeout:      12 * time.Minute,
		Params: []testing.Param{{
			Name: "h264_1080p_i420",
			Val: video.EncodeTestOptions{
				Profile:     videotype.H264MainProf,
				Params:      video.Crowd1080P,
				PixelFormat: videotype.I420,
			},
			ExtraAttr:         []string{"group:crosbolt", "crosbolt_perbuild"},
			ExtraData:         []string{video.Crowd1080P.Name},
			ExtraSoftwareDeps: []string{"android_p", caps.HWEncodeH264},
		}, {
			Name: "h264_1080p_i420_sw",
			Val: video.EncodeTestOptions{
				Profile:     videotype.H264MainProf,
				Params:      video.Crowd1080P,
				PixelFormat: videotype.I420,
				EncoderType: video.SoftwareEncoder,
			},
			ExtraData:         []string{video.Crowd1080P.Name},
			ExtraSoftwareDeps: []string{"android_p", caps.HWEncodeH264},
		}, {
			Name: "h264_1080p_i420_vm",
			Val: video.EncodeTestOptions{
				Profile:     videotype.H264MainProf,
				Params:      video.Crowd1080P,
				PixelFormat: videotype.I420,
			},
			ExtraAttr:         []string{"group:crosbolt", "crosbolt_perbuild"},
			ExtraData:         []string{video.Crowd1080P.Name},
			ExtraSoftwareDeps: []string{"android_vm", caps.HWEncodeH264},
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnPlatform(video.EncoderBlocklistVM...)),
		}, {
			Name: "h264_1080p_i420_sw_vm",
			Val: video.EncodeTestOptions{
				Profile:     videotype.H264MainProf,
				Params:      video.Crowd1080P,
				PixelFormat: videotype.I420,
				EncoderType: video.SoftwareEncoder,
			},
			ExtraData:         []string{video.Crowd1080P.Name},
			ExtraSoftwareDeps: []string{"android_vm"},
		}, {
			Name: "vp8_1080p_i420_vm",
			Val: video.EncodeTestOptions{
				Profile:     videotype.VP8Prof,
				Params:      video.Crowd1080P,
				PixelFormat: videotype.I420,
			},
			ExtraAttr:         []string{"group:crosbolt", "crosbolt_perbuild"},
			ExtraData:         []string{video.Crowd1080P.Name},
			ExtraSoftwareDeps: []string{"android_vm", caps.HWEncodeVP8},
			ExtraHardwareDeps: hwdep.D(
				hwdep.SkipOnPlatform(video.EncoderBlocklistVM...),
				hwdep.Platform(video.EncoderAllowlistVPxVM...)),
		}, {
			Name: "vp8_1080p_i420_sw_vm",
			Val: video.EncodeTestOptions{
				Profile:     videotype.VP8Prof,
				Params:      video.Crowd1080P,
				PixelFormat: videotype.I420,
				EncoderType: video.SoftwareEncoder,
			},
			ExtraData:         []string{video.Crowd1080P.Name},
			ExtraSoftwareDeps: []string{"android_vm"},
		}, {
			Name: "vp9_1080p_i420_vm",
			Val: video.EncodeTestOptions{
				Profile:     videotype.VP9Prof,
				Params:      video.Crowd1080P,
				PixelFormat: videotype.I420,
			},
			ExtraAttr:         []string{"group:crosbolt", "crosbolt_perbuild"},
			ExtraData:         []string{video.Crowd1080P.Name},
			ExtraSoftwareDeps: []string{"android_vm", caps.HWEncodeVP9},
			ExtraHardwareDeps: hwdep.D(
				hwdep.SkipOnPlatform(video.EncoderBlocklistVM...),
				hwdep.Platform(video.EncoderAllowlistVPxVM...)),
		}, {
			Name: "vp9_1080p_i420_sw_vm",
			Val: video.EncodeTestOptions{
				Profile:     videotype.VP9Prof,
				Params:      video.Crowd1080P,
				PixelFormat: videotype.I420,
				EncoderType: video.SoftwareEncoder,
			},
			ExtraData:         []string{video.Crowd1080P.Name},
			ExtraSoftwareDeps: []string{"android_vm", caps.HWEncodeVP9},
			ExtraHardwareDeps: hwdep.D(
				hwdep.SkipOnPlatform(video.EncoderBlocklistVM...),
				hwdep.Platform(video.EncoderAllowlistVPxVM...)),
		}},
	})
}

func VideoEncodeAccelPerf(ctx context.Context, s *testing.State) {
	video.RunARCPerfVideoTest(ctx, s, s.FixtValue().(*arc.PreData).ARC,
		s.Param().(video.EncodeTestOptions), veapCacheExtractedVideo)
}
