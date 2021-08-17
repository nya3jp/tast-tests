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
	tulip180P  = "tulip2-320x180.vp9.webm"
	bear192P   = "bear-320x192.vp9.webm"
	tulip360P  = "tulip2-640x360.vp9.webm"
	tulip361P  = "crowd-641x361.vp9.webm"
	tulip540P  = "tulip2-960x540.vp9.webm"
	tulip720P  = "tulip2-1280x720.vp9.webm"
	crowd1080P = "crowd-1920x1080.vp9.webm"
	crowd2160P = "crowd-3840x2160.vp9.webm"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         EncodeAccel,
		Desc:         "Verifies hardware encode acceleration by running the video_encode_accelerator_tests binary",
		Contacts:     []string{"hiroh@chromium.org", "chromeos-gfx-video@google.com"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      10 * time.Minute,
		Attr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
		Params: []testing.Param{{
			Name:              "h264_180p",
			Val:               encode.MakeTestOptions(tulip180P, videotype.H264BaselineProf),
			ExtraSoftwareDeps: []string{caps.HWEncodeH264},
			ExtraData:         encode.TestData(tulip180P),
		}, {
			Name:              "h264_192p",
			Val:               encode.MakeTestOptions(bear192P, videotype.H264BaselineProf),
			ExtraSoftwareDeps: []string{caps.HWEncodeH264},
			ExtraData:         encode.TestData(bear192P),
		}, {
			Name:              "h264_360p",
			Val:               encode.MakeTestOptions(tulip360P, videotype.H264BaselineProf),
			ExtraSoftwareDeps: []string{caps.HWEncodeH264},
			ExtraData:         encode.TestData(tulip360P),
		}, {
			Name:              "h264_720p",
			Val:               encode.MakeTestOptions(tulip720P, videotype.H264BaselineProf),
			ExtraSoftwareDeps: []string{caps.HWEncodeH264},
			ExtraData:         encode.TestData(tulip720P),
		}, {
			Name:              "h264_1080p",
			Val:               encode.MakeTestOptions(crowd1080P, videotype.H264BaselineProf),
			ExtraSoftwareDeps: []string{caps.HWEncodeH264},
			ExtraData:         encode.TestData(crowd1080P),
		}, {
			Name:              "h264_2160p",
			Val:               encode.MakeTestOptions(crowd2160P, videotype.H264BaselineProf),
			ExtraSoftwareDeps: []string{caps.HWEncodeH264_4K},
			ExtraData:         encode.TestData(crowd2160P),
		}, {
			Name:              "vp8_180p",
			Val:               encode.MakeTestOptions(tulip180P, videotype.VP8Prof),
			ExtraSoftwareDeps: []string{caps.HWEncodeVP8},
			ExtraData:         encode.TestData(tulip180P),
		}, {
			Name:              "vp8_192p",
			Val:               encode.MakeTestOptions(bear192P, videotype.VP8Prof),
			ExtraSoftwareDeps: []string{caps.HWEncodeVP8},
			ExtraData:         encode.TestData(bear192P),
		}, {
			Name:              "vp8_360p",
			Val:               encode.MakeTestOptions(tulip360P, videotype.VP8Prof),
			ExtraSoftwareDeps: []string{caps.HWEncodeVP8},
			ExtraData:         encode.TestData(tulip360P),
		}, {
			Name:              "vp8_720p",
			Val:               encode.MakeTestOptions(tulip720P, videotype.VP8Prof),
			ExtraSoftwareDeps: []string{caps.HWEncodeVP8},
			ExtraData:         encode.TestData(tulip720P),
		}, {
			Name:              "vp8_1080p",
			Val:               encode.MakeTestOptions(crowd1080P, videotype.VP8Prof),
			ExtraSoftwareDeps: []string{caps.HWEncodeVP8},
			ExtraData:         encode.TestData(crowd1080P),
		}, {
			Name:              "vp8_2160p",
			Val:               encode.MakeTestOptions(crowd2160P, videotype.VP8Prof),
			ExtraSoftwareDeps: []string{caps.HWEncodeVP8_4K},
			ExtraData:         encode.TestData(crowd2160P),
		}, {
			Name:              "vp9_180p",
			Val:               encode.MakeTestOptions(tulip180P, videotype.VP9Prof),
			ExtraSoftwareDeps: []string{caps.HWEncodeVP9},
			ExtraData:         encode.TestData(tulip180P),
		}, {
			Name:              "vp9_192p",
			Val:               encode.MakeTestOptions(bear192P, videotype.VP9Prof),
			ExtraSoftwareDeps: []string{caps.HWEncodeVP9},
			ExtraData:         encode.TestData(bear192P),
		}, {
			Name:              "vp9_360p",
			Val:               encode.MakeTestOptions(tulip360P, videotype.VP9Prof),
			ExtraSoftwareDeps: []string{caps.HWEncodeVP9},
			ExtraData:         encode.TestData(tulip360P),
		}, {
			Name:              "vp9_720p",
			Val:               encode.MakeTestOptions(tulip720P, videotype.VP9Prof),
			ExtraSoftwareDeps: []string{caps.HWEncodeVP9},
			ExtraData:         encode.TestData(tulip720P),
		}, {
			Name:              "vp9_1080p",
			Val:               encode.MakeTestOptions(crowd1080P, videotype.VP9Prof),
			ExtraSoftwareDeps: []string{caps.HWEncodeVP9},
			ExtraData:         encode.TestData(crowd1080P),
		}, {
			Name:              "vp9_2160p",
			Val:               encode.MakeTestOptions(crowd2160P, videotype.VP9Prof),
			ExtraSoftwareDeps: []string{caps.HWEncodeVP9_4K},
			ExtraData:         encode.TestData(crowd2160P),
		}, {
			Name:              "vp9_720p_l1t2",
			Val:               encode.MakeTestOptionsWithSVCLayers(tulip720P, videotype.VP9Prof, "L1T2"),
			ExtraSoftwareDeps: []string{caps.HWEncodeVP9, "vaapi"},
			ExtraData:         encode.TestData(tulip720P),
		}, {
			Name:              "vp9_720p_l1t3",
			Val:               encode.MakeTestOptionsWithSVCLayers(tulip720P, videotype.VP9Prof, "L1T3"),
			ExtraSoftwareDeps: []string{caps.HWEncodeVP9, "vaapi"},
			ExtraData:         encode.TestData(tulip720P),
		}, {
			Name:              "vp9_540p_l2t3",
			Val:               encode.MakeTestOptionsWithSVCLayers(tulip540P, videotype.VP9Prof, "L2T3"),
			ExtraSoftwareDeps: []string{caps.HWEncodeVP9, "vaapi"},
			ExtraHardwareDeps: hwdep.D(hwdep.Platform("volteer", "dedede")),
			ExtraData:         encode.TestData(tulip540P),
		}, {
			Name:              "vp9_540p_l3t3",
			Val:               encode.MakeTestOptionsWithSVCLayers(tulip540P, videotype.VP9Prof, "L3T3"),
			ExtraSoftwareDeps: []string{caps.HWEncodeVP9, "vaapi"},
			ExtraHardwareDeps: hwdep.D(hwdep.Platform("volteer", "dedede")),
			ExtraData:         encode.TestData(tulip540P),
		}, {
			Name:              "vp9_720p_l2t3",
			Val:               encode.MakeTestOptionsWithSVCLayers(tulip720P, videotype.VP9Prof, "L2T3"),
			ExtraSoftwareDeps: []string{caps.HWEncodeVP9, "vaapi"},
			ExtraHardwareDeps: hwdep.D(hwdep.Platform("volteer", "dedede")),
			ExtraData:         encode.TestData(tulip720P),
		}, {
			Name:              "vp9_720p_l3t3",
			Val:               encode.MakeTestOptionsWithSVCLayers(tulip720P, videotype.VP9Prof, "L3T3"),
			ExtraSoftwareDeps: []string{caps.HWEncodeVP9, "vaapi"},
			ExtraHardwareDeps: hwdep.D(hwdep.Platform("volteer", "dedede")),
			ExtraData:         encode.TestData(tulip720P),
		}, {
			Name:              "h264_180p_nv12",
			Val:               encode.MakeNV12TestOptions(tulip180P, videotype.H264BaselineProf),
			ExtraSoftwareDeps: []string{caps.HWEncodeH264},
			ExtraData:         encode.TestData(tulip180P),
		}, {
			Name:              "vp8_180p_nv12",
			Val:               encode.MakeNV12TestOptions(tulip180P, videotype.VP8Prof),
			ExtraSoftwareDeps: []string{caps.HWEncodeVP8},
			ExtraData:         encode.TestData(tulip180P),
		}, {
			Name:              "vp9_180p_nv12",
			Val:               encode.MakeNV12TestOptions(tulip180P, videotype.VP9Prof),
			ExtraSoftwareDeps: []string{caps.HWEncodeVP9},
			ExtraData:         encode.TestData(tulip180P),
		}, {
			Name:              "h264_360p_nv12",
			Val:               encode.MakeNV12TestOptions(tulip360P, videotype.H264BaselineProf),
			ExtraSoftwareDeps: []string{caps.HWEncodeH264},
			ExtraData:         encode.TestData(tulip360P),
		}, {
			Name:              "vp8_360p_nv12",
			Val:               encode.MakeNV12TestOptions(tulip360P, videotype.VP8Prof),
			ExtraSoftwareDeps: []string{caps.HWEncodeVP8},
			ExtraData:         encode.TestData(tulip360P),
		}, {
			Name:              "vp9_360p_nv12",
			Val:               encode.MakeNV12TestOptions(tulip360P, videotype.VP9Prof),
			ExtraSoftwareDeps: []string{caps.HWEncodeVP9},
			ExtraData:         encode.TestData(tulip360P),
		}, {
			Name:              "h264_720p_nv12",
			Val:               encode.MakeNV12TestOptions(tulip720P, videotype.H264BaselineProf),
			ExtraSoftwareDeps: []string{caps.HWEncodeH264},
			ExtraData:         encode.TestData(tulip720P),
		}, {
			Name:              "vp8_720p_nv12",
			Val:               encode.MakeNV12TestOptions(tulip720P, videotype.VP8Prof),
			ExtraSoftwareDeps: []string{caps.HWEncodeVP8},
			ExtraData:         encode.TestData(tulip720P),
		}, {
			Name:              "vp9_720p_nv12",
			Val:               encode.MakeNV12TestOptions(tulip720P, videotype.VP9Prof),
			ExtraSoftwareDeps: []string{caps.HWEncodeVP9},
			ExtraData:         encode.TestData(tulip720P),
		}},
	})
}

func EncodeAccel(ctx context.Context, s *testing.State) {
	encode.RunAccelVideoTest(ctx, s, s.Param().(encode.TestOptions))
}
