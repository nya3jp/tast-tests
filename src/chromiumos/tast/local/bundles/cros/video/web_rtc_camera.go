// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"chromiumos/tast/local/bundles/cros/video/webrtc"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WebRTCCamera,
		Desc:         "Test getUserMedia camera acquisition and that we get sane video",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome_login"},
		Data:         []string{"getusermedia.html", "blackframe.js", "ssim.js"},
	})
}

// This test makes WebRTC GetUserMedia call and renders the camera's media
// stream in a video tag. It uses the real webcam on the device.
//
// This test will test VGA and 720p (if supported by the device) and check
// if the gUM call succeeds.
//
// TODO(keiichiw): When adding perf metrics, add comments for them, too
func WebRTCCamera(s *testing.State) {
	webrtc.RunTest(s, "getusermedia.html", "testNextResolution()")
	// TODO(keiichiw): Add perf metrics
}
