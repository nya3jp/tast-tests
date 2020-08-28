// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"

	"chromiumos/tast/local/bundles/cros/video/play"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/local/media/pre"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type contentsParams struct {
	fileName    string
	refFileName string
}

func init() {
	testing.AddTest(&testing.Test{
		Func: Contents,
		Desc: "Verifies that a screenshot of a full screen is sane",
		Contacts: []string{
			"andrescj@chromium.org",
			"chromeos-gfx-video@google.com",
		},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name: "h264_360p_hw",
			Val: contentsParams{
				fileName:    "still-colors-360p.h264.mp4",
				refFileName: "still-colors-360p.ref.png",
			},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         []string{"video.html", "still-colors-360p.h264.mp4", "still-colors-360p.ref.png"},
			ExtraHardwareDeps: hwdep.D(hwdep.SupportsNV12Overlays()),
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "chrome_internal"}, // "chrome_internal" is needed because H.264 is a proprietary codec.
			Pre:               pre.ChromeVideo(),
		}, {
			// TODO(andrescj): move to graphics_nightly after the test is stabilized.
			Name: "h264_360p_exotic_crop_hw",
			Val: contentsParams{
				fileName:    "still-colors-720x480-cropped-to-640x360.h264.mp4",
				refFileName: "still-colors-360p.ref.png",
			},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         []string{"video.html", "still-colors-720x480-cropped-to-640x360.h264.mp4", "still-colors-360p.ref.png"},
			ExtraHardwareDeps: hwdep.D(hwdep.SupportsNV12Overlays()),
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "chrome_internal"}, // "chrome_internal" is needed because H.264 is a proprietary codec.
			Pre:               pre.ChromeVideo(),
		}, {
			Name: "h264_480p_hw",
			Val: contentsParams{
				fileName:    "still-colors-480p.h264.mp4",
				refFileName: "still-colors-480p.ref.png",
			},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         []string{"video.html", "still-colors-480p.h264.mp4", "still-colors-480p.ref.png"},
			ExtraHardwareDeps: hwdep.D(hwdep.SupportsNV12Overlays()),
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "chrome_internal"}, // "chrome_internal" is needed because H.264 is a proprietary codec.
			Pre:               pre.ChromeVideo(),
		}, {
			Name: "h264_720p_hw",
			Val: contentsParams{
				fileName:    "still-colors-720p.h264.mp4",
				refFileName: "still-colors-720p.ref.png",
			},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         []string{"video.html", "still-colors-720p.h264.mp4", "still-colors-720p.ref.png"},
			ExtraHardwareDeps: hwdep.D(hwdep.SupportsNV12Overlays()),
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "chrome_internal"}, // "chrome_internal" is needed because H.264 is a proprietary codec.
			Pre:               pre.ChromeVideo(),
		}, {
			Name: "h264_1080p_hw",
			Val: contentsParams{
				fileName:    "still-colors-1080p.h264.mp4",
				refFileName: "still-colors-1080p.ref.png",
			},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         []string{"video.html", "still-colors-1080p.h264.mp4", "still-colors-1080p.ref.png"},
			ExtraHardwareDeps: hwdep.D(hwdep.SupportsNV12Overlays()),
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "chrome_internal"}, // "chrome_internal" is needed because H.264 is a proprietary codec.
			Pre:               pre.ChromeVideo(),
		}, {
			Name: "h264_360p_composited_hw",
			Val: contentsParams{
				fileName:    "still-colors-360p.h264.mp4",
				refFileName: "still-colors-360p.ref.png",
			},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         []string{"video.html", "still-colors-360p.h264.mp4", "still-colors-360p.ref.png"},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "chrome_internal"}, // "chrome_internal" is needed because H.264 is a proprietary codec.
			Pre:               pre.ChromeCompositedVideo(),
		}, {
			// TODO(andrescj): move to graphics_nightly after the test is stabilized.
			Name: "h264_360p_exotic_crop_composited_hw",
			Val: contentsParams{
				fileName:    "still-colors-720x480-cropped-to-640x360.h264.mp4",
				refFileName: "still-colors-360p.ref.png",
			},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         []string{"video.html", "still-colors-720x480-cropped-to-640x360.h264.mp4", "still-colors-360p.ref.png"},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "chrome_internal"}, // "chrome_internal" is needed because H.264 is a proprietary codec.
			Pre:               pre.ChromeCompositedVideo(),
		}, {
			Name: "h264_480p_composited_hw",
			Val: contentsParams{
				fileName:    "still-colors-480p.h264.mp4",
				refFileName: "still-colors-480p.ref.png",
			},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         []string{"video.html", "still-colors-480p.h264.mp4", "still-colors-480p.ref.png"},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "chrome_internal"}, // "chrome_internal" is needed because H.264 is a proprietary codec.
			Pre:               pre.ChromeCompositedVideo(),
		}, {
			Name: "h264_720p_composited_hw",
			Val: contentsParams{
				fileName:    "still-colors-720p.h264.mp4",
				refFileName: "still-colors-720p.ref.png",
			},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         []string{"video.html", "still-colors-720p.h264.mp4", "still-colors-720p.ref.png"},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "chrome_internal"}, // "chrome_internal" is needed because H.264 is a proprietary codec.
			Pre:               pre.ChromeCompositedVideo(),
		}, {
			Name: "h264_1080p_composited_hw",
			Val: contentsParams{
				fileName:    "still-colors-1080p.h264.mp4",
				refFileName: "still-colors-1080p.ref.png",
			},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         []string{"video.html", "still-colors-1080p.h264.mp4", "still-colors-1080p.ref.png"},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "chrome_internal"}, // "chrome_internal" is needed because H.264 is a proprietary codec.
			Pre:               pre.ChromeCompositedVideo(),
		}},
		// TODO(andrescj): add tests for VP8 and VP9.
		// TODO(andrescj): for non-composited tests, check that overlays were used.
	})
}

// Contents starts playing a video, takes a screenshot, and checks a few interesting pixels.
func Contents(ctx context.Context, s *testing.State) {
	testOpt := s.Param().(contentsParams)
	if err := play.TestPlayAndScreenshot(ctx, s, s.PreValue().(*chrome.Chrome), testOpt.fileName, testOpt.refFileName); err != nil {
		s.Fatal("TestPlayAndScreenshot failed: ", err)
	}
}
