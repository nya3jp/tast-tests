// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package webrtc

import (
	"context"

	"chromiumos/tast/local/bundles/cros/webrtc/peerconnection"
	"chromiumos/tast/local/webrtc"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: RTCPeerConnection,
		Desc: "Verifies that WebRTC RTCPeerConnection in a loopback works",
		Contacts: []string{
			"mcasas@chromium.org", // Test author.
			"chromeos-gfx-video@google.com",
			"chromeos-video-eng@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Data:         append(webrtc.DataFiles(), "loopback_peerconnection.html"),
		Attr:         []string{"group:mainline", "informational"},
		Params: []testing.Param{{
			Name: "h264",
			Val:  "H264",
			// "chrome_internal" is needed because H.264 is a proprietary codec.
			ExtraSoftwareDeps: []string{"chrome_internal"},
		}, {
			Name: "vp8",
			Val:  "VP8",
		}, {
			Name: "vp9",
			Val:  "VP9",
		}},
	})
}

// RTCPeerConnection starts a loopback WebRTC call with two RTCPeerConnections
// and ensures it successfully establishes the call (otherwise the test will
// simply fail).
func RTCPeerConnection(ctx context.Context, s *testing.State) {
	peerconnection.RunRTCPeerConnection(ctx, s, s.Param().(string))
}
