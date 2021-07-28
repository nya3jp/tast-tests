// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"

	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/local/bundles/cros/video/play"
	"chromiumos/tast/local/lacros"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type contentsParams struct {
	fileName    string
	refFileName string
	chromeType  lacros.ChromeType
}

func init() {
	testing.AddTest(&testing.Test{
		Func: Contents,
		Desc: "Verifies that a screenshot of a full screen is valid",
		Contacts: []string{
			"andrescj@chromium.org",
			"chromeos-gfx-video@google.com",
		},
		// TODO(b/162437142): reenable on Zork when it does not hang forever.
		HardwareDeps: hwdep.D(hwdep.InternalDisplay(), hwdep.SkipOnPlatform("zork")),
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name: "h264_360p_hw",
			Val: contentsParams{
				fileName:    "still-colors-360p.h264.mp4",
				refFileName: "still-colors-360p.ref.png",
				chromeType:  lacros.ChromeTypeChromeOS,
			},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         []string{"video.html", "still-colors-360p.h264.mp4", "still-colors-360p.ref.png"},
			ExtraHardwareDeps: hwdep.D(hwdep.SupportsNV12Overlays()),
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "proprietary_codecs"},
			Fixture:           "chromeVideo",
		}, {
			// TODO(andrescj): move to graphics_nightly after the test is stabilized.
			Name: "h264_360p_exotic_crop_hw",
			Val: contentsParams{
				fileName:    "still-colors-720x480-cropped-to-640x360.h264.mp4",
				refFileName: "still-colors-360p.ref.png",
				chromeType:  lacros.ChromeTypeChromeOS,
			},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         []string{"video.html", "still-colors-720x480-cropped-to-640x360.h264.mp4", "still-colors-360p.ref.png"},
			ExtraHardwareDeps: hwdep.D(hwdep.SupportsNV12Overlays()),
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "proprietary_codecs"},
			Fixture:           "chromeVideo",
		}, {
			// TODO(andrescj): move to graphics_nightly after the test is stabilized.
			Name: "h264_360p_exotic_crop_hw_lacros",
			Val: contentsParams{
				fileName:    "still-colors-720x480-cropped-to-640x360.h264.mp4",
				refFileName: "still-colors-360p.ref.png",
				chromeType:  lacros.ChromeTypeLacros,
			},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         []string{"video.html", "still-colors-720x480-cropped-to-640x360.h264.mp4", "still-colors-360p.ref.png"},
			ExtraHardwareDeps: hwdep.D(hwdep.SupportsNV12Overlays()),
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "proprietary_codecs", "lacros"},
			Fixture:           "chromeVideoLacros",
		}, {
			Name: "h264_480p_hw",
			Val: contentsParams{
				fileName:    "still-colors-480p.h264.mp4",
				refFileName: "still-colors-480p.ref.png",
				chromeType:  lacros.ChromeTypeChromeOS,
			},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         []string{"video.html", "still-colors-480p.h264.mp4", "still-colors-480p.ref.png"},
			ExtraHardwareDeps: hwdep.D(hwdep.SupportsNV12Overlays()),
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "proprietary_codecs"},
			Fixture:           "chromeVideo",
		}, {
			Name: "h264_720p_hw",
			Val: contentsParams{
				fileName:    "still-colors-720p.h264.mp4",
				refFileName: "still-colors-720p.ref.png",
				chromeType:  lacros.ChromeTypeChromeOS,
			},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         []string{"video.html", "still-colors-720p.h264.mp4", "still-colors-720p.ref.png"},
			ExtraHardwareDeps: hwdep.D(hwdep.SupportsNV12Overlays()),
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "proprietary_codecs"},
			Fixture:           "chromeVideo",
		}, {
			Name: "h264_1080p_hw",
			Val: contentsParams{
				fileName:    "still-colors-1080p.h264.mp4",
				refFileName: "still-colors-1080p.ref.png",
				chromeType:  lacros.ChromeTypeChromeOS,
			},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         []string{"video.html", "still-colors-1080p.h264.mp4", "still-colors-1080p.ref.png"},
			ExtraHardwareDeps: hwdep.D(hwdep.SupportsNV12Overlays()),
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "proprietary_codecs"},
			Fixture:           "chromeVideo",
		}, {
			Name: "h264_360p_composited_hw",
			Val: contentsParams{
				fileName:    "still-colors-360p.h264.mp4",
				refFileName: "still-colors-360p.ref.png",
				chromeType:  lacros.ChromeTypeChromeOS,
			},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         []string{"video.html", "still-colors-360p.h264.mp4", "still-colors-360p.ref.png"},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "proprietary_codecs"},
			Fixture:           "chromeCompositedVideo",
		}, {
			// TODO(andrescj): move to graphics_nightly after the test is stabilized.
			Name: "h264_360p_exotic_crop_composited_hw",
			Val: contentsParams{
				fileName:    "still-colors-720x480-cropped-to-640x360.h264.mp4",
				refFileName: "still-colors-360p.ref.png",
				chromeType:  lacros.ChromeTypeChromeOS,
			},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         []string{"video.html", "still-colors-720x480-cropped-to-640x360.h264.mp4", "still-colors-360p.ref.png"},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "proprietary_codecs"},
			Fixture:           "chromeCompositedVideo",
		}, {
			// TODO(andrescj): move to graphics_nightly after the test is stabilized.
			Name: "h264_360p_exotic_crop_ash_composited_hw_lacros",
			Val: contentsParams{
				fileName:    "still-colors-720x480-cropped-to-640x360.h264.mp4",
				refFileName: "still-colors-360p.ref.png",
				chromeType:  lacros.ChromeTypeLacros,
			},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         []string{"video.html", "still-colors-720x480-cropped-to-640x360.h264.mp4", "still-colors-360p.ref.png"},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "proprietary_codecs", "lacros"},
			Fixture:           "chromeAshCompositedVideoLacros",
		}, {
			// TODO(andrescj): move to graphics_nightly after the test is stabilized.
			Name: "h264_360p_exotic_crop_lacros_composited_hw_lacros",
			Val: contentsParams{
				fileName:    "still-colors-720x480-cropped-to-640x360.h264.mp4",
				refFileName: "still-colors-360p.ref.png",
				chromeType:  lacros.ChromeTypeLacros,
			},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         []string{"video.html", "still-colors-720x480-cropped-to-640x360.h264.mp4", "still-colors-360p.ref.png"},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "proprietary_codecs", "lacros"},
			Fixture:           "chromeLacrosCompositedVideoLacros",
		}, {
			Name: "h264_480p_composited_hw",
			Val: contentsParams{
				fileName:    "still-colors-480p.h264.mp4",
				refFileName: "still-colors-480p.ref.png",
				chromeType:  lacros.ChromeTypeChromeOS,
			},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         []string{"video.html", "still-colors-480p.h264.mp4", "still-colors-480p.ref.png"},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "proprietary_codecs"},
			Fixture:           "chromeCompositedVideo",
		}, {
			Name: "h264_720p_composited_hw",
			Val: contentsParams{
				fileName:    "still-colors-720p.h264.mp4",
				refFileName: "still-colors-720p.ref.png",
				chromeType:  lacros.ChromeTypeChromeOS,
			},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         []string{"video.html", "still-colors-720p.h264.mp4", "still-colors-720p.ref.png"},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "proprietary_codecs"},
			Fixture:           "chromeCompositedVideo",
		}, {
			Name: "h264_1080p_composited_hw",
			Val: contentsParams{
				fileName:    "still-colors-1080p.h264.mp4",
				refFileName: "still-colors-1080p.ref.png",
				chromeType:  lacros.ChromeTypeChromeOS,
			},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         []string{"video.html", "still-colors-1080p.h264.mp4", "still-colors-1080p.ref.png"},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "proprietary_codecs"},
			Fixture:           "chromeCompositedVideo",
		}},
		// TODO(andrescj): add tests for VP8 and VP9.
		// TODO(andrescj): for non-composited tests, check that overlays were used.
	})
}

// Contents starts playing a video, takes a screenshot, and checks a few interesting pixels.
func Contents(ctx context.Context, s *testing.State) {
	testOpt := s.Param().(contentsParams)

	cr, l, cs, err := lacros.Setup(ctx, s.FixtValue(), testOpt.chromeType)
	if err != nil {
		s.Fatal("Failed to initialize test: ", err)
	}
	defer lacros.CloseLacrosChrome(ctx, l)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}
	defer tconn.Close()

	if err := play.TestPlayAndScreenshot(ctx, s, tconn, cs, testOpt.fileName, testOpt.refFileName); err != nil {
		s.Fatal("TestPlayAndScreenshot failed: ", err)
	}
}
