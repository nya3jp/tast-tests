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

type testParams struct {
	fileName  string
	videoType play.VideoType
}

func init() {
	testing.AddTest(&testing.Test{
		Func: PlayDecodeAccelUsed,
		Desc: "Verifies that video decode acceleration works in Chrome",
		Contacts: []string{
			"acourbot@chromium.org",
			"mcasas@chromium.org",
			"chromeos-gfx-video@google.com",
			"chromeos-video-eng@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Pre:          pre.ChromeVideo(),
		Data:         []string{decode.ChromeMediaInternalsUtilsJSFile},
		// Marked informational due to flakiness on ToT.
		// TODO(crbug.com/1008317): Promote to critical again.
		Attr: []string{"group:graphics", "graphics_perbuild", "informational"},
		Params: []testing.Param{{
			Name: "h264",
			Val:  testParams{fileName: "720_h264.mp4", videoType: play.NormalVideo},
			// "chrome_internal" is needed because H.264 is a proprietary codec.
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "chrome_internal"},
			ExtraData:         []string{"video.html", "720_h264.mp4"},
		}, {
			Name:              "vp8",
			Val:               testParams{fileName: "720_vp8.webm", videoType: play.NormalVideo},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8},
			ExtraData:         []string{"video.html", "720_vp8.webm"},
		}, {
			Name:              "vp9",
			Val:               testParams{fileName: "720_vp9.webm", videoType: play.NormalVideo},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
			ExtraData:         []string{"video.html", "720_vp9.webm"},
		}, {
			Name: "mse_h264",
			Val:  testParams{fileName: "bear-320x240.h264.mpd", videoType: play.MSEVideo},
			// "chrome_internal" is needed because H.264 is a proprietary codec.
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "chrome_internal"},
			ExtraData: append(
				play.MSEDataFiles(),
				"bear-320x240-video-only.h264.mp4",
				"bear-320x240-audio-only.aac.mp4",
				"bear-320x240.h264.mpd"),
		}, {
			Name:              "mse_vp8",
			Val:               testParams{fileName: "bear-320x240.vp8.mpd", videoType: play.MSEVideo},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8},
			ExtraData: append(
				play.MSEDataFiles(),
				"bear-320x240-video-only.vp8.webm",
				"bear-320x240-audio-only.vorbis.webm",
				"bear-320x240.vp8.mpd"),
		}, {
			Name:              "mse_vp9",
			Val:               testParams{fileName: "bear-320x240.vp9.mpd", videoType: play.MSEVideo},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
			ExtraData: append(
				play.MSEDataFiles(),
				"bear-320x240-video-only.vp9.webm",
				"bear-320x240-audio-only.opus.webm",
				"bear-320x240.vp9.mpd"),
		}},
	})
}

// PlayDecodeAccelUsed plays a given file with Chrome and verifies a video
// decode accelerator was used. If videoType is NormalVideo, a simple <video>
// player is instantiated with the input video file as source URL, whereas if
// it's MSEVideo,then TestPlay tries to feed the media files via a SourceBuffer
// (using MSE, the Media Source Extensions protocol, and a DASH MPD file).
func PlayDecodeAccelUsed(ctx context.Context, s *testing.State) {
	testOpt := s.Param().(testParams)
	play.TestPlay(ctx, s, s.PreValue().(*chrome.Chrome),
		testOpt.fileName, testOpt.videoType, play.VerifyHWAcceleratorUsed)
}
