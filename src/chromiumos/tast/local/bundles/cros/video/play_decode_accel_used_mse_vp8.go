// Copyright 2018 The Chromium OS Authors. All rights reserved.
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

func init() {
	testing.AddTest(&testing.Test{
		Func: PlayDecodeAccelUsedMSEVP8,
		Desc: "Verifies that VP8 video decode acceleration works when MSE is used",
		Contacts: []string{
			"acourbot@chromium.org",
			"mcasas@chromium.org",
			"chromeos-gfx-video@google.com",
			"chromeos-video-eng@google.com",
		},
		SoftwareDeps: []string{caps.HWDecodeVP8, "chrome"},
		Pre:          pre.ChromeVideo(),
		Data: append(
			play.MSEDataFiles(),
			"bear-320x240-video-only.vp8.webm",
			"bear-320x240-audio-only.vorbis.webm",
			"bear-320x240.vp8.mpd",
			play.ChromeMediaInternalsUtilsJSFile,
		),
		// Marked informational due to flakiness on ToT.
		// TODO(crbug.com/1008317): Promote to critical again.
		Attr: []string{"group:mainline", "informational"},
	})
}

// PlayDecodeAccelUsedMSEVP8 plays a VP8 video stream by shaka player, which uses
// Media Source Extensions (MSE).
// After that, it checks if video decode accelerator was used.
func PlayDecodeAccelUsedMSEVP8(ctx context.Context, s *testing.State) {
	play.TestPlay(ctx, s, s.PreValue().(*chrome.Chrome),
		"bear-320x240.vp8.mpd", play.MSEVideo, play.VerifyHwAcceleratorUsed)
}
