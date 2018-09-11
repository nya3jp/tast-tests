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
		Func:         WebRTCPeerConnectionWithCameraH264,
		Desc:         "Ensures WebRTC call can be established and produces healthy H264 video",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome_login"},
		Data:         append(webrtc.DataFiles(), "third_party/munge_sdp.js", "loopback.html"),
	})
}

// WebRTCPeerConnectionWithCameraH264 starts a loopback WebRTC call with two
// peer connections and ensures it successfully establishes the call (otherwise
// the test will simply fail). If successful, it looks at the video frames
// coming out on the receiving side of the call and looks for freezes and black
// frames.
//
// If this test shows black frames and video.WebRTCCamera does not, it could
// mean H264 video isn't encoded/decoded right on this device but that the
// camera works. Finally, input and output FPS are logged.
//
// This test uses the real webcam unless it is running under QEMU. Under QEMU,
// it uses "vivid" instead, which is the virtual video test driver and can be
// used as an external USB camera.
//
// TODO(keiichiw): When adding perf metrics, add comments here.
func WebRTCPeerConnectionWithCameraH264(s *testing.State) {
	webrtc.RunTest(s, "loopback.html", "testWebRtcLoopbackCall('H264')")
	// TODO(keiichiw): Add perf metrics.
}
