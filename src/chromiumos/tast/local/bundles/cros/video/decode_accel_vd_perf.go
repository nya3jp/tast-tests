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

type videoDecodeAccelVdPerfTestParam struct {
	dataPath               string
	disableGlobalVaapiLock bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         DecodeAccelVDPerf,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Measures hardware video decode performance of media::VideoDecoders by running the video_decode_accelerator_perf_tests binary (see go/vd-migration)",
		Contacts: []string{
			"mcasas@chromium.org",
			"hiroh@chromium.org", // Underlying binary author.
			"chromeos-gfx-video@google.com",
		},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name:              "av1_1080p_30fps",
			Val:               videoDecodeAccelVdPerfTestParam{dataPath: "1080p_30fps_300frames.av1.ivf"},
			ExtraSoftwareDeps: []string{caps.HWDecodeAV1},
			ExtraData:         []string{"1080p_30fps_300frames.av1.ivf", "1080p_30fps_300frames.av1.ivf.json"},
		}, {
			Name:              "av1_1080p_60fps",
			Val:               videoDecodeAccelVdPerfTestParam{dataPath: "1080p_60fps_600frames.av1.ivf"},
			ExtraSoftwareDeps: []string{caps.HWDecodeAV1_60},
			ExtraData:         []string{"1080p_60fps_600frames.av1.ivf", "1080p_60fps_600frames.av1.ivf.json"},
		}, {
			Name:              "av1_1080p_60fps_global_vaapi_lock_disabled",
			Val:               videoDecodeAccelVdPerfTestParam{dataPath: "1080p_60fps_600frames.av1.ivf", disableGlobalVaapiLock: true},
			ExtraSoftwareDeps: []string{caps.HWDecodeAV1_60, "thread_safe_libva_backend"},
			ExtraData:         []string{"1080p_60fps_600frames.av1.ivf", "1080p_60fps_600frames.av1.ivf.json"},
		}, {
			Name:              "av1_2160p_30fps",
			Val:               videoDecodeAccelVdPerfTestParam{dataPath: "2160p_30fps_300frames.av1.ivf"},
			ExtraSoftwareDeps: []string{caps.HWDecodeAV1_4K},
			ExtraData:         []string{"2160p_30fps_300frames.av1.ivf", "2160p_30fps_300frames.av1.ivf.json"},
		}, {
			Name:              "av1_2160p_60fps",
			Val:               videoDecodeAccelVdPerfTestParam{dataPath: "2160p_60fps_600frames.av1.ivf"},
			ExtraSoftwareDeps: []string{caps.HWDecodeAV1_4K60},
			ExtraData:         []string{"2160p_60fps_600frames.av1.ivf", "2160p_60fps_600frames.av1.ivf.json"},
		}, {
			Name:              "h264_1080p_30fps",
			Val:               videoDecodeAccelVdPerfTestParam{dataPath: "1080p_30fps_300frames.h264"},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "proprietary_codecs"},
			ExtraData:         []string{"1080p_30fps_300frames.h264", "1080p_30fps_300frames.h264.json"},
		}, {
			Name:              "h264_1080p_60fps",
			Val:               videoDecodeAccelVdPerfTestParam{dataPath: "1080p_60fps_600frames.h264"},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264_60, "proprietary_codecs"},
			ExtraData:         []string{"1080p_60fps_600frames.h264", "1080p_60fps_600frames.h264.json"},
		}, {
			Name:              "h264_1080p_60fps_global_vaapi_lock_disabled",
			Val:               videoDecodeAccelVdPerfTestParam{dataPath: "1080p_60fps_600frames.h264", disableGlobalVaapiLock: true},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264_60, "proprietary_codecs", "thread_safe_libva_backend"},
			ExtraData:         []string{"1080p_60fps_600frames.h264", "1080p_60fps_600frames.h264.json"},
		}, {
			Name:              "h264_2160p_30fps",
			Val:               videoDecodeAccelVdPerfTestParam{dataPath: "2160p_30fps_300frames.h264"},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264_4K, "proprietary_codecs"},
			ExtraData:         []string{"2160p_30fps_300frames.h264", "2160p_30fps_300frames.h264.json"},
		}, {
			Name:              "h264_2160p_60fps",
			Val:               videoDecodeAccelVdPerfTestParam{dataPath: "2160p_60fps_600frames.h264"},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264_4K60, "proprietary_codecs"},
			ExtraData:         []string{"2160p_60fps_600frames.h264", "2160p_60fps_600frames.h264.json"},
		}, {
			Name:              "hevc_1080p_30fps",
			Val:               videoDecodeAccelVdPerfTestParam{dataPath: "1080p_30fps_300frames.hevc"},
			ExtraSoftwareDeps: []string{caps.HWDecodeHEVC, "proprietary_codecs", "protected_content"},
			ExtraData:         []string{"1080p_30fps_300frames.hevc", "1080p_30fps_300frames.hevc.json"},
		}, {
			Name:              "hevc_1080p_60fps",
			Val:               videoDecodeAccelVdPerfTestParam{dataPath: "1080p_60fps_600frames.hevc"},
			ExtraSoftwareDeps: []string{caps.HWDecodeHEVC60, "proprietary_codecs", "protected_content"},
			ExtraData:         []string{"1080p_60fps_600frames.hevc", "1080p_60fps_600frames.hevc.json"},
		}, {
			Name:              "hevc_1080p_60fps_global_vaapi_lock_disabled",
			Val:               videoDecodeAccelVdPerfTestParam{dataPath: "1080p_60fps_600frames.hevc", disableGlobalVaapiLock: true},
			ExtraSoftwareDeps: []string{caps.HWDecodeHEVC60, "proprietary_codecs", "protected_content", "thread_safe_libva_backend"},
			ExtraData:         []string{"1080p_60fps_600frames.hevc", "1080p_60fps_600frames.hevc.json"},
		}, {
			Name:              "hevc_2160p_30fps",
			Val:               videoDecodeAccelVdPerfTestParam{dataPath: "2160p_30fps_300frames.hevc"},
			ExtraSoftwareDeps: []string{caps.HWDecodeHEVC4K, "proprietary_codecs", "protected_content"},
			ExtraData:         []string{"2160p_30fps_300frames.hevc", "2160p_30fps_300frames.hevc.json"},
		}, {
			Name:              "hevc_2160p_60fps",
			Val:               videoDecodeAccelVdPerfTestParam{dataPath: "2160p_60fps_600frames.hevc"},
			ExtraSoftwareDeps: []string{caps.HWDecodeHEVC4K60, "proprietary_codecs", "protected_content"},
			ExtraData:         []string{"2160p_60fps_600frames.hevc", "2160p_60fps_600frames.hevc.json"},
		}, {
			Name:              "vp8_1080p_30fps",
			Val:               videoDecodeAccelVdPerfTestParam{dataPath: "1080p_30fps_300frames.vp8.ivf"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8},
			ExtraData:         []string{"1080p_30fps_300frames.vp8.ivf", "1080p_30fps_300frames.vp8.ivf.json"},
		}, {
			Name:              "vp8_1080p_60fps",
			Val:               videoDecodeAccelVdPerfTestParam{dataPath: "1080p_60fps_600frames.vp8.ivf"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8_60},
			ExtraData:         []string{"1080p_60fps_600frames.vp8.ivf", "1080p_60fps_600frames.vp8.ivf.json"},
		}, {
			Name:              "vp8_1080p_60fps_global_vaapi_lock_disabled",
			Val:               videoDecodeAccelVdPerfTestParam{dataPath: "1080p_60fps_600frames.vp8.ivf", disableGlobalVaapiLock: true},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8_60, "thread_safe_libva_backend"},
			ExtraData:         []string{"1080p_60fps_600frames.vp8.ivf", "1080p_60fps_600frames.vp8.ivf.json"},
		}, {
			Name:              "vp8_2160p_30fps",
			Val:               videoDecodeAccelVdPerfTestParam{dataPath: "2160p_30fps_300frames.vp8.ivf"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8_4K},
			ExtraData:         []string{"2160p_30fps_300frames.vp8.ivf", "2160p_30fps_300frames.vp8.ivf.json"},
		}, {
			Name:              "vp8_2160p_60fps",
			Val:               videoDecodeAccelVdPerfTestParam{dataPath: "2160p_60fps_600frames.vp8.ivf"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8_4K60},
			ExtraData:         []string{"2160p_60fps_600frames.vp8.ivf", "2160p_60fps_600frames.vp8.ivf.json"},
		}, {
			Name:              "vp9_1080p_30fps",
			Val:               videoDecodeAccelVdPerfTestParam{dataPath: "1080p_30fps_300frames.vp9.ivf"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
			ExtraData:         []string{"1080p_30fps_300frames.vp9.ivf", "1080p_30fps_300frames.vp9.ivf.json"},
		}, {
			Name:              "vp9_1080p_60fps",
			Val:               videoDecodeAccelVdPerfTestParam{dataPath: "1080p_60fps_600frames.vp9.ivf"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9_60},
			ExtraData:         []string{"1080p_60fps_600frames.vp9.ivf", "1080p_60fps_600frames.vp9.ivf.json"},
		}, {
			Name:              "vp9_1080p_60fps_global_vaapi_lock_disabled",
			Val:               videoDecodeAccelVdPerfTestParam{dataPath: "1080p_60fps_600frames.vp9.ivf", disableGlobalVaapiLock: true},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9_60, "thread_safe_libva_backend"},
			ExtraData:         []string{"1080p_60fps_600frames.vp9.ivf", "1080p_60fps_600frames.vp9.ivf.json"},
		}, {
			Name:              "vp9_2160p_30fps",
			Val:               videoDecodeAccelVdPerfTestParam{dataPath: "2160p_30fps_300frames.vp9.ivf"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9_4K},
			ExtraData:         []string{"2160p_30fps_300frames.vp9.ivf", "2160p_30fps_300frames.vp9.ivf.json"},
		}, {
			Name:              "vp9_2160p_60fps",
			Val:               videoDecodeAccelVdPerfTestParam{dataPath: "2160p_60fps_600frames.vp9.ivf"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9_4K60},
			ExtraData:         []string{"2160p_60fps_600frames.vp9.ivf", "2160p_60fps_600frames.vp9.ivf.json"},
		}},
	})
}

func DecodeAccelVDPerf(ctx context.Context, s *testing.State) {
	param := s.Param().(videoDecodeAccelVdPerfTestParam)

	if err := decoding.RunAccelVideoPerfTest(ctx, s.OutDir(), s.DataPath(param.dataPath), decoding.TestParams{DecoderType: decoding.VD, DisableGlobalVaapiLock: param.disableGlobalVaapiLock}); err != nil {
		s.Fatal("test failed: ", err)
	}
}
