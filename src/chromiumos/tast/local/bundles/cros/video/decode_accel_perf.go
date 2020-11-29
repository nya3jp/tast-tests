// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"

	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/local/media/decoding"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DecodeAccelPerf,
		Desc:         "Measures hardware video decode performance by running the video_decode_accelerator_perf_tests binary",
		Contacts:     []string{"dstaessens@chromium.org", "chromeos-video-eng@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name:              "h264_1080p_30fps",
			Val:               "1080p_30fps_300frames.h264",
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "video_decoder_legacy_supported"},
			ExtraData:         []string{"1080p_30fps_300frames.h264", "1080p_30fps_300frames.h264.json"},
		}, {
			Name:              "h264_1080p_60fps",
			Val:               "1080p_60fps_600frames.h264",
			ExtraSoftwareDeps: []string{caps.HWDecodeH264_60, "video_decoder_legacy_supported"},
			ExtraData:         []string{"1080p_60fps_600frames.h264", "1080p_60fps_600frames.h264.json"},
		}, {
			Name:              "h264_2160p_30fps",
			Val:               "2160p_30fps_300frames.h264",
			ExtraSoftwareDeps: []string{caps.HWDecodeH264_4K, "video_decoder_legacy_supported"},
			ExtraData:         []string{"2160p_30fps_300frames.h264", "2160p_30fps_300frames.h264.json"},
		}, {
			Name:              "h264_2160p_60fps",
			Val:               "2160p_60fps_600frames.h264",
			ExtraSoftwareDeps: []string{caps.HWDecodeH264_4K60, "video_decoder_legacy_supported"},
			ExtraData:         []string{"2160p_60fps_600frames.h264", "2160p_60fps_600frames.h264.json"},
		}, {
			Name:              "vp8_1080p_30fps",
			Val:               "1080p_30fps_300frames.vp8.ivf",
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8, "video_decoder_legacy_supported"},
			ExtraData:         []string{"1080p_30fps_300frames.vp8.ivf", "1080p_30fps_300frames.vp8.ivf.json"},
		}, {
			Name:              "vp8_1080p_60fps",
			Val:               "1080p_60fps_600frames.vp8.ivf",
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8_60, "video_decoder_legacy_supported"},
			ExtraData:         []string{"1080p_60fps_600frames.vp8.ivf", "1080p_60fps_600frames.vp8.ivf.json"},
		}, {
			Name:              "vp8_2160p_30fps",
			Val:               "2160p_30fps_300frames.vp8.ivf",
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8_4K, "video_decoder_legacy_supported"},
			ExtraData:         []string{"2160p_30fps_300frames.vp8.ivf", "2160p_30fps_300frames.vp8.ivf.json"},
		}, {
			Name:              "vp8_2160p_60fps",
			Val:               "2160p_60fps_600frames.vp8.ivf",
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8_4K60, "video_decoder_legacy_supported"},
			ExtraData:         []string{"2160p_60fps_600frames.vp8.ivf", "2160p_60fps_600frames.vp8.ivf.json"},
		}, {
			Name:              "vp9_1080p_30fps",
			Val:               "1080p_30fps_300frames.vp9.ivf",
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9, "video_decoder_legacy_supported"},
			ExtraData:         []string{"1080p_30fps_300frames.vp9.ivf", "1080p_30fps_300frames.vp9.ivf.json"},
		}, {
			Name:              "vp9_1080p_60fps",
			Val:               "1080p_60fps_600frames.vp9.ivf",
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9_60, "video_decoder_legacy_supported"},
			ExtraData:         []string{"1080p_60fps_600frames.vp9.ivf", "1080p_60fps_600frames.vp9.ivf.json"},
		}, {
			Name:              "vp9_2160p_30fps",
			Val:               "2160p_30fps_300frames.vp9.ivf",
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9_4K, "video_decoder_legacy_supported"},
			ExtraData:         []string{"2160p_30fps_300frames.vp9.ivf", "2160p_30fps_300frames.vp9.ivf.json"},
		}, {
			Name:              "vp9_2160p_60fps",
			Val:               "2160p_60fps_600frames.vp9.ivf",
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9_4K60, "video_decoder_legacy_supported"},
			ExtraData:         []string{"2160p_60fps_600frames.vp9.ivf", "2160p_60fps_600frames.vp9.ivf.json"},
		}},
	})
}

func DecodeAccelPerf(ctx context.Context, s *testing.State) {
	if err := decoding.RunAccelVideoPerfTest(ctx, s.OutDir(), s.DataPath(s.Param().(string)), decoding.VDA); err != nil {
		s.Fatal("test failed: ", err)
	}
}
