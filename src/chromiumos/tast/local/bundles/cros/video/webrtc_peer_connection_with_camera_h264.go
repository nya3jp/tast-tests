// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/video/lib/caps"
	"chromiumos/tast/local/bundles/cros/video/lib/videotype"
	"chromiumos/tast/local/bundles/cros/video/webrtc"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WebRTCPeerConnectionWithCameraH264,
		Desc:         "Verifies that WebRTC loopback works (H264)",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{caps.USBCamera, "chrome_login", "chrome_internal"},
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
// camera works.
//
// This test uses the real webcam unless it is running under QEMU. Under QEMU,
// it uses "vivid" instead, which is the virtual video test driver and can be
// used as an external USB camera.
func WebRTCPeerConnectionWithCameraH264(ctx context.Context, s *testing.State) {
	// Run loopback call for 3 seconds.
	webrtc.RunWebRTCPeerConnectionWithCamera(ctx, s, videotype.H264, 3*time.Second)
}
