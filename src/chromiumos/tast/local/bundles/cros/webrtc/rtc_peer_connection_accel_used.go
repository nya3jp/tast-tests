// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package webrtc

import (
	"context"

	"chromiumos/tast/local/bundles/cros/webrtc/peerconnection"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/local/webrtc"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: RTCPeerConnectionAccelUsed,
		Desc: "Verifies that WebRTC RTCPeerConnection uses a hardware accelerator",
		Contacts: []string{
			"hiroh@chromium.org",
			"mcasas@chromium.org", // Test author.
			"chromeos-gfx-video@google.com",
			"chromeos-video-eng@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Data:         webrtc.LoopbackDataFiles(),
		Attr:         []string{"group:mainline"},
		Params: []testing.Param{{
			Name:              "enc_vp8",
			Val:               peerconnection.Encoding,
			ExtraSoftwareDeps: []string{caps.HWEncodeVP8},
		}, {
			Name:              "dec_vp8",
			Val:               peerconnection.Decoding,
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8},
		}},
	})
}

// RTCPeerConnectionAccelUsed verifies that a PeerConnection uses accelerated encoding / decoding.
func RTCPeerConnectionAccelUsed(ctx context.Context, s *testing.State) {
	peerconnection.RunPeerConnection(ctx, s, s.Param().(peerconnection.CodecType))
}
