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
	"chromiumos/tast/local/webrtc"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: PeerConnH264,
		Desc: "Verifies that WebRTC loopback works (H264)",
		Contacts: []string{
			"mcasas@chromium.org",
			"chromeos-gfx-video@google.com",
		},
		Attr: []string{"informational"},
		// "chrome_internal" is needed because H.264 is a proprietary codec.
		SoftwareDeps: []string{caps.BuiltinOrVividCamera, "chrome", "chrome_internal"},
		Pre:          pre.ChromeVideoWithFakeWebcam(),
		Data:         append(webrtc.DataFiles(), "third_party/munge_sdp.js", "loopback_camera.html"),
	})
}

// PeerConnH264 starts a loopback WebRTC call with two RTCPeerConnections and
// ensures it successfully establishes the call (otherwise the test will simply
// fail). If successful, it looks at the video frames coming out on the
// receiving side of the call and looks for freezes and black frames.
func PeerConnH264(ctx context.Context, s *testing.State) {
	duration := 3 * time.Second

	webrtc.RunPeerConn(ctx, s, s.PreValue().(*chrome.Chrome), videotype.H264,
		duration, webrtc.VerboseLogging)
}
