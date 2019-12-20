// Copyright 2019 The Chromium OS Authors. All rights reserved.
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
)

type playParamsVD struct {
	fileName   string
	videoType  play.VideoType
	verifyMode play.VerifyHWAcceleratorMode
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         PlayVD,
		Desc:         "Checks simple video playback in Chrome is working when using a media::VideoDecoder (see go/vd-migration)",
		Contacts:     []string{"dstaessens@chromium.org", "akahuang@chromium.org", "chromeos-video-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"cros_video_decoder", "chrome"},
		Data:         []string{"video.html"},
		Pre:          pre.ChromeVideoVD(),
		Params: []testing.Param{{
			Name:              "h264",
			Val:               playParamsVD{fileName: "720_h264.mp4", videoType: play.NormalVideo, verifyMode: play.NoVerifyHWAcceleratorUsed},
			ExtraData:         []string{"video.html", "720_h264.mp4"},
			ExtraSoftwareDeps: []string{"chrome_internal"}, // "chrome_internal" is needed because H.264 is a proprietary codec.
		}, {
			Name:      "vp8",
			Val:       playParamsVD{fileName: "720_vp8.webm", videoType: play.NormalVideo, verifyMode: play.NoVerifyHWAcceleratorUsed},
			ExtraData: []string{"video.html", "720_vp8.webm"},
		}, {
			Name:      "vp9",
			Val:       playParamsVD{fileName: "720_vp9.webm", videoType: play.NormalVideo, verifyMode: play.NoVerifyHWAcceleratorUsed},
			ExtraData: []string{"video.html", "720_vp9.webm"},
		}, {
			Name:              "h264_hw",
			Val:               playParamsVD{fileName: "720_h264.mp4", videoType: play.NormalVideo, verifyMode: play.VerifyHWAcceleratorUsed},
			ExtraData:         []string{"video.html", "720_h264.mp4"},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "chrome_internal"}, // "chrome_internal" is needed because H.264 is a proprietary codec.
		}, {
			Name:              "vp8_hw",
			Val:               playParamsVD{fileName: "720_vp8.webm", videoType: play.NormalVideo, verifyMode: play.VerifyHWAcceleratorUsed},
			ExtraData:         []string{"video.html", "720_vp8.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8},
		}, {
			Name:              "vp9_hw",
			Val:               playParamsVD{fileName: "720_vp9.webm", videoType: play.NormalVideo, verifyMode: play.VerifyHWAcceleratorUsed},
			ExtraData:         []string{"video.html", "720_vp9.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
		}},
	})
}

// PlayVD plays a given file in Chrome and verifies that the playback happens
// correctly when using a media::VideoDecoder; if verifyMode says so, it
// verifies if playback uses hardware acceleration.
func PlayVD(ctx context.Context, s *testing.State) {
	testOpt := s.Param().(playParamsVD)
	play.TestPlay(ctx, s, s.PreValue().(*chrome.Chrome), testOpt.fileName, testOpt.videoType, testOpt.verifyMode)
}
