// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"
	"time"

	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/local/bundles/cros/video/play"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/graphics"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type memCheckParams struct {
	fileName  string
	sizes     []graphics.Size
	videoType play.VideoType
}

func init() {
	testing.AddTest(&testing.Test{
		Func: MemCheck,
		Desc: "Checks video playback in Chrome has no leaks",
		Contacts: []string{
			"mcasas@chromium.org",
			"chromeos-gfx-video@google.com",
		},
		HardwareDeps: hwdep.D(hwdep.SupportsNV12Overlays()),
		SoftwareDeps: []string{"chrome", "graphics_debugfs"},
		Params: []testing.Param{{
			Name:              "av1_hw",
			Val:               memCheckParams{fileName: "720_av1.mp4", sizes: []graphics.Size{{Width: 1280, Height: 720}}, videoType: play.NormalVideo},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
			ExtraData:         []string{"video.html", "720_av1.mp4"},
			ExtraHardwareDeps: hwdep.D(hwdep.SupportsNV12Overlays()),
			ExtraSoftwareDeps: []string{"video_overlays", caps.HWDecodeAV1},
			Fixture:           "chromeVideoWithGuestLoginAndHWAV1Decoding",
			Timeout:           10 * time.Minute,
		}, {
			Name:              "h264_hw",
			Val:               memCheckParams{fileName: "720_h264.mp4", sizes: []graphics.Size{{Width: 1280, Height: 720}}, videoType: play.NormalVideo},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
			ExtraData:         []string{"video.html", "720_h264.mp4"},
			ExtraHardwareDeps: hwdep.D(hwdep.SupportsNV12Overlays()),
			ExtraSoftwareDeps: []string{"video_overlays", caps.HWDecodeH264, "proprietary_codecs"},
			Fixture:           "chromeVideoWithGuestLogin",
			Timeout:           10 * time.Minute,
		}, {
			Name:              "hevc_hw",
			Val:               memCheckParams{fileName: "720_hevc.mp4", sizes: []graphics.Size{{Width: 1280, Height: 720}}, videoType: play.NormalVideo},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
			ExtraData:         []string{"video.html", "720_hevc.mp4"},
			ExtraHardwareDeps: hwdep.D(hwdep.SupportsNV12Overlays()),
			ExtraSoftwareDeps: []string{"video_overlays", caps.HWDecodeHEVC, "proprietary_codecs", "protected_content"},
			Fixture:           "chromeVideoWithGuestLoginAndClearHEVCHWDecoding",
			Timeout:           10 * time.Minute,
		}, {
			Name:              "vp8_hw",
			Val:               memCheckParams{fileName: "720_vp8.webm", sizes: []graphics.Size{{Width: 1280, Height: 720}}, videoType: play.NormalVideo},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
			ExtraData:         []string{"video.html", "720_vp8.webm"},
			ExtraHardwareDeps: hwdep.D(hwdep.SupportsNV12Overlays()),
			ExtraSoftwareDeps: []string{"video_overlays", caps.HWDecodeVP8},
			Fixture:           "chromeVideoWithGuestLogin",
			Timeout:           10 * time.Minute,
		}, {
			Name:              "vp9_hw",
			Val:               memCheckParams{fileName: "720_vp9.webm", sizes: []graphics.Size{{Width: 1280, Height: 720}}, videoType: play.NormalVideo},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
			ExtraData:         []string{"video.html", "720_vp9.webm"},
			ExtraHardwareDeps: hwdep.D(hwdep.SupportsNV12Overlays()),
			ExtraSoftwareDeps: []string{"video_overlays", caps.HWDecodeVP9},
			Fixture:           "chromeVideoWithGuestLogin",
			Timeout:           10 * time.Minute,
		}, {
			Name:              "av1_hw_switch",
			Val:               memCheckParams{fileName: "dash_smpte_av1.mp4.mpd", sizes: []graphics.Size{{Width: 256, Height: 144}, {Width: 426, Height: 240}}, videoType: play.MSEVideo},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
			ExtraData:         append(play.MSEDataFiles(), "dash_smpte_av1.mp4.mpd", "dash_smpte_144.av1.mp4", "dash_smpte_240.av1.mp4"),
			ExtraHardwareDeps: hwdep.D(hwdep.SupportsNV12Overlays()),
			ExtraSoftwareDeps: []string{"video_overlays", caps.HWDecodeAV1},
			Fixture:           "chromeVideoWithGuestLoginAndHWAV1Decoding",
			Timeout:           10 * time.Minute,
		}, {
			Name:              "h264_hw_switch",
			Val:               memCheckParams{fileName: "cars_dash_mp4.mpd", sizes: []graphics.Size{{Width: 256, Height: 144}, {Width: 426, Height: 240}}, videoType: play.MSEVideo},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_weekly"},
			ExtraData:         append(play.MSEDataFiles(), "cars_dash_mp4.mpd", "cars_144_h264.mp4", "cars_240_h264.mp4"),
			ExtraHardwareDeps: hwdep.D(hwdep.SupportsNV12Overlays()),
			ExtraSoftwareDeps: []string{"video_overlays", caps.HWDecodeH264, "proprietary_codecs"},
			Fixture:           "chromeVideoWithGuestLogin",
			Timeout:           10 * time.Minute,
		}, {
			Name:              "hevc_hw_switch",
			Val:               memCheckParams{fileName: "cars_dash_hevc.mpd", sizes: []graphics.Size{{Width: 256, Height: 144}, {Width: 426, Height: 240}}, videoType: play.MSEVideo},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_weekly"},
			ExtraData:         append(play.MSEDataFiles(), "cars_dash_hevc.mpd", "cars_144_hevc.mp4", "cars_240_hevc.mp4"),
			ExtraHardwareDeps: hwdep.D(hwdep.SupportsNV12Overlays()),
			ExtraSoftwareDeps: []string{"video_overlays", caps.HWDecodeHEVC, "proprietary_codecs", "protected_content"},
			Fixture:           "chromeVideoWithGuestLoginAndClearHEVCHWDecoding",
			Timeout:           10 * time.Minute,
		}},
	})
}

// MemCheck plays a given fileName in Chrome and verifies there are no graphics
// memory leaks by comparing its usage before, during and after.
func MemCheck(ctx context.Context, s *testing.State) {
	testOpt := s.Param().(memCheckParams)
	cr := s.FixtValue().(*chrome.Chrome)
	const unmutePlayer = false

	testPlay := func() error {
		return play.TestPlay(ctx, s, cr, cr, testOpt.fileName, testOpt.videoType, play.VerifyHWAcceleratorUsed, unmutePlayer)
	}

	backend, err := graphics.GetBackend()
	if err != nil {
		s.Fatal("Error getting the graphics backend: ", err)
	}
	if err := graphics.VerifyGraphicsMemory(ctx, testPlay, backend, testOpt.sizes); err != nil {
		s.Fatal("Test failed: ", err)
	}
}
