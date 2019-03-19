// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/video/lib/caps"
	"chromiumos/tast/local/bundles/cros/video/lib/pre"
	"chromiumos/tast/local/bundles/cros/video/lib/vm"
	"chromiumos/tast/local/bundles/cros/video/webrtc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WebRTCCamera,
		Desc:         "Verifies that getUserMedia captures video",
		Contacts:     []string{"keiichiw@chromium.org", "chromeos-video-eng@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{caps.BuiltinCamera, "chrome_login", "camera_720p"},
		Pre:          pre.LoggedInVideo(),
		Data:         append(webrtc.DataFiles(), "getusermedia.html"),
	})
}

// WebRTCCamera makes WebRTC getUserMedia call and renders the camera's media
// stream in a video tag. It will test VGA and 720p and check if the gUM call succeeds.
// This test will fail when an error occurs or too many frames are broken.
//
// WebRTCCamera performs video capturing for 3 seconds with 480p and 720p.
// It is a short version of video.WebRTCCameraPerf.
//
// This test uses the real webcam unless it is running under QEMU. Under QEMU,
// it uses "vivid" instead, which is the virtual video test driver and can be
// used as an external USB camera. In this case, the time limit is 10 seconds.
func WebRTCCamera(ctx context.Context, s *testing.State) {
	duration := 3 * time.Second
	// Since we use vivid on VM and it's slower than real cameras,
	// we use a longer time limit: https://crbug.com/929537
	if vm.IsRunningOnVM() {
		duration = 10 * time.Second
	}

	// Run tests for 480p and 720p.
	webrtc.RunWebRTCCamera(ctx, s, s.PreValue().(*chrome.Chrome), duration)
}
