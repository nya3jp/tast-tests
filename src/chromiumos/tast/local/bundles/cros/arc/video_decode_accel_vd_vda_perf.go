// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/local/media/decoding"
	"chromiumos/tast/testing"
)

type videoDecodeAccelVDVDAPerfTestParam struct {
	dataPath        string
	useLinearOutput bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         VideoDecodeAccelVDVDAPerf,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Measures performance of hardware decode acceleration performance using media::VideoDecoder through the VDA interface, by running the video_decode_accelerator_perf_tests binary (see go/vd-migration)",
		Contacts:     []string{"dstaessens@chromium.org", "chromeos-video-eng@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"arc", "chrome", "video_decoder_direct"},
		Timeout:      5 * time.Minute,
		Params: []testing.Param{{
			Name:              "h264_1080p_30fps",
			Val:               videoDecodeAccelVDVDAPerfTestParam{dataPath: "1080p_30fps_300frames.h264"},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264},
			ExtraData:         []string{"1080p_30fps_300frames.h264", "1080p_30fps_300frames.h264.json"},
		}, {
			Name:              "h264_1080p_60fps",
			Val:               videoDecodeAccelVDVDAPerfTestParam{dataPath: "1080p_60fps_600frames.h264"},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264_60},
			ExtraData:         []string{"1080p_60fps_600frames.h264", "1080p_60fps_600frames.h264.json"},
		}, {
			Name:              "h264_2160p_30fps",
			Val:               videoDecodeAccelVDVDAPerfTestParam{dataPath: "2160p_30fps_300frames.h264"},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264_4K},
			ExtraData:         []string{"2160p_30fps_300frames.h264", "2160p_30fps_300frames.h264.json"},
		}, {
			Name:              "h264_2160p_60fps",
			Val:               videoDecodeAccelVDVDAPerfTestParam{dataPath: "2160p_60fps_600frames.h264"},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264_4K60},
			ExtraData:         []string{"2160p_60fps_600frames.h264", "2160p_60fps_600frames.h264.json"},
		}, {
			Name:              "vp8_1080p_30fps",
			Val:               videoDecodeAccelVDVDAPerfTestParam{dataPath: "1080p_30fps_300frames.vp8.ivf"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8},
			ExtraData:         []string{"1080p_30fps_300frames.vp8.ivf", "1080p_30fps_300frames.vp8.ivf.json"},
		}, {
			Name:              "vp8_1080p_60fps",
			Val:               videoDecodeAccelVDVDAPerfTestParam{dataPath: "1080p_60fps_600frames.vp8.ivf"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8_60},
			ExtraData:         []string{"1080p_60fps_600frames.vp8.ivf", "1080p_60fps_600frames.vp8.ivf.json"},
		}, {
			Name:              "vp8_2160p_30fps",
			Val:               videoDecodeAccelVDVDAPerfTestParam{dataPath: "2160p_30fps_300frames.vp8.ivf"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8_4K},
			ExtraData:         []string{"2160p_30fps_300frames.vp8.ivf", "2160p_30fps_300frames.vp8.ivf.json"},
		}, {
			Name:              "vp8_2160p_60fps",
			Val:               videoDecodeAccelVDVDAPerfTestParam{dataPath: "2160p_60fps_600frames.vp8.ivf"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8_4K60},
			ExtraData:         []string{"2160p_60fps_600frames.vp8.ivf", "2160p_60fps_600frames.vp8.ivf.json"},
		}, {
			Name:              "vp9_1080p_30fps",
			Val:               videoDecodeAccelVDVDAPerfTestParam{dataPath: "1080p_30fps_300frames.vp9.ivf"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
			ExtraData:         []string{"1080p_30fps_300frames.vp9.ivf", "1080p_30fps_300frames.vp9.ivf.json"},
		}, {
			Name:              "vp9_1080p_60fps",
			Val:               videoDecodeAccelVDVDAPerfTestParam{dataPath: "1080p_60fps_600frames.vp9.ivf"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9_60},
			ExtraData:         []string{"1080p_60fps_600frames.vp9.ivf", "1080p_60fps_600frames.vp9.ivf.json"},
		}, {
			Name:              "vp9_2160p_30fps",
			Val:               videoDecodeAccelVDVDAPerfTestParam{dataPath: "2160p_30fps_300frames.vp9.ivf"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9_4K},
			ExtraData:         []string{"2160p_30fps_300frames.vp9.ivf", "2160p_30fps_300frames.vp9.ivf.json"},
		}, {
			Name:              "vp9_2160p_60fps",
			Val:               videoDecodeAccelVDVDAPerfTestParam{dataPath: "2160p_60fps_600frames.vp9.ivf"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9_4K60},
			ExtraData:         []string{"2160p_60fps_600frames.vp9.ivf", "2160p_60fps_600frames.vp9.ivf.json"},
		},
			{
				Name:              "h264_linear_output_1080p_30fps",
				Val:               videoDecodeAccelVDVDAPerfTestParam{dataPath: "1080p_30fps_300frames.h264", useLinearOutput: true},
				ExtraSoftwareDeps: []string{caps.HWDecodeH264},
				ExtraData:         []string{"1080p_30fps_300frames.h264", "1080p_30fps_300frames.h264.json"},
			},
			{
				Name:              "vp8_linear_output_1080p_30fps",
				Val:               videoDecodeAccelVDVDAPerfTestParam{dataPath: "1080p_30fps_300frames.vp8.ivf", useLinearOutput: true},
				ExtraSoftwareDeps: []string{caps.HWDecodeVP8},
				ExtraData:         []string{"1080p_30fps_300frames.vp8.ivf", "1080p_30fps_300frames.vp8.ivf.json"},
			}, {
				Name:              "vp9_linear_output_1080p_30fps",
				Val:               videoDecodeAccelVDVDAPerfTestParam{dataPath: "1080p_30fps_300frames.vp9.ivf", useLinearOutput: true},
				ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
				ExtraData:         []string{"1080p_30fps_300frames.vp9.ivf", "1080p_30fps_300frames.vp9.ivf.json"},
			}},
	})
}

func VideoDecodeAccelVDVDAPerf(ctx context.Context, s *testing.State) {
	param := s.Param().(videoDecodeAccelVDVDAPerfTestParam)
	if err := decoding.RunAccelVideoPerfTest(ctx, s.OutDir(), s.DataPath(param.dataPath), decoding.TestParams{DecoderType: decoding.VDVDA, LinearOutput: param.useLinearOutput}); err != nil {
		s.Fatal("test failed: ", err)
	}
}
