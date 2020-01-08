// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package webrtc

import (
	"context"

	"chromiumos/tast/local/bundles/cros/webrtc/peerconnection"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/local/media/pre"
	"chromiumos/tast/local/webrtc"
	"chromiumos/tast/testing"
)

// rtcTest is used to describe the config used to run each test case.
type rtcTest struct {
	codec   peerconnection.CodecType // Encoding or decoding.
	profile string                   // Codec to try, e.g. VP8, VP9.
}

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
		Data:         append(webrtc.DataFiles(), peerconnection.LoopbackFile),
		Attr:         []string{"group:mainline"},
		// TODO(crbug.com/1017374): add "enc_vp9".
		Params: []testing.Param{{
			Name:              "enc_vp8",
			Val:               rtcTest{codec: peerconnection.Encoding, profile: "VP8"},
			ExtraSoftwareDeps: []string{caps.HWEncodeVP8},
			Pre:               pre.ChromeVideoWithFakeWebcam(),
		}, {
			Name:              "dec_vp8",
			Val:               rtcTest{codec: peerconnection.Decoding, profile: "VP8"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8},
			Pre:               pre.ChromeVideoWithFakeWebcam(),
		}, {
			Name:              "dec_vp9",
			Val:               rtcTest{codec: peerconnection.Decoding, profile: "VP9"},
			ExtraAttr:         []string{"informational"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
			Pre:               pre.ChromeVideoWithFakeWebcam(),
		}, {
			Name:              "enc_h264",
			Val:               rtcTest{codec: peerconnection.Encoding, profile: "H264"},
			ExtraAttr:         []string{"informational"},
			ExtraSoftwareDeps: []string{caps.HWEncodeH264, "chrome_internal"}, // "chrome_internal" is needed because H.264 is a proprietary codec.
			Pre:               pre.ChromeVideoWithFakeWebcamAndH264AMDEncoder(),
		}, {
			Name:              "dec_h264",
			Val:               rtcTest{codec: peerconnection.Decoding, profile: "H264"},
			ExtraAttr:         []string{"informational"},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "chrome_internal"}, // "chrome_internal" is needed because H.264 is a proprietary codec.
			Pre:               pre.ChromeVideoWithFakeWebcam(),
		}},
	})
}

// RTCPeerConnectionAccelUsed verifies that a PeerConnection uses accelerated encoding / decoding.
func RTCPeerConnectionAccelUsed(ctx context.Context, s *testing.State) {
	testOpt := s.Param().(rtcTest)
	peerconnection.RunRTCPeerConnectionAccelUsed(ctx, s, s.PreValue().(*chrome.Chrome), testOpt.codec, testOpt.profile)
}
