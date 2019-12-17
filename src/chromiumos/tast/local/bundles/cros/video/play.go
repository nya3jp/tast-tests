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

func init() {
	testing.AddTest(&testing.Test{
		Func: Play,
		Desc: "Checks simple unrestricted (HW, SW) video playback in Chrome is working",
		Contacts: []string{
			"acourbot@chromium.org",
			"mcasas@chromium.org",
			"chromeos-gfx-video@google.com",
			"chromeos-video-eng@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Pre:          pre.ChromeVideo(),
		Data:         []string{"video.html"},
		Attr:         []string{"group:graphics", "graphics_perbuild"},
		Params: []testing.Param{{
			Name:      "av1",
			Val:       "720p_30fps_300frames.av1.mp4",
			ExtraData: []string{"720p_30fps_300frames.av1.mp4"},
			ExtraAttr: []string{"informational"},
		}, {
			Name:      "h264",
			Val:       "720_h264.mp4",
			ExtraData: []string{"720_h264.mp4"},
			// "chrome_internal" is needed because H.264 is a proprietary codec.
			ExtraSoftwareDeps: []string{"chrome_internal"},
			// TODO(crbug.com/1029188): Promote to critical again.
			// This test is a fallout of ui.AssistantStartup errors
			ExtraAttr: []string{"informational"},
		}, {
			Name:      "vp8",
			Val:       "720_vp8.webm",
			ExtraData: []string{"720_vp8.webm"},
		}, {
			Name:      "vp9",
			Val:       "720_vp9.webm",
			ExtraData: []string{"720_vp9.webm"},
		}},
	})
}

// Play plays a given file with Chrome.
func Play(ctx context.Context, s *testing.State) {
	play.TestPlay(ctx, s, s.PreValue().(*chrome.Chrome),
		s.Param().(string), play.NormalVideo,
		play.NoVerifyHWAcceleratorUsed)
}
