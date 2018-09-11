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
		Desc:         "Ensures getUserMedia acquires a video input device and it provides a sane video",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome_login"},
		Data:         append(webrtc.DataFiles(), "getusermedia.html"),
	})
}

// WebRTCCamera makes WebRTC GetUserMedia call and renders the camera's media
// stream in a video tag. It will test VGA and 720p (if supported by the device)
// and check if the gUM call succeeds.
//
// This test uses the real webcam unless it is running under QEMU. Under QEMU,
// it uses "vivid" instead, which is the virtual video test driver and can be
// used as an external USB camera.
//
// TODO(keiichiw): When adding perf metrics, add comments here.
func WebRTCCamera(s *testing.State) {
	webrtc.RunTest(s, "getusermedia.html", "testNextResolution()")
	// TODO(keiichiw): Add perf metrics.
}
