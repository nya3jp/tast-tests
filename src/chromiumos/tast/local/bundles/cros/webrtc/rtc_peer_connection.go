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
	codec   peerconnection.CodecType // Encoding, decoding, or don't care.
	profile string                   // Codec to try, e.g. VP8, VP9.
}

func init() {
	testing.AddTest(&testing.Test{
		Func: RTCPeerConnection,
		Desc: "Verifies that WebRTC RTCPeerConnection works, maybe verifying use of a hardware accelerator",
		Contacts: []string{
			"mcasas@chromium.org", // Test author.
			"chromeos-gfx-video@google.com",
			"chromeos-video-eng@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Data:         append(webrtc.DataFiles(), peerconnection.LoopbackFile),
		Attr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
		// TODO(crbug.com/1017374): add "vp9_enc".
		Params: []testing.Param{{
			Name:              "vp8_enc",
			Val:               rtcTest{codec: peerconnection.Encoding, profile: "VP8"},
			ExtraSoftwareDeps: []string{caps.HWEncodeVP8},
			Pre:               pre.ChromeVideoWithFakeWebcam(),
		}, {
			Name:              "vp8_dec",
			Val:               rtcTest{codec: peerconnection.Decoding, profile: "VP8"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8},
			Pre:               pre.ChromeVideoWithFakeWebcam(),
		}, {
			Name:              "vp9_dec",
			Val:               rtcTest{codec: peerconnection.Decoding, profile: "VP9"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
			Pre:               pre.ChromeVideoWithFakeWebcam(),
		}, {
			Name: "vp9_enc",
			Val:  rtcTest{codec: peerconnection.Encoding, profile: "VP9"},
			// TODO(crbug.com/811912): Remove "vaapi" and pre.ChromeVideoWithFakeWebcamAndVP9VaapiEncoder()
			// once the feature is enabled by default on VA-API devices.
			ExtraSoftwareDeps: []string{caps.HWEncodeVP9, "vaapi"},
			Pre:               pre.ChromeVideoWithFakeWebcamAndVP9VaapiEncoder(),
		}, {
			Name:              "h264_enc",
			Val:               rtcTest{codec: peerconnection.Encoding, profile: "H264"},
			ExtraSoftwareDeps: []string{caps.HWEncodeH264, "chrome_internal"}, // "chrome_internal" is needed because H.264 is a proprietary codec.
			Pre:               pre.ChromeVideoWithFakeWebcamAndH264AMDEncoder(),
		}, {
			Name:              "h264_dec",
			Val:               rtcTest{codec: peerconnection.Decoding, profile: "H264"},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "chrome_internal"}, // "chrome_internal" is needed because H.264 is a proprietary codec.
			Pre:               pre.ChromeVideoWithFakeWebcam(),
		}, {
			Name: "vp8",
			Val:  rtcTest{codec: peerconnection.DontCare, profile: "VP8"},
			Pre:  pre.ChromeVideoWithFakeWebcam(),
		}, {
			Name: "vp9",
			Val:  rtcTest{codec: peerconnection.DontCare, profile: "VP9"},
			Pre:  pre.ChromeVideoWithFakeWebcam(),
		}, {
			Name:              "h264",
			Val:               rtcTest{codec: peerconnection.DontCare, profile: "H264"},
			ExtraSoftwareDeps: []string{"chrome_internal"}, // "chrome_internal" is needed because H.264 is a proprietary codec.
			Pre:               pre.ChromeVideoWithFakeWebcam(),
		}},
	})
}

// RTCPeerConnection verifies that a PeerConnection works correcttly and, if
// specified, verifies it uses accelerated encoding / decoding.
func RTCPeerConnection(ctx context.Context, s *testing.State) {
	testOpt := s.Param().(rtcTest)
	peerconnection.RunRTCPeerConnection(ctx, s, s.PreValue().(*chrome.Chrome), testOpt.codec, testOpt.profile)
}
