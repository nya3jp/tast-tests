// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"

	"chromiumos/tast/local/bundles/cros/video/decode"
	"chromiumos/tast/local/bundles/cros/video/play"
	"chromiumos/tast/local/media/caps"
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
		Attr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
		Params: []testing.Param{{
			Name:      "av1",
			Val:       playParams{fileName: "720p_30fps_300frames.av1.mp4", videoType: play.NormalVideo, verifyMode: play.NoVerifyHWAcceleratorUsed},
			ExtraData: []string{"video.html", "720p_30fps_300frames.av1.mp4"},
		}, {
			Name:              "h264",
			Val:               playParams{fileName: "720_h264.mp4", videoType: play.NormalVideo, verifyMode: play.NoVerifyHWAcceleratorUsed},
			ExtraData:         []string{"video.html", "720_h264.mp4"},
			ExtraSoftwareDeps: []string{"chrome_internal"}, // "chrome_internal" is needed because H.264 is a proprietary codec.
		}, {
			Name:      "vp8",
			Val:       playParams{fileName: "720_vp8.webm", videoType: play.NormalVideo, verifyMode: play.NoVerifyHWAcceleratorUsed},
			ExtraData: []string{"video.html", "720_vp8.webm"},
		}, {
			Name:      "vp9",
			Val:       playParams{fileName: "720_vp9.webm", videoType: play.NormalVideo, verifyMode: play.NoVerifyHWAcceleratorUsed},
			ExtraData: []string{"video.html", "720_vp9.webm"},
		}, {
			Name:              "h264_sw",
			Val:               playParams{fileName: "720_h264.mp4", videoType: play.NormalVideo, verifyMode: play.VerifyNoHWAcceleratorUsed},
			ExtraData:         []string{"video.html", "720_h264.mp4"},
			ExtraSoftwareDeps: []string{"chrome_internal"}, // "chrome_internal" is needed because H.264 is a proprietary codec.
		}, {
			Name:      "vp8_sw",
			Val:       playParams{fileName: "720_vp8.webm", videoType: play.NormalVideo, verifyMode: play.VerifyNoHWAcceleratorUsed},
			ExtraData: []string{"video.html", "720_vp8.webm"},
		}, {
			Name:      "vp9_sw",
			Val:       playParams{fileName: "720_vp9.webm", videoType: play.NormalVideo, verifyMode: play.VerifyNoHWAcceleratorUsed},
			ExtraData: []string{"video.html", "720_vp9.webm"},
		}, {
			Name:              "h264_hw",
			Val:               playParams{fileName: "720_h264.mp4", videoType: play.NormalVideo, verifyMode: play.VerifyHWAcceleratorUsed},
			ExtraData:         []string{"video.html", "720_h264.mp4"},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "chrome_internal"}, // "chrome_internal" is needed because H.264 is a proprietary codec.
		}, {
			Name:              "vp8_hw",
			Val:               playParams{fileName: "720_vp8.webm", videoType: play.NormalVideo, verifyMode: play.VerifyHWAcceleratorUsed},
			ExtraData:         []string{"video.html", "720_vp8.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8},
		}, {
			Name:              "vp9_hw",
			Val:               playParams{fileName: "720_vp9.webm", videoType: play.NormalVideo, verifyMode: play.VerifyHWAcceleratorUsed},
			ExtraData:         []string{"video.html", "720_vp9.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
		}, {
			Name:              "h264_hw_mse",
			Val:               playParams{fileName: "bear-320x240.h264.mpd", videoType: play.MSEVideo, verifyMode: play.VerifyHWAcceleratorUsed},
			ExtraData:         append(play.MSEDataFiles(), "bear-320x240-video-only.h264.mp4", "bear-320x240-audio-only.aac.mp4", "bear-320x240.h264.mpd"),
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "chrome_internal"}, // "chrome_internal" is needed because H.264 is a proprietary codec.
		}, {
			Name:              "vp8_hw_mse",
			Val:               playParams{fileName: "bear-320x240.vp8.mpd", videoType: play.MSEVideo, verifyMode: play.VerifyHWAcceleratorUsed},
			ExtraData:         append(play.MSEDataFiles(), "bear-320x240-video-only.vp8.webm", "bear-320x240-audio-only.vorbis.webm", "bear-320x240.vp8.mpd"),
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8},
		}, {
			Name:              "vp9_hw_mse",
			Val:               playParams{fileName: "bear-320x240.vp9.mpd", videoType: play.MSEVideo, verifyMode: play.VerifyHWAcceleratorUsed},
			ExtraData:         append(play.MSEDataFiles(), "bear-320x240-video-only.vp9.webm", "bear-320x240-audio-only.opus.webm", "bear-320x240.vp9.mpd"),
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
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
	play.TestPlayWithArgs(ctx, s, testOpt.fileName, testOpt.videoType, testOpt.verifyMode)
}
