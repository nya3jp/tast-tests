// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/video/lib/caps"
	"chromiumos/tast/local/bundles/cros/video/webrtc"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WebRTCCamera,
		Desc:         "Verifies that getUserMedia captures video",
		SoftwareDeps: []string{caps.USBCamera, "chrome_login", "camera_720p"},
		Data:         append(webrtc.DataFiles(), "getusermedia.html"),
	})
}

// WebRTCCamera makes WebRTC getUserMedia call and renders the camera's media
// stream in a video tag. It will test VGA and 720p and check if the gUM call succeeds.
// This test will fail when an error occurs or too many frames are broken.
//
// WebRTCCamera performs video capturing for 3 seconds. It is a short version of
// video.WebRTCCameraPerf.
//
// This test uses the real webcam unless it is running under QEMU. Under QEMU,
// it uses "vivid" instead, which is the virtual video test driver and can be
// used as an external USB camera.
func WebRTCCamera(ctx context.Context, s *testing.State) {
	// Run tests for 3 seconds per resolution.
	webrtc.RunWebRTCCamera(ctx, s, 3*time.Second)
}
