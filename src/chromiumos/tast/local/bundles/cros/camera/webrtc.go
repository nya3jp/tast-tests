// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"time"

	// TODO(crbug.com/963772) Move libraries in video to camera or media folder.
	"chromiumos/tast/local/bundles/cros/video/lib/caps"
	"chromiumos/tast/local/bundles/cros/video/lib/pre"
	"chromiumos/tast/local/bundles/cros/video/lib/vm"
	"chromiumos/tast/local/bundles/cros/video/webrtc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: WebRTC,
		Desc: "Verifies that getUserMedia captures video",
		Contacts: []string{
			"keiichiw@chromium.org", // Video team
			"shik@chromium.org",     // Camera team
			"chromeos-camera-eng@google.com",
		},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{caps.BuiltinOrVividCamera, "chrome", "camera_720p"},
		Pre:          pre.ChromeVideo(),
		Data:         append(webrtc.DataFiles(), "getusermedia.html"),
	})
}

// WebRTC makes WebRTC getUserMedia call and renders the camera's media stream
// in a video tag. It will test VGA and 720p and check if the gUM call succeeds.
// This test will fail when an error occurs or too many frames are broken.
//
// WebRTC performs video capturing for 3 seconds with 480p and 720p. It is a
// short version of video.WebRTCPerf.
//
// This test uses the real webcam unless it is running under QEMU. Under QEMU,
// it uses "vivid" instead, which is the virtual video test driver and can be
// used as an external USB camera. In this case, the time limit is 10 seconds.
func WebRTC(ctx context.Context, s *testing.State) {
	duration := 3 * time.Second
	// Since we use vivid on VM and it's slower than real cameras,
	// we use a longer time limit: https://crbug.com/929537
	if vm.IsRunningOnVM() {
		duration = 10 * time.Second
	}

	// Run tests for 480p and 720p.
	webrtc.RunWebRTC(ctx, s, s.PreValue().(*chrome.Chrome), duration,
		webrtc.VerboseLogging)
}
