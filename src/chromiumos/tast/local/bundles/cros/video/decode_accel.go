// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"

	"chromiumos/tast/local/bundles/cros/video/decode"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DecodeAccel,
		Desc:         "Verifies hardware decode acceleration by running the video_decode_accelerator_tests binary",
		Contacts:     []string{"acourbot@chromium.org", "dstaessens@chromium.org", "chromeos-video-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name:              "h264",
			Val:               decode.TestOptions{Filename: "test-25fps.h264", DecoderType: decode.VDA, ValidateFrames: false},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264},
			ExtraData:         []string{"test-25fps.h264", "test-25fps.h264.json"},
		}, {
			Name:              "vp8",
			Val:               decode.TestOptions{Filename: "test-25fps.vp8", DecoderType: decode.VDA, ValidateFrames: false},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8},
			ExtraData:         []string{"test-25fps.vp8", "test-25fps.vp8.json"},
		}, {
			Name:              "vp9",
			Val:               decode.TestOptions{Filename: "test-25fps.vp9", DecoderType: decode.VDA, ValidateFrames: false},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
			ExtraData:         []string{"test-25fps.vp9", "test-25fps.vp9.json"},
		}, {
			Name: "vp9_2",
			Val:  decode.TestOptions{Filename: "test-25fps.vp9_2", DecoderType: decode.VDA, ValidateFrames: false},
			// TODO(crbug.com/911754): reenable this test once HDR VP9.2 is implemented.
			ExtraAttr:         []string{"disabled"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9_2},
			ExtraData:         []string{"test-25fps.vp9_2", "test-25fps.vp9_2.json"},
		}, {
			Name:              "h264_resolution_switch",
			Val:               decode.TestOptions{Filename: "switch_1080p_720p_240frames.h264", DecoderType: decode.VDA, ValidateFrames: false},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264},
			ExtraData:         []string{"switch_1080p_720p_240frames.h264", "switch_1080p_720p_240frames.h264.json"},
		}, {
			Name:              "vp8_resolution_switch",
			Val:               decode.TestOptions{Filename: "resolution_change_500frames.vp8.ivf", DecoderType: decode.VDA, ValidateFrames: false},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8},
			ExtraData:         []string{"resolution_change_500frames.vp8.ivf", "resolution_change_500frames.vp8.ivf.json"},
		}, {
			Name:              "vp9_resolution_switch",
			Val:               decode.TestOptions{Filename: "resolution_change_500frames.vp9.ivf", DecoderType: decode.VDA, ValidateFrames: false},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
			ExtraData:         []string{"resolution_change_500frames.vp9.ivf", "resolution_change_500frames.vp9.ivf.json"},
		}, {
			Name: "vp8_odd_dimensions",
			Val:  decode.TestOptions{Filename: "test-25fps-321x241.vp8", DecoderType: decode.VDA, ValidateFrames: false},
			// TODO(b/138915749): Enable once decoding odd dimension videos is fixed.
			ExtraAttr:         []string{"disabled"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8},
			ExtraData:         []string{"test-25fps-321x241.vp8", "test-25fps-321x241.vp8.json"},
		}, {
			Name: "vp9_odd_dimensions",
			Val:  decode.TestOptions{Filename: "test-25fps-321x241.vp9", DecoderType: decode.VDA, ValidateFrames: false},
			// TODO(b/138915749): Enable once decoding odd dimension videos is fixed.
			ExtraAttr:         []string{"disabled"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
			ExtraData:         []string{"test-25fps-321x241.vp9", "test-25fps-321x241.vp9.json"},
		}, {
			// This test uses a video that makes use of the VP9 show-existing-frame feature and is used in Android CTS:
			// https://android.googlesource.com/platform/cts/+/master/tests/tests/media/res/raw/vp90_2_17_show_existing_frame.vp9
			Name:              "vp9_show_existing_frame",
			Val:               decode.TestOptions{Filename: "vda_sanity-vp90_2_17_show_existing_frame.vp9", DecoderType: decode.VDA, ValidateFrames: false},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
			ExtraData:         []string{"vda_sanity-vp90_2_17_show_existing_frame.vp9", "vda_sanity-vp90_2_17_show_existing_frame.vp9.json"},
		}, {
			// H264 stream in which a profile changes from Baseline to Main.
			Name:              "h264_profile_change",
			Val:               decode.TestOptions{Filename: "test-25fps_basemain.h264", DecoderType: decode.VDA, ValidateFrames: false},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264},
			ExtraData:         []string{"test-25fps_basemain.h264", "test-25fps_basemain.h264.json"},
		}, {
			// TODO(1020776): Re-enable frame validation by default when flakiness is fixed and remove validate tests.
			Name:              "h264_validate",
			Val:               decode.TestOptions{Filename: "test-25fps.h264", DecoderType: decode.VDA, ValidateFrames: true},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264},
			ExtraData:         []string{"test-25fps.h264", "test-25fps.h264.json"},
		}, {
			Name:              "vp8_validate",
			Val:               decode.TestOptions{Filename: "test-25fps.vp8", DecoderType: decode.VDA, ValidateFrames: true},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8},
			ExtraData:         []string{"test-25fps.vp8", "test-25fps.vp8.json"},
		}, {
			Name:              "vp9_validate",
			Val:               decode.TestOptions{Filename: "test-25fps.vp9", DecoderType: decode.VDA, ValidateFrames: true},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
			ExtraData:         []string{"test-25fps.vp9", "test-25fps.vp9.json"},
		}},
	})
}

func DecodeAccel(ctx context.Context, s *testing.State) {
	decode.RunAccelVideoTest(ctx, s, s.Param().(decode.TestOptions))
}
