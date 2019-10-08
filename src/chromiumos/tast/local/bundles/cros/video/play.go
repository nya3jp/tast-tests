// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"

	"chromiumos/tast/local/bundles/cros/video/play"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/media/pre"
	"chromiumos/tast/testing"
)

// playTest is used to describe the config used to run each test.
type playTest struct {
	filename string // Filename containing the video to play.
}

func init() {
	testing.AddTest(&testing.Test{
		Func: Play,
		Desc: "Checks simple unrestricted (hw, sw) video playback in Chrome is working",
		Contacts: []string{
			"acourbot@chromium.org",
			"mcasas@chromium.org",
			"chromeos-gfx-video@google.com",
			"chromeos-video-eng@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Pre:          pre.ChromeVideo(),
		Data:         []string{"video.html"},
		Attr:         []string{"group:mainline"},
		Params: []testing.Param{{
			Name:      "av1",
			Val:       playTest{filename: "720p_30fps_300frames.av1.mp4"},
			ExtraData: []string{"720p_30fps_300frames.av1.mp4"},
		}, {
			Name:      "h264",
			Val:       playTest{filename: "720_h264.mp4"},
			ExtraData: []string{"720_h264.mp4"},
			// "chrome_internal" is needed because H.264 is a proprietary codec.
			ExtraSoftwareDeps: []string{"chrome_internal"},
		}, {
			Name:      "vp8",
			Val:       playTest{filename: "720_vp8.webm"},
			ExtraData: []string{"720_vp8.webm"},
		}, {
			Name:      "vp9",
			Val:       playTest{filename: "720_vp9.webm"},
			ExtraData: []string{"720_vp9.webm"},
		}},
	})
}

// Play plays a given file with Chrome.
func Play(ctx context.Context, s *testing.State) {
	play.TestPlay(ctx, s, s.PreValue().(*chrome.Chrome),
		s.Param().(playTest).filename, play.NormalVideo,
		play.NoVerifyHWAcceleratorUsed)
}
