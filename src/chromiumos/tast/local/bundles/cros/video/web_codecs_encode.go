// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"

	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/local/bundles/cros/video/webcodecs"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/media/videotype"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WebCodecsEncode,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Verifies that WebCodecs encoding API works, maybe verifying use of a hardware accelerator",
		Contacts: []string{
			"hiroh@chromium.org", // Test author.
			"chromeos-gfx-video@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Data:         append(append(webcodecs.MP4DemuxerDataFiles(), webcodecs.EncodeDataFiles()...), webcodecs.VideoDataFiles()...),
		Attr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
		Fixture:      "chromeWebCodecs",
		Params: []testing.Param{{
			Name:              "h264_sw",
			Val:               webcodecs.TestEncodeArgs{Codec: videotype.H264, Acceleration: webcodecs.PreferSoftware},
			ExtraSoftwareDeps: []string{"proprietary_codecs"},
		}, {
			Name:              "h264_hw",
			Val:               webcodecs.TestEncodeArgs{Codec: videotype.H264, Acceleration: webcodecs.PreferHardware},
			ExtraSoftwareDeps: []string{"proprietary_codecs", caps.HWEncodeH264},
		}, {
			Name:              "h264_sw_l1t2",
			Val:               webcodecs.TestEncodeArgs{Codec: videotype.H264, Acceleration: webcodecs.PreferSoftware, ScalabilityMode: "L1T2"},
			ExtraSoftwareDeps: []string{"proprietary_codecs"},
		}, {
			Name:              "h264_hw_l1t2",
			Val:               webcodecs.TestEncodeArgs{Codec: videotype.H264, Acceleration: webcodecs.PreferHardware, ScalabilityMode: "L1T2"},
			ExtraSoftwareDeps: []string{"proprietary_codecs", caps.HWEncodeH264, "vaapi"},
			// TODO(b/199487660): Run on AMD platforms once their driver supports H.264 temporal layer encoding.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnPlatform("grunt", "zork")),
		}, {
			Name:              "h264_sw_l1t3",
			Val:               webcodecs.TestEncodeArgs{Codec: videotype.H264, Acceleration: webcodecs.PreferSoftware, ScalabilityMode: "L1T3"},
			ExtraSoftwareDeps: []string{"proprietary_codecs"},
		}, {
			Name:              "h264_hw_l1t3",
			Val:               webcodecs.TestEncodeArgs{Codec: videotype.H264, Acceleration: webcodecs.PreferHardware, ScalabilityMode: "L1T3"},
			ExtraSoftwareDeps: []string{"proprietary_codecs", caps.HWEncodeH264, "vaapi"},
			// TODO(b/199487660): Run on AMD platforms once their driver supports H.264 temporal layer encoding.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnPlatform("grunt", "zork")),
		}, {
			Name: "vp8_sw",
			Val:  webcodecs.TestEncodeArgs{Codec: videotype.VP8, Acceleration: webcodecs.PreferSoftware},
		}, {
			Name:              "vp8_hw",
			Val:               webcodecs.TestEncodeArgs{Codec: videotype.VP8, Acceleration: webcodecs.PreferHardware},
			ExtraSoftwareDeps: []string{caps.HWEncodeVP8},
		}, {
			Name: "vp8_sw_l1t3",
			Val:  webcodecs.TestEncodeArgs{Codec: videotype.VP8, Acceleration: webcodecs.PreferSoftware, ScalabilityMode: "L1T3"},
		}, {
			Name:              "vp8_hw_l1t3",
			Val:               webcodecs.TestEncodeArgs{Codec: videotype.VP8, Acceleration: webcodecs.PreferHardware, ScalabilityMode: "L1T3"},
			ExtraSoftwareDeps: []string{caps.HWEncodeVP8, "vaapi"},
		}, {
			Name: "vp9_sw",
			Val:  webcodecs.TestEncodeArgs{Codec: videotype.VP9, Acceleration: webcodecs.PreferSoftware},
		}, {
			Name:              "vp9_hw",
			Val:               webcodecs.TestEncodeArgs{Codec: videotype.VP9, Acceleration: webcodecs.PreferHardware},
			ExtraSoftwareDeps: []string{caps.HWEncodeVP9},
		}, {
			Name: "vp9_sw_l1t2",
			Val:  webcodecs.TestEncodeArgs{Codec: videotype.VP9, Acceleration: webcodecs.PreferSoftware, ScalabilityMode: "L1T2"},
		}, {
			Name:              "vp9_hw_l1t2",
			Val:               webcodecs.TestEncodeArgs{Codec: videotype.VP9, Acceleration: webcodecs.PreferHardware, ScalabilityMode: "L1T2"},
			ExtraSoftwareDeps: []string{caps.HWEncodeVP9, "vaapi"},
		}, {
			Name: "vp9_sw_l1t3",
			Val:  webcodecs.TestEncodeArgs{Codec: videotype.VP9, Acceleration: webcodecs.PreferSoftware, ScalabilityMode: "L1T3"},
		}, {
			Name:              "vp9_hw_l1t3",
			Val:               webcodecs.TestEncodeArgs{Codec: videotype.VP9, Acceleration: webcodecs.PreferHardware, ScalabilityMode: "L1T3"},
			ExtraSoftwareDeps: []string{caps.HWEncodeVP9, "vaapi"},
		}},
	})
}

func WebCodecsEncode(ctx context.Context, s *testing.State) {
	args := s.Param().(webcodecs.TestEncodeArgs)

	if err := webcodecs.RunEncodeTest(ctx, s.FixtValue().(*chrome.Chrome),
		s.DataFileSystem(), args, s.DataPath(webcodecs.Crowd720p), s.OutDir()); err != nil {
		s.Error("Test failed: ", err)
	}
}
