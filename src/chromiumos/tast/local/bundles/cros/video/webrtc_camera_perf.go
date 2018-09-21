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
		Func:         WebRTCCameraPerf,
		Desc:         "Is the full version of WebRTCCamera.",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome_login"},
		Data:         append(webrtc.DataFiles(), "getusermedia.html"),
	})
}

// WebRTCCameraPerf is the full version of WebRTCCamera.
// It renders the camera's media stream in VGA and 720p (if supported) for
// 20 seconds.
//
// This test uses the real webcam unless it is running under QEMU. Under QEMU,
// it uses "vivid" instead, which is the virtual video test driver and can be
// used as an external USB camera.
//
// TODO(keiichiw): When adding perf metrics, add comments here.
func WebRTCCameraPerf(s *testing.State) {
	// Run tests for 20 seconds per resolutions.
	webrtc.RunTest(s, "getusermedia.html", "testNextResolution(20)")
	// TODO(keiichiw): Add perf metrics.
}
