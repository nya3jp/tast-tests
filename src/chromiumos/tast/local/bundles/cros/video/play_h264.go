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
		Func:         PlayH264,
		Desc:         "Checks H264 video playback is working",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome_login"},
		Data:         []string{"bear_h264_320x180.mp4", "video.html"},
	})
}

func PlayH264(s *testing.State) {
	play.TestPlay(s, "bear_h264_320x180.mp4")
}
