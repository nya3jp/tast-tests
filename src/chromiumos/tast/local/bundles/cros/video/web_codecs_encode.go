// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"

	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/local/bundles/cros/video/webcodecs"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/lacros"
	"chromiumos/tast/local/media/videotype"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WebCodecsEncode,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Verifies that WebCodecs encoding API works, maybe verifying use of a hardware accelerator",
		Contacts: []string{
			"hiroh@chromium.org", // Test author.
			"chromeos-gfx-video@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Data:         append(append(webcodecs.MP4DemuxerDataFiles(), webcodecs.EncodeDataFiles()...), webcodecs.VideoDataFiles()...),
		Attr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
		Params: []testing.Param{{
			Name:              "h264_sw",
			Val:               webcodecs.TestEncodeArgs{Codec: videotype.H264, Acceleration: webcodecs.PreferSoftware, BitrateMode: "constant", BrowserType: browser.TypeAsh},
			ExtraSoftwareDeps: []string{"proprietary_codecs"},
			Fixture:           "chromeWebCodecs",
		}, {
			Name:              "h264_hw",
			Val:               webcodecs.TestEncodeArgs{Codec: videotype.H264, Acceleration: webcodecs.PreferHardware, BitrateMode: "constant", BrowserType: browser.TypeAsh},
			ExtraSoftwareDeps: []string{"proprietary_codecs", caps.HWEncodeH264},
			Fixture:           "chromeWebCodecs",
		}, {
			Name:              "h264_hw_lacros",
			Val:               webcodecs.TestEncodeArgs{Codec: videotype.H264, Acceleration: webcodecs.PreferHardware, BitrateMode: "constant", BrowserType: browser.TypeLacros},
			ExtraSoftwareDeps: []string{"proprietary_codecs", caps.HWEncodeH264, "lacros"},
			Fixture:           "chromeWebCodecsLacros",
		}, {
			Name:              "h264_sw_l1t2",
			Val:               webcodecs.TestEncodeArgs{Codec: videotype.H264, Acceleration: webcodecs.PreferSoftware, ScalabilityMode: "L1T2", BitrateMode: "constant", BrowserType: browser.TypeAsh},
			ExtraSoftwareDeps: []string{"proprietary_codecs"},
			Fixture:           "chromeWebCodecs",
		}, {
			Name:              "h264_hw_l1t2",
			Val:               webcodecs.TestEncodeArgs{Codec: videotype.H264, Acceleration: webcodecs.PreferHardware, ScalabilityMode: "L1T2", BitrateMode: "constant", BrowserType: browser.TypeAsh},
			ExtraSoftwareDeps: []string{"proprietary_codecs", caps.HWEncodeH264, "vaapi"},
			Fixture:           "chromeWebCodecs",
		}, {
			Name:              "h264_sw_l1t3",
			Val:               webcodecs.TestEncodeArgs{Codec: videotype.H264, Acceleration: webcodecs.PreferSoftware, ScalabilityMode: "L1T3", BitrateMode: "constant", BrowserType: browser.TypeAsh},
			ExtraSoftwareDeps: []string{"proprietary_codecs"},
			Fixture:           "chromeWebCodecs",
		}, {
			Name:              "h264_hw_l1t3",
			Val:               webcodecs.TestEncodeArgs{Codec: videotype.H264, Acceleration: webcodecs.PreferHardware, ScalabilityMode: "L1T3", BitrateMode: "constant", BrowserType: browser.TypeAsh},
			ExtraSoftwareDeps: []string{"proprietary_codecs", caps.HWEncodeH264, "vaapi"},
			Fixture:           "chromeWebCodecs",
		}, {
			Name:              "h264_sw_vbr",
			Val:               webcodecs.TestEncodeArgs{Codec: videotype.H264, Acceleration: webcodecs.PreferSoftware, BitrateMode: "variable", BrowserType: browser.TypeAsh},
			ExtraSoftwareDeps: []string{"proprietary_codecs"},
			Fixture:           "chromeWebCodecs",
		}, {
			Name:              "h264_hw_vbr",
			Val:               webcodecs.TestEncodeArgs{Codec: videotype.H264, Acceleration: webcodecs.PreferHardware, BitrateMode: "variable", BrowserType: browser.TypeAsh},
			ExtraSoftwareDeps: []string{"proprietary_codecs", caps.HWEncodeH264VBR},
			Fixture:           "chromeWebCodecsWithHWVBREncoding",
		}, {
			Name:    "vp8_sw",
			Val:     webcodecs.TestEncodeArgs{Codec: videotype.VP8, Acceleration: webcodecs.PreferSoftware, BitrateMode: "constant", BrowserType: browser.TypeAsh},
			Fixture: "chromeWebCodecs",
		}, {
			Name:              "vp8_hw",
			Val:               webcodecs.TestEncodeArgs{Codec: videotype.VP8, Acceleration: webcodecs.PreferHardware, BitrateMode: "constant", BrowserType: browser.TypeAsh},
			ExtraSoftwareDeps: []string{caps.HWEncodeVP8},
			Fixture:           "chromeWebCodecs",
		}, {
			Name:              "vp8_hw_lacros",
			Val:               webcodecs.TestEncodeArgs{Codec: videotype.VP8, Acceleration: webcodecs.PreferHardware, BitrateMode: "constant", BrowserType: browser.TypeLacros},
			ExtraSoftwareDeps: []string{caps.HWEncodeVP8, "lacros"},
			Fixture:           "chromeWebCodecsLacros",
		}, {
			Name:    "vp8_sw_l1t3",
			Val:     webcodecs.TestEncodeArgs{Codec: videotype.VP8, Acceleration: webcodecs.PreferSoftware, ScalabilityMode: "L1T3", BitrateMode: "constant", BrowserType: browser.TypeAsh},
			Fixture: "chromeWebCodecs",
		}, {
			Name:              "vp8_hw_l1t3",
			Val:               webcodecs.TestEncodeArgs{Codec: videotype.VP8, Acceleration: webcodecs.PreferHardware, ScalabilityMode: "L1T3", BitrateMode: "constant", BrowserType: browser.TypeAsh},
			ExtraSoftwareDeps: []string{caps.HWEncodeVP8, "vaapi"},
			Fixture:           "chromeWebCodecsWithHWVp8TemporalLayerEncoding",
		}, {
			Name:    "vp8_sw_vbr",
			Val:     webcodecs.TestEncodeArgs{Codec: videotype.VP8, Acceleration: webcodecs.PreferSoftware, BitrateMode: "variable", BrowserType: browser.TypeAsh},
			Fixture: "chromeWebCodecs",
		}, {
			Name:    "vp9_sw",
			Val:     webcodecs.TestEncodeArgs{Codec: videotype.VP9, Acceleration: webcodecs.PreferSoftware, BitrateMode: "constant", BrowserType: browser.TypeAsh},
			Fixture: "chromeWebCodecs",
		}, {
			Name:              "vp9_hw",
			Val:               webcodecs.TestEncodeArgs{Codec: videotype.VP9, Acceleration: webcodecs.PreferHardware, BitrateMode: "constant", BrowserType: browser.TypeAsh},
			ExtraSoftwareDeps: []string{caps.HWEncodeVP9},
			Fixture:           "chromeWebCodecs",
		}, {
			Name:              "vp9_hw_lacros",
			Val:               webcodecs.TestEncodeArgs{Codec: videotype.VP9, Acceleration: webcodecs.PreferHardware, BitrateMode: "constant", BrowserType: browser.TypeLacros},
			ExtraSoftwareDeps: []string{caps.HWEncodeVP9, "lacros"},
			Fixture:           "chromeWebCodecsLacros",
		}, {
			Name:    "vp9_sw_l1t2",
			Val:     webcodecs.TestEncodeArgs{Codec: videotype.VP9, Acceleration: webcodecs.PreferSoftware, ScalabilityMode: "L1T2", BitrateMode: "constant", BrowserType: browser.TypeAsh},
			Fixture: "chromeWebCodecs",
		}, {
			Name:              "vp9_hw_l1t2",
			Val:               webcodecs.TestEncodeArgs{Codec: videotype.VP9, Acceleration: webcodecs.PreferHardware, ScalabilityMode: "L1T2", BitrateMode: "constant", BrowserType: browser.TypeAsh},
			ExtraSoftwareDeps: []string{caps.HWEncodeVP9, "vaapi"},
			Fixture:           "chromeWebCodecs",
		}, {
			Name:    "vp9_sw_l1t3",
			Val:     webcodecs.TestEncodeArgs{Codec: videotype.VP9, Acceleration: webcodecs.PreferSoftware, ScalabilityMode: "L1T3", BitrateMode: "constant", BrowserType: browser.TypeAsh},
			Fixture: "chromeWebCodecs",
		}, {
			Name:              "vp9_hw_l1t3",
			Val:               webcodecs.TestEncodeArgs{Codec: videotype.VP9, Acceleration: webcodecs.PreferHardware, ScalabilityMode: "L1T3", BitrateMode: "constant", BrowserType: browser.TypeAsh},
			ExtraSoftwareDeps: []string{caps.HWEncodeVP9, "vaapi"},
			Fixture:           "chromeWebCodecs",
		}, {
			Name:    "vp9_sw_vbr",
			Val:     webcodecs.TestEncodeArgs{Codec: videotype.VP9, Acceleration: webcodecs.PreferSoftware, BitrateMode: "variable", BrowserType: browser.TypeAsh},
			Fixture: "chromeWebCodecs",
		}, {
			Name:    "av1_sw",
			Val:     webcodecs.TestEncodeArgs{Codec: videotype.AV1, Acceleration: webcodecs.PreferSoftware, BitrateMode: "constant", BrowserType: browser.TypeAsh},
			Fixture: "chromeWebCodecs",
		}, {
			Name:    "av1_sw_l1t2",
			Val:     webcodecs.TestEncodeArgs{Codec: videotype.AV1, Acceleration: webcodecs.PreferSoftware, ScalabilityMode: "L1T2", BitrateMode: "constant", BrowserType: browser.TypeAsh},
			Fixture: "chromeWebCodecs",
		}, {
			Name:    "av1_sw_l1t3",
			Val:     webcodecs.TestEncodeArgs{Codec: videotype.AV1, Acceleration: webcodecs.PreferSoftware, ScalabilityMode: "L1T3", BitrateMode: "constant", BrowserType: browser.TypeAsh},
			Fixture: "chromeWebCodecs",
		}, {
			Name:    "av1_sw_vbr",
			Val:     webcodecs.TestEncodeArgs{Codec: videotype.AV1, Acceleration: webcodecs.PreferSoftware, BitrateMode: "variable", BrowserType: browser.TypeAsh},
			Fixture: "chromeWebCodecs",
		}},
	})
}

func WebCodecsEncode(ctx context.Context, s *testing.State) {
	args := s.Param().(webcodecs.TestEncodeArgs)

	_, l, cs, err := lacros.Setup(ctx, s.FixtValue(), args.BrowserType)
	if err != nil {
		s.Fatal("Failed to initialize test: ", err)
	}
	defer lacros.CloseLacros(ctx, l)

	if err := webcodecs.RunEncodeTest(ctx, cs,
		s.DataFileSystem(), args, s.DataPath(webcodecs.Crowd720p), s.OutDir()); err != nil {
		s.Error("Test failed: ", err)
	}
}
