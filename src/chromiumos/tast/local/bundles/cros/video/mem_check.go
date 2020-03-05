// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/video/decode"
	"chromiumos/tast/local/bundles/cros/video/play"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/graphics"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/local/media/pre"
	"chromiumos/tast/testing"
)

type memCheckParams struct {
	fileName    string
	videoWidth  int
	videoHeight int
}

func init() {
	testing.AddTest(&testing.Test{
		Func: MemCheck,
		Desc: "Checks video playback in Chrome has no leaks",
		Contacts: []string{
			"mcasas@chromium.org",
			"chromeos-gfx-video@google.com",
		},
		SoftwareDeps: []string{"chrome", "graphics_debugfs"},
		Data:         []string{decode.ChromeMediaInternalsUtilsJSFile},
		Params: []testing.Param{{
			Name:              "h264_hw",
			Val:               memCheckParams{fileName: "720_h264.mp4", videoWidth: 1280, videoHeight: 720},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
			ExtraData:         []string{"video.html", "720_h264.mp4"},
			ExtraSoftwareDeps: []string{"amd64", "video_overlays", caps.HWDecodeH264, "chrome_internal"}, // "chrome_internal" is needed because H.264 is a proprietary codec.
			Pre:               pre.ChromeVideoWithGuestLogin(),
			Timeout:           10 * time.Minute,
		}, {
			Name:              "vp8_hw",
			Val:               memCheckParams{fileName: "720_vp8.webm", videoWidth: 1280, videoHeight: 720},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
			ExtraData:         []string{"video.html", "720_vp8.webm"},
			ExtraSoftwareDeps: []string{"amd64", "video_overlays", caps.HWDecodeVP8},
			Pre:               pre.ChromeVideoWithGuestLogin(),
			Timeout:           10 * time.Minute,
		}, {
			Name:              "vp9_hw",
			Val:               memCheckParams{fileName: "720_vp9.webm", videoWidth: 1280, videoHeight: 720},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
			ExtraData:         []string{"video.html", "720_vp9.webm"},
			ExtraSoftwareDeps: []string{"amd64", "video_overlays", caps.HWDecodeVP9},
			Pre:               pre.ChromeVideoWithGuestLogin(),
			Timeout:           10 * time.Minute,
		}},
	})
}

// MemCheck plays a given fileName in Chrome and verifies there are no graphics
// memory leaks by comparing its usage before, during and after.
func MemCheck(ctx context.Context, s *testing.State) {
	testOpt := s.Param().(memCheckParams)

	testPlay := func() error {
		return play.TestPlay(ctx, s, s.PreValue().(*chrome.Chrome), testOpt.fileName, play.NormalVideo, play.VerifyHWAcceleratorUsed)
	}

	backend, err := graphics.GetBackend()
	if err != nil {
		s.Fatal("Error getting the graphics backend: ", err)
	}
	if err := graphics.CompareGraphicsMemoryBeforeAfter(ctx, testPlay, backend, testOpt.videoWidth, testOpt.videoHeight); err != nil {
		s.Fatal("Test failed: ", err)
	}
}
