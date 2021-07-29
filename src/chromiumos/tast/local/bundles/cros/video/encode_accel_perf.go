// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"
	"time"

	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/local/bundles/cros/video/encode"
	"chromiumos/tast/local/media/videotype"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

const (
	crowd180p  = "crowd-320x180_30frames.vp9.webm"
	crowd360p  = "crowd-640x360_30frames.vp9.webm"
	crowd720p  = "crowd-1280x720_30frames.vp9.webm"
	crowd1080p = "crowd-1920x1080_30frames.vp9.webm"
	crowd2160p = "crowd-3840x2160_30frames.vp9.webm"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         EncodeAccelPerf,
		Desc:         "Measures hardware video encode performance by running the video_encode_accelerator_perf_tests binary",
		Contacts:     []string{"hiroh@chromium.org", "chromeos-gfx-video@google.com"},
		Attr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
		SoftwareDeps: []string{"chrome"},
		// Default timeout (i.e. 2 minutes) is not enough.
		Timeout: 10 * time.Minute,
		Params: []testing.Param{{
			Name:              "h264_180p",
			Val:               encode.MakeTestOptions(crowd180p, videotype.H264BaselineProf),
			ExtraData:         encode.TestData(crowd180p),
			ExtraSoftwareDeps: []string{caps.HWEncodeH264},
		}, {
			Name:              "h264_360p",
			Val:               encode.MakeTestOptions(crowd360p, videotype.H264BaselineProf),
			ExtraData:         encode.TestData(crowd360p),
			ExtraSoftwareDeps: []string{caps.HWEncodeH264},
		}, {
			Name:              "h264_720p",
			Val:               encode.MakeTestOptions(crowd720p, videotype.H264BaselineProf),
			ExtraData:         encode.TestData(crowd720p),
			ExtraSoftwareDeps: []string{caps.HWEncodeH264},
		}, {
			Name:              "h264_1080p",
			Val:               encode.MakeTestOptions(crowd1080p, videotype.H264BaselineProf),
			ExtraData:         encode.TestData(crowd1080p),
			ExtraSoftwareDeps: []string{caps.HWEncodeH264},
		}, {
			Name:              "h264_2160p",
			Val:               encode.MakeTestOptions(crowd2160p, videotype.H264BaselineProf),
			ExtraData:         encode.TestData(crowd2160p),
			ExtraSoftwareDeps: []string{caps.HWEncodeH264_4K},
		}, {
			Name:              "h264_1080p_baseline_x2",
			Val:               encode.MakeBitrateTestOptions(crowd1080p, videotype.H264BaselineProf, 1920*1080*2),
			ExtraData:         encode.TestData(crowd1080p),
			ExtraSoftwareDeps: []string{caps.HWEncodeH264_4K},
		}, {
			Name:              "h264_1080p_baseline_x4",
			Val:               encode.MakeBitrateTestOptions(crowd1080p, videotype.H264BaselineProf, 1920*1080*4),
			ExtraData:         encode.TestData(crowd1080p),
			ExtraSoftwareDeps: []string{caps.HWEncodeH264_4K},
		}, {
			Name:              "h264_1080p_baseline_x6",
			Val:               encode.MakeBitrateTestOptions(crowd1080p, videotype.H264BaselineProf, 1920*1080*6),
			ExtraData:         encode.TestData(crowd1080p),
			ExtraSoftwareDeps: []string{caps.HWEncodeH264_4K},
		}, {
			Name:              "h264_1080p_baseline_x8",
			Val:               encode.MakeBitrateTestOptions(crowd1080p, videotype.H264BaselineProf, 1920*1080*8),
			ExtraData:         encode.TestData(crowd1080p),
			ExtraSoftwareDeps: []string{caps.HWEncodeH264_4K},
		}, {
			Name:              "h264_1080p_main_x2",
			Val:               encode.MakeBitrateTestOptions(crowd1080p, videotype.H264MainProf, 1920*1080*2),
			ExtraData:         encode.TestData(crowd1080p),
			ExtraSoftwareDeps: []string{caps.HWEncodeH264_4K},
		}, {
			Name:              "h264_1080p_main_x4",
			Val:               encode.MakeBitrateTestOptions(crowd1080p, videotype.H264MainProf, 1920*1080*4),
			ExtraData:         encode.TestData(crowd1080p),
			ExtraSoftwareDeps: []string{caps.HWEncodeH264_4K},
		}, {
			Name:              "h264_1080p_main_x6",
			Val:               encode.MakeBitrateTestOptions(crowd1080p, videotype.H264MainProf, 1920*1080*6),
			ExtraData:         encode.TestData(crowd1080p),
			ExtraSoftwareDeps: []string{caps.HWEncodeH264_4K},
		}, {
			Name:              "h264_1080p_main_x8",
			Val:               encode.MakeBitrateTestOptions(crowd1080p, videotype.H264MainProf, 1920*1080*8),
			ExtraData:         encode.TestData(crowd1080p),
			ExtraSoftwareDeps: []string{caps.HWEncodeH264_4K},
		}, {
			Name:              "h264_1080p_high_x2",
			Val:               encode.MakeBitrateTestOptions(crowd1080p, videotype.H264HighProf, 1920*1080*2),
			ExtraData:         encode.TestData(crowd1080p),
			ExtraSoftwareDeps: []string{caps.HWEncodeH264_4K},
		}, {
			Name:              "h264_1080p_high_x4",
			Val:               encode.MakeBitrateTestOptions(crowd1080p, videotype.H264HighProf, 1920*1080*4),
			ExtraData:         encode.TestData(crowd1080p),
			ExtraSoftwareDeps: []string{caps.HWEncodeH264_4K},
		}, {
			Name:              "h264_1080p_high_x6",
			Val:               encode.MakeBitrateTestOptions(crowd1080p, videotype.H264HighProf, 1920*1080*6),
			ExtraData:         encode.TestData(crowd1080p),
			ExtraSoftwareDeps: []string{caps.HWEncodeH264_4K},
		}, {
			Name:              "h264_1080p_high_x8",
			Val:               encode.MakeBitrateTestOptions(crowd1080p, videotype.H264HighProf, 1920*1080*8),
			ExtraData:         encode.TestData(crowd1080p),
			ExtraSoftwareDeps: []string{caps.HWEncodeH264_4K},
		}, {
			Name:              "vp8_180p",
			Val:               encode.MakeTestOptions(crowd180p, videotype.VP8Prof),
			ExtraData:         encode.TestData(crowd180p),
			ExtraSoftwareDeps: []string{caps.HWEncodeVP8},
		}, {
			Name:              "vp8_360p",
			Val:               encode.MakeTestOptions(crowd360p, videotype.VP8Prof),
			ExtraData:         encode.TestData(crowd360p),
			ExtraSoftwareDeps: []string{caps.HWEncodeVP8},
		}, {
			Name:              "vp8_720p",
			Val:               encode.MakeTestOptions(crowd720p, videotype.VP8Prof),
			ExtraData:         encode.TestData(crowd720p),
			ExtraSoftwareDeps: []string{caps.HWEncodeVP8},
		}, {
			Name:              "vp8_1080p",
			Val:               encode.MakeTestOptions(crowd1080p, videotype.VP8Prof),
			ExtraData:         encode.TestData(crowd1080p),
			ExtraSoftwareDeps: []string{caps.HWEncodeVP8},
		}, {
			Name:              "vp8_2160p",
			Val:               encode.MakeTestOptions(crowd2160p, videotype.VP8Prof),
			ExtraData:         encode.TestData(crowd2160p),
			ExtraSoftwareDeps: []string{caps.HWEncodeVP8_4K},
		}, {
			Name:              "vp9_180p",
			Val:               encode.MakeTestOptions(crowd180p, videotype.VP9Prof),
			ExtraData:         encode.TestData(crowd180p),
			ExtraSoftwareDeps: []string{caps.HWEncodeVP9},
		}, {
			Name:              "vp9_360p",
			Val:               encode.MakeTestOptions(crowd360p, videotype.VP9Prof),
			ExtraData:         encode.TestData(crowd360p),
			ExtraSoftwareDeps: []string{caps.HWEncodeVP9},
		}, {
			Name:              "vp9_720p",
			Val:               encode.MakeTestOptions(crowd720p, videotype.VP9Prof),
			ExtraData:         encode.TestData(crowd720p),
			ExtraSoftwareDeps: []string{caps.HWEncodeVP9},
		}, {
			Name:              "vp9_720p_l1t2",
			Val:               encode.MakeTestOptionsWithSVCLayers(crowd720p, videotype.VP9Prof, "L1T2"),
			ExtraSoftwareDeps: []string{caps.HWEncodeVP9, "vaapi"},
			ExtraData:         encode.TestData(crowd720p),
		}, {
			Name:              "vp9_720p_l1t3",
			Val:               encode.MakeTestOptionsWithSVCLayers(crowd720p, videotype.VP9Prof, "L1T3"),
			ExtraData:         encode.TestData(crowd720p),
			ExtraSoftwareDeps: []string{caps.HWEncodeVP9, "vaapi"},
		}, {
			Name:              "vp9_720p_l2t3",
			Val:               encode.MakeTestOptionsWithSVCLayers(crowd720p, videotype.VP9Prof, "L2T3"),
			ExtraData:         encode.TestData(crowd720p),
			ExtraHardwareDeps: hwdep.D(hwdep.Platform("volteer", "dedede")),
			ExtraSoftwareDeps: []string{caps.HWEncodeVP9, "vaapi"},
		}, {
			Name:              "vp9_720p_l3t3",
			Val:               encode.MakeTestOptionsWithSVCLayers(crowd720p, videotype.VP9Prof, "L3T3"),
			ExtraData:         encode.TestData(crowd720p),
			ExtraHardwareDeps: hwdep.D(hwdep.Platform("volteer", "dedede")),
			ExtraSoftwareDeps: []string{caps.HWEncodeVP9, "vaapi"},
		}, {
			Name:              "vp9_1080p",
			Val:               encode.MakeTestOptions(crowd1080p, videotype.VP9Prof),
			ExtraData:         encode.TestData(crowd1080p),
			ExtraSoftwareDeps: []string{caps.HWEncodeVP9},
		}, {
			Name:              "vp9_2160p",
			Val:               encode.MakeTestOptions(crowd2160p, videotype.VP9Prof),
			ExtraData:         encode.TestData(crowd2160p),
			ExtraSoftwareDeps: []string{caps.HWEncodeVP9_4K},
		}},
	})
}

func EncodeAccelPerf(ctx context.Context, s *testing.State) {
	if err := encode.RunAccelVideoPerfTest(ctx, s, s.Param().(encode.TestOptions)); err != nil {
		s.Fatal("test failed: ", err)
	}
}
