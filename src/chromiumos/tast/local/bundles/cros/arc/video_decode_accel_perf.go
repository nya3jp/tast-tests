// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"

	"chromiumos/tast/local/bundles/cros/arc/c2e2etest"
	"chromiumos/tast/local/bundles/cros/arc/video"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VideoDecodeAccelPerf,
		Desc:         "Measures ARC++ hardware video decode performance by running the c2_e2e_test APK",
		Contacts:     []string{"akahuang@chromium.org", "chromeos-video-eng@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		Data:         []string{c2e2etest.X86ApkName, c2e2etest.ArmApkName},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "arcBooted",
		Timeout:      video.PerfTestRuntime,
		Params: []testing.Param{{
			Name:              "h264_1080p_30fps",
			Val:               video.DecodeTestOptions{TestVideo: "1080p_30fps_300frames.h264"},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "android_p"},
			ExtraData:         []string{"1080p_30fps_300frames.h264", "1080p_30fps_300frames.h264.json"},
		}, {
			Name:              "h264_1080p_30fps_vm",
			Val:               video.DecodeTestOptions{TestVideo: "1080p_30fps_300frames.h264"},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "android_vm"},
			ExtraData:         []string{"1080p_30fps_300frames.h264", "1080p_30fps_300frames.h264.json"},
		}, {
			Name:              "h264_1080p_60fps",
			Val:               video.DecodeTestOptions{TestVideo: "1080p_60fps_600frames.h264"},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264_60, "android_p"},
			ExtraData:         []string{"1080p_60fps_600frames.h264", "1080p_60fps_600frames.h264.json"},
		}, {
			Name:              "h264_1080p_60fps_vm",
			Val:               video.DecodeTestOptions{TestVideo: "1080p_60fps_600frames.h264"},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264_60, "android_vm"},
			ExtraData:         []string{"1080p_60fps_600frames.h264", "1080p_60fps_600frames.h264.json"},
		}, {
			Name:              "h264_2160p_30fps",
			Val:               video.DecodeTestOptions{TestVideo: "2160p_30fps_300frames.h264"},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264_4K, "android_p"},
			ExtraData:         []string{"2160p_30fps_300frames.h264", "2160p_30fps_300frames.h264.json"},
		}, {
			Name:              "h264_2160p_30fps_vm",
			Val:               video.DecodeTestOptions{TestVideo: "2160p_30fps_300frames.h264"},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264_4K, "android_vm"},
			ExtraData:         []string{"2160p_30fps_300frames.h264", "2160p_30fps_300frames.h264.json"},
		}, {
			Name:              "h264_2160p_60fps",
			Val:               video.DecodeTestOptions{TestVideo: "2160p_60fps_600frames.h264"},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264_4K60, "android_p"},
			ExtraData:         []string{"2160p_60fps_600frames.h264", "2160p_60fps_600frames.h264.json"},
		}, {
			Name:              "h264_2160p_60fps_vm",
			Val:               video.DecodeTestOptions{TestVideo: "2160p_60fps_600frames.h264"},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264_4K60, "android_vm"},
			ExtraData:         []string{"2160p_60fps_600frames.h264", "2160p_60fps_600frames.h264.json"},
		}, {
			Name:              "vp8_1080p_30fps",
			Val:               video.DecodeTestOptions{TestVideo: "1080p_30fps_300frames.vp8.ivf"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8, "android_p"},
			ExtraData:         []string{"1080p_30fps_300frames.vp8.ivf", "1080p_30fps_300frames.vp8.ivf.json"},
		}, {
			Name:              "vp8_1080p_30fps_vm",
			Val:               video.DecodeTestOptions{TestVideo: "1080p_30fps_300frames.vp8.ivf"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8, "android_vm"},
			ExtraData:         []string{"1080p_30fps_300frames.vp8.ivf", "1080p_30fps_300frames.vp8.ivf.json"},
		}, {
			Name:              "vp8_1080p_60fps",
			Val:               video.DecodeTestOptions{TestVideo: "1080p_60fps_600frames.vp8.ivf"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8_60, "android_p"},
			ExtraData:         []string{"1080p_60fps_600frames.vp8.ivf", "1080p_60fps_600frames.vp8.ivf.json"},
		}, {
			Name:              "vp8_1080p_60fps_vm",
			Val:               video.DecodeTestOptions{TestVideo: "1080p_60fps_600frames.vp8.ivf"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8_60, "android_vm"},
			ExtraData:         []string{"1080p_60fps_600frames.vp8.ivf", "1080p_60fps_600frames.vp8.ivf.json"},
		}, {
			Name:              "vp8_2160p_30fps",
			Val:               video.DecodeTestOptions{TestVideo: "2160p_30fps_300frames.vp8.ivf"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8_4K, "android_p"},
			ExtraData:         []string{"2160p_30fps_300frames.vp8.ivf", "2160p_30fps_300frames.vp8.ivf.json"},
		}, {
			Name:              "vp8_2160p_30fps_vm",
			Val:               video.DecodeTestOptions{TestVideo: "2160p_30fps_300frames.vp8.ivf"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8_4K, "android_vm"},
			ExtraData:         []string{"2160p_30fps_300frames.vp8.ivf", "2160p_30fps_300frames.vp8.ivf.json"},
		}, {
			Name:              "vp8_2160p_60fps",
			Val:               video.DecodeTestOptions{TestVideo: "2160p_60fps_600frames.vp8.ivf"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8_4K60, "android_p"},
			ExtraData:         []string{"2160p_60fps_600frames.vp8.ivf", "2160p_60fps_600frames.vp8.ivf.json"},
		}, {
			Name:              "vp8_2160p_60fps_vm",
			Val:               video.DecodeTestOptions{TestVideo: "2160p_60fps_600frames.vp8.ivf"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8_4K60, "android_vm"},
			ExtraData:         []string{"2160p_60fps_600frames.vp8.ivf", "2160p_60fps_600frames.vp8.ivf.json"},
		}, {
			Name:              "vp9_1080p_30fps",
			Val:               video.DecodeTestOptions{TestVideo: "1080p_30fps_300frames.vp9.ivf"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9, "android_p"},
			ExtraData:         []string{"1080p_30fps_300frames.vp9.ivf", "1080p_30fps_300frames.vp9.ivf.json"},
		}, {
			Name:              "vp9_1080p_30fps_vm",
			Val:               video.DecodeTestOptions{TestVideo: "1080p_30fps_300frames.vp9.ivf"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9, "android_vm"},
			ExtraData:         []string{"1080p_30fps_300frames.vp9.ivf", "1080p_30fps_300frames.vp9.ivf.json"},
		}, {
			Name:              "vp9_1080p_60fps",
			Val:               video.DecodeTestOptions{TestVideo: "1080p_60fps_600frames.vp9.ivf"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9_60, "android_p"},
			ExtraData:         []string{"1080p_60fps_600frames.vp9.ivf", "1080p_60fps_600frames.vp9.ivf.json"},
		}, {
			Name:              "vp9_1080p_60fps_vm",
			Val:               video.DecodeTestOptions{TestVideo: "1080p_60fps_600frames.vp9.ivf"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9_60, "android_vm"},
			ExtraData:         []string{"1080p_60fps_600frames.vp9.ivf", "1080p_60fps_600frames.vp9.ivf.json"},
		}, {
			Name:              "vp9_2160p_30fps",
			Val:               video.DecodeTestOptions{TestVideo: "2160p_30fps_300frames.vp9.ivf"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9_4K, "android_p"},
			ExtraData:         []string{"2160p_30fps_300frames.vp9.ivf", "2160p_30fps_300frames.vp9.ivf.json"},
		}, {
			Name:              "vp9_2160p_30fps_vm",
			Val:               video.DecodeTestOptions{TestVideo: "2160p_30fps_300frames.vp9.ivf"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9_4K, "android_vm"},
			ExtraData:         []string{"2160p_30fps_300frames.vp9.ivf", "2160p_30fps_300frames.vp9.ivf.json"},
		}, {
			Name:              "vp9_2160p_60fps",
			Val:               video.DecodeTestOptions{TestVideo: "2160p_60fps_600frames.vp9.ivf"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9_4K60, "android_p"},
			ExtraData:         []string{"2160p_60fps_600frames.vp9.ivf", "2160p_60fps_600frames.vp9.ivf.json"},
		}, {
			Name:              "vp9_2160p_60fps_vm",
			Val:               video.DecodeTestOptions{TestVideo: "2160p_60fps_600frames.vp9.ivf"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9_4K60, "android_vm"},
			ExtraData:         []string{"2160p_60fps_600frames.vp9.ivf", "2160p_60fps_600frames.vp9.ivf.json"},
		}},
	})
}

func VideoDecodeAccelPerf(ctx context.Context, s *testing.State) {
	video.RunARCVideoPerfTest(ctx, s, s.Param().(video.DecodeTestOptions))
}
