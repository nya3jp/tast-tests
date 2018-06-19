// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"chromiumos/tast/local/bundles/cros/video/play"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PlayVP9,
		Desc:         "Checks VP9 video playback is working",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome_login"},
		Data:         []string{"bear_vp9_320x240.webm", "video.html"},
	})
}

func PlayVP9(s *testing.State) {
	play.TestPlay(s, "bear_vp9_320x240.webm")
}
