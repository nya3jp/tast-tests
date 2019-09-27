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
		Func:         PlayDecodeAccelUsedVP9,
		Desc:         "Verifies that VP9 video decode acceleration works in Chrome",
		Contacts:     []string{"deanliao@chromium.org", "chromeos-video-eng@google.com"},
		SoftwareDeps: []string{caps.HWDecodeVP9, "chrome"},
		Pre:          pre.ChromeVideo(),
		Data:         []string{"720_vp9.webm", "video.html"},
		// Marked informational due to flakiness on ToT.
		// TODO(crbug.com/1008317): Promote to critical again.
		Attr: []string{"informational"},
	})
}

// PlayDecodeAccelUsedVP9 plays 720_vp9.webm with Chrome and
// checks if video decode accelerator was used.
func PlayDecodeAccelUsedVP9(ctx context.Context, s *testing.State) {
	play.TestPlay(ctx, s, s.PreValue().(*chrome.Chrome),
		"720_vp9.webm", play.NormalVideo, play.CheckHistogram)
}
