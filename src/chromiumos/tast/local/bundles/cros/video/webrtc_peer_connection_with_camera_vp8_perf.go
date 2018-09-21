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
		Func:         WebRTCPeerConnectionWithCameraVP8Perf,
		Desc:         "Is the full version of video.WebRTCPeerConnectionWithCameraVP8.",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome_login"},
		Data:         append(webrtc.DataFiles(), "third_party/munge_sdp.js", "loopback.html"),
	})
}

// WebRTCPeerConnectionWithCameraVP8Perf is the full version of
// video.WebRTCPeerConnectionWithCameraVP8.
// This test performs a WebRTC loopback call for 20 seconds.
//
// This test uses the real webcam unless it is running under QEMU. Under QEMU,
// it uses "vivid" instead, which is the virtual video test driver and can be
// used as an external USB camera.
//
// TODO(keiichiw): When adding perf metrics, add comments.
func WebRTCPeerConnectionWithCameraVP8Perf(s *testing.State) {
	// Run loopback call for 20 seconds.
	webrtc.RunTest(s, "loopback.html", "testWebRtcLoopbackCall('VP8', 20)")
	// TODO(keiichiw): Add perf metrics.
}
