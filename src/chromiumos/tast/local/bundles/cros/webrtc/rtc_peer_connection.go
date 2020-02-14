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
	"chromiumos/tast/testing"
)

// rtcTest is used to describe the config used to run each test case.
type rtcTest struct {
	// Whether to verify if a hardware accelerator was used, and which one, e.g.
	// decoder, encoder, if so.
	verifyMode peerconnection.VerifyHWAcceleratorMode
	profile    string // Codec to try, e.g. VP8, VP9.
	// Simulcast is a technique to send multiple differently encoded versions
	// of the same media source in different RTP streams; this is used for
	// example for video conference services.
	// See https://www.w3.org/TR/webrtc/#simulcast-functionality
	simulcast bool
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
		Data:         append(peerconnection.DataFiles(), peerconnection.LoopbackFile),
		Attr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
		Params: []testing.Param{{
			Name:              "vp8_enc",
			Val:               rtcTest{verifyMode: peerconnection.VerifyHWEncoderUsed, profile: "VP8", simulcast: false},
			ExtraSoftwareDeps: []string{caps.HWEncodeVP8},
			Pre:               pre.ChromeVideoWithFakeWebcam(),
		}, {
			Name:              "vp8_dec",
			Val:               rtcTest{verifyMode: peerconnection.VerifyHWDecoderUsed, profile: "VP8", simulcast: false},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8},
			Pre:               pre.ChromeVideoWithFakeWebcam(),
		}, {
			Name:              "vp9_dec",
			Val:               rtcTest{verifyMode: peerconnection.VerifyHWDecoderUsed, profile: "VP9", simulcast: false},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
			Pre:               pre.ChromeVideoWithFakeWebcam(),
		}, {
			Name: "vp9_enc",
			Val:  rtcTest{verifyMode: peerconnection.VerifyHWEncoderUsed, profile: "VP9", simulcast: false},
			// TODO(crbug.com/811912): Remove "vaapi" and pre.ChromeVideoWithFakeWebcamAndVP9VaapiEncoder()
			// once the feature is enabled by default on VA-API devices.
			ExtraSoftwareDeps: []string{caps.HWEncodeVP9, "vaapi"},
			Pre:               pre.ChromeVideoWithFakeWebcamAndVP9VaapiEncoder(),
		}, {
			Name:              "h264_enc",
			Val:               rtcTest{verifyMode: peerconnection.VerifyHWEncoderUsed, profile: "H264", simulcast: false},
			ExtraSoftwareDeps: []string{caps.HWEncodeH264, "proprietary_codecs"},
			Pre:               pre.ChromeVideoWithFakeWebcam(),
		}, {
			Name:              "h264_dec",
			Val:               rtcTest{verifyMode: peerconnection.VerifyHWDecoderUsed, profile: "H264", simulcast: false},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "proprietary_codecs"},
			Pre:               pre.ChromeVideoWithFakeWebcam(),
		}, {
			Name: "vp8",
			Val:  rtcTest{verifyMode: peerconnection.NoVerifyHWAcceleratorUsed, profile: "VP8", simulcast: false},
			Pre:  pre.ChromeVideoWithFakeWebcam(),
		}, {
			Name: "vp9",
			Val:  rtcTest{verifyMode: peerconnection.NoVerifyHWAcceleratorUsed, profile: "VP9", simulcast: false},
			Pre:  pre.ChromeVideoWithFakeWebcam(),
		}, {
			Name:              "h264",
			Val:               rtcTest{verifyMode: peerconnection.NoVerifyHWAcceleratorUsed, profile: "H264", simulcast: false},
			ExtraSoftwareDeps: []string{"proprietary_codecs"},
			Pre:               pre.ChromeVideoWithFakeWebcam(),
		}, {
			Name: "vp8_simulcast",
			Val:  rtcTest{verifyMode: peerconnection.NoVerifyHWAcceleratorUsed, profile: "VP8", simulcast: true},
			Pre:  pre.ChromeVideoWithFakeWebcam(),
		}, {
			Name:              "vp8_enc_simulcast",
			Val:               rtcTest{verifyMode: peerconnection.VerifyHWEncoderUsed, profile: "VP8", simulcast: true},
			ExtraSoftwareDeps: []string{caps.HWEncodeVP8},
			Pre:               pre.ChromeVideoWithFakeWebcam(),
		}, {
			Name: "vp9_enc_temporal_layer",
			Val:  rtcTest{verifyMode: peerconnection.VerifyHWEncoderUsed, profile: "VP9", simulcast: false},
			// TODO(crbug.com/811912): Remove "vaapi" and replace pre.ChromeVideoWithFakeWebcamAndForceThreeTemporalLayersAndVP9VaapiEncoder()
			// with one to force only three temporal layers, once the feature is enabled by default on VA-API devices.
			ExtraSoftwareDeps: []string{caps.HWEncodeVP9, "vaapi"},
			Pre:               pre.ChromeVideoWithFakeWebcamAndForceThreeTemporalLayersAndVP9VaapiEncoder(),
		}, {
			Name:              "vp8_enc_cam",
			Val:               rtcTest{verifyMode: peerconnection.VerifyHWEncoderUsed, profile: "VP8", simulcast: false},
			ExtraSoftwareDeps: []string{caps.BuiltinCamera, caps.HWEncodeVP8},
			Pre:               pre.ChromeCameraPerf(),
		}, {
			Name:              "h264_enc_cam",
			Val:               rtcTest{verifyMode: peerconnection.VerifyHWEncoderUsed, profile: "H264", simulcast: false},
			ExtraSoftwareDeps: []string{caps.BuiltinCamera, caps.HWEncodeH264, "proprietary_codecs"},
			Pre:               pre.ChromeCameraPerf(),
		}, {
			Name:              "vp9_enc_cam",
			Val:               rtcTest{verifyMode: peerconnection.VerifyHWEncoderUsed, profile: "VP9", simulcast: false},
			ExtraSoftwareDeps: []string{caps.BuiltinCamera, caps.HWEncodeVP9},
			Pre:               pre.ChromeCameraPerfWithVP9VaapiEncoder(),
		}, {
			Name:              "vp8_dec_alt",
			Val:               rtcTest{verifyMode: peerconnection.VerifyHWDecoderUsed, profile: "VP8", simulcast: false},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8},
			Pre:               pre.ChromeVideoWithFakeWebcamAndAlternateVideoDecoder(),
		}, {
			Name:              "vp9_dec_alt",
			Val:               rtcTest{verifyMode: peerconnection.VerifyHWDecoderUsed, profile: "VP9", simulcast: false},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
			Pre:               pre.ChromeVideoWithFakeWebcamAndAlternateVideoDecoder(),
		}, {
			Name:              "h264_dec_alt",
			Val:               rtcTest{verifyMode: peerconnection.VerifyHWDecoderUsed, profile: "H264", simulcast: false},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "proprietary_codecs"},
			Pre:               pre.ChromeVideoWithFakeWebcamAndAlternateVideoDecoder(),
		}},
	})
}

// RTCPeerConnection verifies that a PeerConnection works correcttly and, if
// specified, verifies it uses accelerated encoding / decoding.
func RTCPeerConnection(ctx context.Context, s *testing.State) {
	testOpt := s.Param().(rtcTest)
	if err := peerconnection.RunRTCPeerConnection(ctx, s.PreValue().(*chrome.Chrome), s.DataFileSystem(), testOpt.verifyMode, testOpt.profile, testOpt.simulcast); err != nil {
		s.Error("Failed to run RunRTCPeerConnection: ", err)
	}
}
