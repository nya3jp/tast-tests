// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package webrtc

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/local/media/pre"
	"chromiumos/tast/local/media/videotype"
	"chromiumos/tast/local/media/vm"
	"chromiumos/tast/local/webrtc"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: PeerConnH264,
		Desc: "Verifies that WebRTC loopback works (H264)",
		Contacts: []string{
			"keiichiw@chromium.org", // Video team
			"shik@chromium.org",     // Camera team
			"chromeos-camera-eng@google.com",
		},
		Attr: []string{"informational"},
		// "chrome_internal" is needed because H.264 is a proprietary codec.
		SoftwareDeps: []string{caps.BuiltinOrVividCamera, "chrome", "chrome_internal"},
		Pre:          pre.ChromeVideo(),
		Data:         append(webrtc.DataFiles(), "third_party/munge_sdp.js", "loopback_camera.html"),
	})
}

// PeerConnH264 starts a loopback WebRTC call with two peer connections
// and ensures it successfully establishes the call (otherwise the test will
// simply fail). If successful, it looks at the video frames coming out on the
// receiving side of the call and looks for freezes and black frames.
//
// If this test shows black frames and camera.GetUserMedia does not, it could
// mean H.264 video isn't encoded/decoded right on this device but that the
// camera works.
//
// This test uses the real webcam unless it is running under QEMU. Under QEMU,
// it uses "vivid" instead, which is the virtual video test driver and can be
// used as an external USB camera.
func PeerConnH264(ctx context.Context, s *testing.State) {
	duration := 3 * time.Second
	// Since we use vivid on VM and it's slower than real cameras,
	// we use a longer time limit: https://crbug.com/929537
	if vm.IsRunningOnVM() {
		duration = 10 * time.Second
	}

	webrtc.RunPeerConn(ctx, s, s.PreValue().(*chrome.Chrome), videotype.H264,
		duration, webrtc.VerboseLogging)
}
