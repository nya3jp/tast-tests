// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"chromiumos/tast/local/bundles/cros/video/playback"
	"chromiumos/tast/testing"
)

func init() {
	const (
		video = "traffic-1920x1080-8005020218f6b86bfa978e550d04956e.mp4"
	)
	testing.AddTest(&testing.Test{
		Func:         PlaybackPerfH264,
		Desc:         "Measure Playback Performance with/without HW accelerationr.",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome_login"},
		Data:         []string{video},
	})
}

func PlaybackPerfH264(s *testing.State) {
	const (
		video     = "traffic-1920x1080-8005020218f6b86bfa978e550d04956e.mp4"
		videoDesc = "h264_1080p"
	)

	playback.RunTest(s, video, videoDesc)
}
