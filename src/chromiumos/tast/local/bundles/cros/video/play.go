// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"

	"chromiumos/tast/local/bundles/cros/video/decode"
	"chromiumos/tast/local/bundles/cros/video/play"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/local/media/pre"
	"chromiumos/tast/testing"
)

type playParams struct {
	fileName   string
	videoType  play.VideoType
	verifyMode play.VerifyHWAcceleratorMode
}

func init() {
	testing.AddTest(&testing.Test{
		Func: Play,
		Desc: "Checks simple video playback in Chrome is working",
		Contacts: []string{
			"mcasas@chromium.org",
			"chromeos-gfx-video@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{decode.ChromeMediaInternalsUtilsJSFile},
		Params: []testing.Param{{
			Name:      "av1",
			Val:       playParams{fileName: "720p_30fps_300frames.av1.mp4", videoType: play.NormalVideo, verifyMode: play.NoVerifyHWAcceleratorUsed},
			ExtraAttr: []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData: []string{"video.html", "720p_30fps_300frames.av1.mp4"},
			Pre:       pre.ChromeVideo(),
		}, {
			Name:              "h264",
			Val:               playParams{fileName: "bear-320x240.h264.mp4", videoType: play.NormalVideo, verifyMode: play.NoVerifyHWAcceleratorUsed},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         []string{"video.html", "bear-320x240.h264.mp4"},
			ExtraSoftwareDeps: []string{"chrome_internal"}, // "chrome_internal" is needed because H.264 is a proprietary codec.
			Pre:               pre.ChromeVideo(),
		}, {
			Name:      "vp8",
			Val:       playParams{fileName: "bear-320x240.vp8.webm", videoType: play.NormalVideo, verifyMode: play.NoVerifyHWAcceleratorUsed},
			ExtraAttr: []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData: []string{"video.html", "bear-320x240.vp8.webm"},
			Pre:       pre.ChromeVideo(),
		}, {
			Name:      "vp9",
			Val:       playParams{fileName: "bear-320x240.vp9.webm", videoType: play.NormalVideo, verifyMode: play.NoVerifyHWAcceleratorUsed},
			ExtraAttr: []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData: []string{"video.html", "bear-320x240.vp9.webm"},
			Pre:       pre.ChromeVideo(),
		}, {
			Name:              "h264_sw",
			Val:               playParams{fileName: "bear-320x240.h264.mp4", videoType: play.NormalVideo, verifyMode: play.VerifyNoHWAcceleratorUsed},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         []string{"video.html", "bear-320x240.h264.mp4"},
			ExtraSoftwareDeps: []string{"chrome_internal"}, // "chrome_internal" is needed because H.264 is a proprietary codec.
			Pre:               pre.ChromeVideoWithSWDecoding(),
		}, {
			Name:      "vp8_sw",
			Val:       playParams{fileName: "bear-320x240.vp8.webm", videoType: play.NormalVideo, verifyMode: play.VerifyNoHWAcceleratorUsed},
			ExtraAttr: []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData: []string{"video.html", "bear-320x240.vp8.webm"},
			Pre:       pre.ChromeVideoWithSWDecoding(),
		}, {
			Name:      "vp9_sw",
			Val:       playParams{fileName: "bear-320x240.vp9.webm", videoType: play.NormalVideo, verifyMode: play.VerifyNoHWAcceleratorUsed},
			ExtraAttr: []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData: []string{"video.html", "bear-320x240.vp9.webm"},
			Pre:       pre.ChromeVideoWithSWDecoding(),
		}, {
			Name:              "h264_hw",
			Val:               playParams{fileName: "bear-320x240.h264.mp4", videoType: play.NormalVideo, verifyMode: play.VerifyHWAcceleratorUsed},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         []string{"video.html", "bear-320x240.h264.mp4"},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "chrome_internal"}, // "chrome_internal" is needed because H.264 is a proprietary codec.
			Pre:               pre.ChromeVideo(),
		}, {
			Name:              "vp8_hw",
			Val:               playParams{fileName: "bear-320x240.vp8.webm", videoType: play.NormalVideo, verifyMode: play.VerifyHWAcceleratorUsed},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         []string{"video.html", "bear-320x240.vp8.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8},
			Pre:               pre.ChromeVideo(),
		}, {
			Name:              "vp9_hw",
			Val:               playParams{fileName: "bear-320x240.vp9.webm", videoType: play.NormalVideo, verifyMode: play.VerifyHWAcceleratorUsed},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         []string{"video.html", "bear-320x240.vp9.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
			Pre:               pre.ChromeVideo(),
		}, {
			Name:              "h264_hw_mse",
			Val:               playParams{fileName: "bear-320x240.h264.mpd", videoType: play.MSEVideo, verifyMode: play.VerifyHWAcceleratorUsed},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         append(play.MSEDataFiles(), "bear-320x240-video-only.h264.mp4", "bear-320x240-audio-only.aac.mp4", "bear-320x240.h264.mpd"),
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "chrome_internal"}, // "chrome_internal" is needed because H.264 is a proprietary codec.
			Pre:               pre.ChromeVideo(),
		}, {
			Name:              "vp8_hw_mse",
			Val:               playParams{fileName: "bear-320x240.vp8.mpd", videoType: play.MSEVideo, verifyMode: play.VerifyHWAcceleratorUsed},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         append(play.MSEDataFiles(), "bear-320x240-video-only.vp8.webm", "bear-320x240-audio-only.vorbis.webm", "bear-320x240.vp8.mpd"),
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8},
			Pre:               pre.ChromeVideo(),
		}, {
			Name:              "vp9_hw_mse",
			Val:               playParams{fileName: "bear-320x240.vp9.mpd", videoType: play.MSEVideo, verifyMode: play.VerifyHWAcceleratorUsed},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         append(play.MSEDataFiles(), "bear-320x240-video-only.vp9.webm", "bear-320x240-audio-only.opus.webm", "bear-320x240.vp9.mpd"),
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
			Pre:               pre.ChromeVideo(),
		}, {
			Name:      "av1_guest",
			Val:       playParams{fileName: "720p_30fps_300frames.av1.mp4", videoType: play.NormalVideo, verifyMode: play.NoVerifyHWAcceleratorUsed},
			ExtraAttr: []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData: []string{"video.html", "720p_30fps_300frames.av1.mp4"},
			Pre:       pre.ChromeVideoWithGuestLogin(),
		}, {
			Name:              "h264_guest",
			Val:               playParams{fileName: "bear-320x240.h264.mp4", videoType: play.NormalVideo, verifyMode: play.NoVerifyHWAcceleratorUsed},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         []string{"video.html", "bear-320x240.h264.mp4"},
			ExtraSoftwareDeps: []string{"chrome_internal"}, // "chrome_internal" is needed because H.264 is a proprietary codec.
			Pre:               pre.ChromeVideoWithGuestLogin(),
		}, {
			Name:      "vp8_guest",
			Val:       playParams{fileName: "bear-320x240.vp8.webm", videoType: play.NormalVideo, verifyMode: play.NoVerifyHWAcceleratorUsed},
			ExtraAttr: []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData: []string{"video.html", "bear-320x240.vp8.webm"},
			Pre:       pre.ChromeVideoWithGuestLogin(),
		}, {
			Name:      "vp9_guest",
			Val:       playParams{fileName: "bear-320x240.vp9.webm", videoType: play.NormalVideo, verifyMode: play.NoVerifyHWAcceleratorUsed},
			ExtraAttr: []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData: []string{"video.html", "bear-320x240.vp9.webm"},
			Pre:       pre.ChromeVideoWithGuestLogin(),
		}, {
			Name:              "h264_hw_alt",
			Val:               playParams{fileName: "bear-320x240.h264.mp4", videoType: play.NormalVideo, verifyMode: play.VerifyHWAcceleratorUsed},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         []string{"video.html", "bear-320x240.h264.mp4"},
			ExtraSoftwareDeps: []string{"cros_video_decoder", caps.HWDecodeH264, "chrome_internal"}, // "chrome_internal" is needed because H.264 is a proprietary codec.
			Pre:               pre.ChromeVideoVD(),
		}, {
			Name:              "vp8_hw_alt",
			Val:               playParams{fileName: "bear-320x240.vp8.webm", videoType: play.NormalVideo, verifyMode: play.VerifyHWAcceleratorUsed},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         []string{"video.html", "bear-320x240.vp8.webm"},
			ExtraSoftwareDeps: []string{"cros_video_decoder", caps.HWDecodeVP8},
			Pre:               pre.ChromeVideoVD(),
		}, {
			Name:              "vp9_hw_alt",
			Val:               playParams{fileName: "bear-320x240.vp9.webm", videoType: play.NormalVideo, verifyMode: play.VerifyHWAcceleratorUsed},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         []string{"video.html", "bear-320x240.vp9.webm"},
			ExtraSoftwareDeps: []string{"cros_video_decoder", caps.HWDecodeVP9},
			Pre:               pre.ChromeVideoVD(),
		}},
	})
}

// Play plays a given file in Chrome and verifies that the playback happens
// correctly; if verifyMode says so, it verifies if playback uses hardware
// acceleration.
// If videoType is NormalVideo, a simple <video> player is instantiated with the
// input filename, whereas if it's MSEVideo,then we try to feed the media files
// via a SourceBuffer (using MSE, the Media Source Extensions protocol, and a
// DASH MPD file).
func Play(ctx context.Context, s *testing.State) {
	testOpt := s.Param().(playParams)
	play.TestPlay(ctx, s, s.PreValue().(*chrome.Chrome), testOpt.fileName, testOpt.videoType, testOpt.verifyMode)
}
