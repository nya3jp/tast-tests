// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package webrtc

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/media/pre"
	"chromiumos/tast/local/media/videotype"
	"chromiumos/tast/local/webrtc"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: PeerConnVP8,
		Desc: "Verifies that WebRTC loopback works (VP8)",
		Contacts: []string{
			"mcasas@chromium.org",
			"chromeos-gfx-video@google.com",
			"chromeos-video-eng@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Pre:          pre.ChromeVideoWithFakeWebcam(),
		Data:         append(webrtc.DataFiles(), "third_party/munge_sdp.js", "loopback_camera.html"),
	})
}

// PeerConnVP8 starts a loopback WebRTC call with two RTCPeerConnections and
// ensures it successfully establishes the call (otherwise the test will simply
// fail). If successful, it looks at the video frames coming out on the
// receiving side of the call and looks for freezes and black frames.
func PeerConnVP8(ctx context.Context, s *testing.State) {
	duration := 3 * time.Second

	webrtc.RunPeerConn(ctx, s, s.PreValue().(*chrome.Chrome), videotype.VP8,
		duration, webrtc.VerboseLogging)
}
