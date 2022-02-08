// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"

	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/local/media/decoding"
	"chromiumos/tast/testing"
)

type videoDecodeAccelPerfTestParam struct {
	dataPath               string
	disableGlobalVaapiLock bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         DecodeAccelPerf,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Measures hardware video decode performance by running the video_decode_accelerator_perf_tests binary",
		Contacts: []string{
			"mcasas@chromium.org",
			"chromeos-gfx-video@google.com",
		},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome", "video_decoder_legacy_supported"},
		Params: []testing.Param{{
			Name:              "h264_1080p_30fps",
			Val:               videoDecodeAccelPerfTestParam{dataPath: "1080p_30fps_300frames.h264"},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "proprietary_codecs"},
			ExtraData:         []string{"1080p_30fps_300frames.h264", "1080p_30fps_300frames.h264.json"},
		}, {
			Name:              "h264_1080p_60fps",
			Val:               videoDecodeAccelPerfTestParam{dataPath: "1080p_60fps_600frames.h264"},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264_60, "proprietary_codecs"},
			ExtraData:         []string{"1080p_60fps_600frames.h264", "1080p_60fps_600frames.h264.json"},
		}, {
			Name:              "h264_1080p_60fps_global_vaapi_lock_disabled",
			Val:               videoDecodeAccelPerfTestParam{dataPath: "1080p_60fps_600frames.h264", disableGlobalVaapiLock: true},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264_60, "proprietary_codecs", "thread_safe_libva_backend"},
			ExtraData:         []string{"1080p_60fps_600frames.h264", "1080p_60fps_600frames.h264.json"},
		}, {
			Name:              "h264_2160p_30fps",
			Val:               videoDecodeAccelPerfTestParam{dataPath: "2160p_30fps_300frames.h264"},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264_4K, "proprietary_codecs"},
			ExtraData:         []string{"2160p_30fps_300frames.h264", "2160p_30fps_300frames.h264.json"},
		}, {
			Name:              "h264_2160p_60fps",
			Val:               videoDecodeAccelPerfTestParam{dataPath: "2160p_60fps_600frames.h264"},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264_4K60, "proprietary_codecs"},
			ExtraData:         []string{"2160p_60fps_600frames.h264", "2160p_60fps_600frames.h264.json"},
		}, {
			Name:              "vp8_1080p_30fps",
			Val:               videoDecodeAccelPerfTestParam{dataPath: "1080p_30fps_300frames.vp8.ivf"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8},
			ExtraData:         []string{"1080p_30fps_300frames.vp8.ivf", "1080p_30fps_300frames.vp8.ivf.json"},
		}, {
			Name:              "vp8_1080p_60fps",
			Val:               videoDecodeAccelPerfTestParam{dataPath: "1080p_60fps_600frames.vp8.ivf"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8_60},
			ExtraData:         []string{"1080p_60fps_600frames.vp8.ivf", "1080p_60fps_600frames.vp8.ivf.json"},
		}, {
			Name:              "vp8_1080p_60fps_global_vaapi_lock_disabled",
			Val:               videoDecodeAccelPerfTestParam{dataPath: "1080p_60fps_600frames.vp8.ivf", disableGlobalVaapiLock: true},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8_60, "thread_safe_libva_backend"},
			ExtraData:         []string{"1080p_60fps_600frames.vp8.ivf", "1080p_60fps_600frames.vp8.ivf.json"},
		}, {
			Name:              "vp8_2160p_30fps",
			Val:               videoDecodeAccelPerfTestParam{dataPath: "2160p_30fps_300frames.vp8.ivf"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8_4K},
			ExtraData:         []string{"2160p_30fps_300frames.vp8.ivf", "2160p_30fps_300frames.vp8.ivf.json"},
		}, {
			Name:              "vp8_2160p_60fps",
			Val:               videoDecodeAccelPerfTestParam{dataPath: "2160p_60fps_600frames.vp8.ivf"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8_4K60},
			ExtraData:         []string{"2160p_60fps_600frames.vp8.ivf", "2160p_60fps_600frames.vp8.ivf.json"},
		}, {
			Name:              "vp9_1080p_30fps",
			Val:               videoDecodeAccelPerfTestParam{dataPath: "1080p_30fps_300frames.vp9.ivf"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
			ExtraData:         []string{"1080p_30fps_300frames.vp9.ivf", "1080p_30fps_300frames.vp9.ivf.json"},
		}, {
			Name:              "vp9_1080p_60fps",
			Val:               videoDecodeAccelPerfTestParam{dataPath: "1080p_60fps_600frames.vp9.ivf"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9_60},
			ExtraData:         []string{"1080p_60fps_600frames.vp9.ivf", "1080p_60fps_600frames.vp9.ivf.json"},
		}, {
			Name:              "vp9_1080p_60fps_global_vaapi_lock_disabled",
			Val:               videoDecodeAccelPerfTestParam{dataPath: "1080p_60fps_600frames.vp9.ivf", disableGlobalVaapiLock: true},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9_60, "thread_safe_libva_backend"},
			ExtraData:         []string{"1080p_60fps_600frames.vp9.ivf", "1080p_60fps_600frames.vp9.ivf.json"},
		}, {
			Name:              "vp9_2160p_30fps",
			Val:               videoDecodeAccelPerfTestParam{dataPath: "2160p_30fps_300frames.vp9.ivf"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9_4K},
			ExtraData:         []string{"2160p_30fps_300frames.vp9.ivf", "2160p_30fps_300frames.vp9.ivf.json"},
		}, {
			Name:              "vp9_2160p_60fps",
			Val:               videoDecodeAccelPerfTestParam{dataPath: "2160p_60fps_600frames.vp9.ivf"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9_4K60},
			ExtraData:         []string{"2160p_60fps_600frames.vp9.ivf", "2160p_60fps_600frames.vp9.ivf.json"},
		}},
	})
}

func DecodeAccelPerf(ctx context.Context, s *testing.State) {
	param := s.Param().(videoDecodeAccelPerfTestParam)

	if err := decoding.RunAccelVideoPerfTest(ctx, s.OutDir(), s.DataPath(param.dataPath), decoding.TestParams{DecoderType: decoding.VDA, DisableGlobalVaapiLock: param.disableGlobalVaapiLock}); err != nil {
		s.Fatal("test failed: ", err)
	}
}
