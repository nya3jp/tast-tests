// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"

	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/local/bundles/cros/video/webcodecs"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WebCodecsDecode,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Verifies that WebCodecs API works, maybe verifying use of a hardware accelerator",
		Contacts: []string{
			"hiroh@chromium.org", // Test author.
			"chromeos-gfx-video@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Data:         append(webcodecs.DecodeDataFiles(), webcodecs.MP4DemuxerDataFiles()...),
		Attr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
		Fixture:      "chromeWebCodecs",
		Params: []testing.Param{{
			Name:      "av1_sw",
			Val:       webcodecs.TestDecodeArgs{VideoFile: "bear-320x240.av1.mp4", Acceleration: webcodecs.PreferSoftware},
			ExtraData: []string{"bear-320x240.av1.mp4", "bear-320x240.av1.mp4.json"},
		}, {
			Name:              "av1_hw",
			Val:               webcodecs.TestDecodeArgs{VideoFile: "bear-320x240.av1.mp4", Acceleration: webcodecs.PreferHardware},
			ExtraSoftwareDeps: []string{caps.HWDecodeAV1},
			ExtraData:         []string{"bear-320x240.av1.mp4", "bear-320x240.av1.mp4.json"},
		}, {
			Name:              "h264_sw",
			Val:               webcodecs.TestDecodeArgs{VideoFile: "bear-320x240.h264.mp4", Acceleration: webcodecs.PreferSoftware},
			ExtraSoftwareDeps: []string{"proprietary_codecs"},
			ExtraData:         []string{"bear-320x240.h264.mp4", "bear-320x240.h264.mp4.json"},
		}, {
			Name:              "h264_hw",
			Val:               webcodecs.TestDecodeArgs{VideoFile: "bear-320x240.h264.mp4", Acceleration: webcodecs.PreferHardware},
			ExtraSoftwareDeps: []string{"proprietary_codecs", caps.HWDecodeH264},
			ExtraData:         []string{"bear-320x240.h264.mp4", "bear-320x240.h264.mp4.json"},
		}, {
			Name:      "vp8_sw",
			Val:       webcodecs.TestDecodeArgs{VideoFile: "bear-320x240.vp8.mp4", Acceleration: webcodecs.PreferSoftware},
			ExtraData: []string{"bear-320x240.vp8.mp4", "bear-320x240.vp8.mp4.json"},
		}, {
			Name:              "vp8_hw",
			Val:               webcodecs.TestDecodeArgs{VideoFile: "bear-320x240.vp8.mp4", Acceleration: webcodecs.PreferHardware},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8},
			ExtraData:         []string{"bear-320x240.vp8.mp4", "bear-320x240.vp8.mp4.json"},
		}, {
			Name:      "vp9_sw",
			Val:       webcodecs.TestDecodeArgs{VideoFile: "bear-320x240.vp9.mp4", Acceleration: webcodecs.PreferSoftware},
			ExtraData: []string{"bear-320x240.vp9.mp4", "bear-320x240.vp9.mp4.json"},
		}, {
			Name:              "vp9_hw",
			Val:               webcodecs.TestDecodeArgs{VideoFile: "bear-320x240.vp9.mp4", Acceleration: webcodecs.PreferHardware},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
			ExtraData:         []string{"bear-320x240.vp9.mp4", "bear-320x240.vp9.mp4.json"},
		}},
	})
}

func WebCodecsDecode(ctx context.Context, s *testing.State) {
	args := s.Param().(webcodecs.TestDecodeArgs)
	if err := webcodecs.RunDecodeTest(ctx, s.FixtValue().(*chrome.Chrome),
		s.DataFileSystem(), args, s.DataPath(args.VideoFile+".json"), s.OutDir()); err != nil {
		s.Error("Test failed: ", err)
	}
}
