// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/local/media/pre"
	"chromiumos/tast/local/media/vm"
	"chromiumos/tast/local/webrtc"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: GetUserMedia,
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

// GetUserMedia calls getUserMedia call and renders the camera's media stream
// in a video tag. It will test VGA and 720p and check if the gUM call succeeds.
// This test will fail when an error occurs or too many frames are broken.
//
// GetUserMedia performs video capturing for 3 seconds with 480p and 720p.
// (It's 10 seconds in case it runs under QEMU.) This a short version of
// camera.GetUserMediaPerf.
//
// This test uses the real webcam unless it is running under QEMU. Under QEMU,
// it uses "vivid" instead, which is the virtual video test driver and can be
// used as an external USB camera. In this case, the time limit is 10 seconds.
func GetUserMedia(ctx context.Context, s *testing.State) {
	duration := 3 * time.Second
	// Since we use vivid on VM and it's slower than real cameras,
	// we use a longer time limit: https://crbug.com/929537
	if vm.IsRunningOnVM() {
		duration = 10 * time.Second
	}

	// Run tests for 480p and 720p.
	webrtc.RunGetUserMedia(ctx, s, s.PreValue().(*chrome.Chrome), duration,
		webrtc.VerboseLogging)
}
