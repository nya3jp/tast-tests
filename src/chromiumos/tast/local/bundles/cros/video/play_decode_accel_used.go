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
		Data:         []string{"video.html", decode.ChromeMediaInternalsUtilsJSFile},
		// Marked informational due to flakiness on ToT.
		// TODO(crbug.com/1008317): Promote to critical again.
		Attr: []string{"group:mainline", "informational"},
		Params: []testing.Param{{
			Name:      "h264",
			Val:       "720_h264.mp4",
			ExtraData: []string{"720_h264.mp4"},
			// "chrome_internal" is needed because H.264 is a proprietary codec.
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "chrome_internal"},
		}, {
			Name:              "vp8",
			Val:               "720_vp8.webm",
			ExtraData:         []string{"720_vp8.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8},
		}, {
			Name:              "vp9",
			Val:               "720_vp9.webm",
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
			ExtraData:         []string{"720_vp9.webm"},
		}},
	})
}

// PlayDecodeAccelUsed plays a given file with Chrome and verifies a video
// decode accelerator was used.
func PlayDecodeAccelUsed(ctx context.Context, s *testing.State) {
	play.TestPlay(ctx, s, s.PreValue().(*chrome.Chrome),
		s.Param().(string), play.NormalVideo, play.VerifyHWAcceleratorUsed)
}
