// Copyright 2018 The Chromium OS Authors. All rights reserved.
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
		Func:         PlayVP9,
		Desc:         "Checks VP9 video playback is working",
		Contacts:     []string{"deanliao@chromium.org", "chromeos-video-eng@google.com"},
		SoftwareDeps: []string{"chrome"},
		Pre:          pre.ChromeVideo(),
		Data:         []string{"720_vp9.webm", "video.html"},
	})
}

// PlayVP9 plays 720_vp9.webm with Chrome.
func PlayVP9(ctx context.Context, s *testing.State) {
	play.TestPlay(ctx, s, s.PreValue().(*chrome.Chrome),
		"720_vp9.webm", play.NormalVideo, play.NoCheckHistogram)
}
