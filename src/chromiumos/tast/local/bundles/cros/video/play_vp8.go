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
		Func:         PlayVP8,
		Desc:         "Checks VP8 video playback is working",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome_login"},
		Data:         []string{"bear_vp8_320x180_20180629.webm", "video.html"},
	})
}

func PlayVP8(s *testing.State) {
	play.TestPlay(s, "bear_vp8_320x180_20180629.webm")
}
