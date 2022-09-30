// Copyright 2019 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package webrtc

import (
	"context"

	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/local/bundles/cros/webrtc/peerconnection"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
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
	// ScalableVideoCodec "scalabilityMode" identifier.
	// https://www.w3.org/TR/webrtc-svc/#scalabilitymodes
	svc string
	// If non-empty, the media to send through the RTC connection will be obtained
	// using getDisplayMedia() and the value corresponds to the surface type. If
	// empty, the media to send will be obtained using getUserMedia().
	displayMediaType peerconnection.DisplayMediaType
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         RTCPeerConnection,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Verifies that WebRTC RTCPeerConnection works, maybe verifying use of a hardware accelerator",
		Contacts: []string{
			"mcasas@chromium.org", // Test author.
			"hiroh@chromium.org",
			"chromeos-gfx-video@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Data:         append(peerconnection.DataFiles(), peerconnection.LoopbackFile),
		Attr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
		Params: []testing.Param{{
			Name:              "h264",
			Val:               rtcTest{verifyMode: peerconnection.NoVerifyHWAcceleratorUsed, profile: "H264"},
			ExtraSoftwareDeps: []string{"proprietary_codecs"},
			Fixture:           "chromeVideoWithFakeWebcam",
		}, {
			Name:    "vp8",
			Val:     rtcTest{verifyMode: peerconnection.NoVerifyHWAcceleratorUsed, profile: "VP8"},
			Fixture: "chromeVideoWithFakeWebcam",
		}, {
			Name:    "vp8_simulcast",
			Val:     rtcTest{verifyMode: peerconnection.NoVerifyHWAcceleratorUsed, profile: "VP8", simulcast: true},
			Fixture: "chromeVideoWithFakeWebcam",
		}, {
			Name:    "vp9",
			Val:     rtcTest{verifyMode: peerconnection.NoVerifyHWAcceleratorUsed, profile: "VP9"},
			Fixture: "chromeVideoWithFakeWebcam",
		}, {
			Name:              "h264_dec",
			Val:               rtcTest{verifyMode: peerconnection.VerifyHWDecoderUsed, profile: "H264"},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "proprietary_codecs"},
			Fixture:           "chromeVideoWithFakeWebcam",
		}, {
			Name:              "h264_dec_alt",
			Val:               rtcTest{verifyMode: peerconnection.VerifyHWDecoderUsed, profile: "H264"},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "video_decoder_legacy_supported", "proprietary_codecs"},
			Fixture:           "chromeVideoWithFakeWebcamAndAlternateVideoDecoder",
		}, {
			Name:              "vp8_dec",
			Val:               rtcTest{verifyMode: peerconnection.VerifyHWDecoderUsed, profile: "VP8"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8},
			Fixture:           "chromeVideoWithFakeWebcam",
		}, {
			Name:              "vp8_dec_alt",
			Val:               rtcTest{verifyMode: peerconnection.VerifyHWDecoderUsed, profile: "VP8"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8, "video_decoder_legacy_supported"},
			Fixture:           "chromeVideoWithFakeWebcamAndAlternateVideoDecoder",
		}, {
			Name:              "vp9_dec",
			Val:               rtcTest{verifyMode: peerconnection.VerifyHWDecoderUsed, profile: "VP9"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
			Fixture:           "chromeVideoWithFakeWebcam",
		}, {
			Name:              "vp9_dec_alt",
			Val:               rtcTest{verifyMode: peerconnection.VerifyHWDecoderUsed, profile: "VP9"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9, "video_decoder_legacy_supported"},
			Fixture:           "chromeVideoWithFakeWebcamAndAlternateVideoDecoder",
		}, {
			// This is a decoding test of 2 temporal layers test, via the (experimental) API.
			// See https://www.w3.org/TR/webrtc-svc/#scalabilitymodes for SVC identifiers.
			Name:              "vp9_dec_svc_l1t2",
			Val:               rtcTest{verifyMode: peerconnection.VerifyHWDecoderUsed, profile: "VP9", svc: "L1T2"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
			Fixture:           "chromeVideoWithFakeWebcamAndSVCEnabled",
		}, {
			// This is a decoding test of 3 temporal layers test, via the (experimental) API.
			// See https://www.w3.org/TR/webrtc-svc/#scalabilitymodes for SVC identifiers.
			Name:              "vp9_dec_svc_l1t3",
			Val:               rtcTest{verifyMode: peerconnection.VerifyHWDecoderUsed, profile: "VP9", svc: "L1T3"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
			Fixture:           "chromeVideoWithFakeWebcamAndSVCEnabled",
		}, {
			// This is a decoding test of 3 spatial layers, 3 temporal layers (each) k-SVC test, via the (experimental) API.
			// See https://www.w3.org/TR/webrtc-svc/#scalabilitymodes for SVC identifiers.
			Name:              "vp9_dec_svc_l3t3_key",
			Val:               rtcTest{verifyMode: peerconnection.VerifyHWDecoderUsed, profile: "VP9", svc: "L3T3_KEY"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
			ExtraHardwareDeps: hwdep.D(hwdep.SupportsVP9KSVCHWDecoding()),
			Fixture:           "chromeVideoWithFakeWebcamAndSVCEnabled",
		}, {
			Name:              "h264_enc",
			Val:               rtcTest{verifyMode: peerconnection.VerifyHWEncoderUsed, profile: "H264"},
			ExtraSoftwareDeps: []string{caps.HWEncodeH264, "proprietary_codecs"},
			Fixture:           "chromeVideoWithFakeWebcam",
		}, {
			Name:              "h264_enc_cam",
			Val:               rtcTest{verifyMode: peerconnection.VerifyHWEncoderUsed, profile: "H264"},
			ExtraSoftwareDeps: []string{caps.BuiltinCamera, caps.HWEncodeH264, "proprietary_codecs"},
			Fixture:           "chromeCameraPerf",
		}, {
			Name:              "vp8_enc",
			Val:               rtcTest{verifyMode: peerconnection.VerifyHWEncoderUsed, profile: "VP8"},
			ExtraSoftwareDeps: []string{caps.HWEncodeVP8},
			Fixture:           "chromeVideoWithFakeWebcam",
		}, {
			Name:              "vp8_enc_cam",
			Val:               rtcTest{verifyMode: peerconnection.VerifyHWEncoderUsed, profile: "VP8"},
			ExtraSoftwareDeps: []string{caps.BuiltinCamera, caps.HWEncodeVP8},
			Fixture:           "chromeCameraPerf",
		}, {
			Name:              "vp8_enc_simulcast",
			Val:               rtcTest{verifyMode: peerconnection.VerifyHWEncoderUsed, profile: "VP8", simulcast: true},
			ExtraSoftwareDeps: []string{caps.HWEncodeVP8},
			Fixture:           "chromeVideoWithFakeWebcam",
		}, {
			Name:              "vp8_capture_monitor",
			Val:               rtcTest{verifyMode: peerconnection.VerifyHWEncoderUsed, profile: "VP8", displayMediaType: peerconnection.CaptureMonitor},
			ExtraHardwareDeps: hwdep.D(hwdep.InternalDisplay()),
			ExtraSoftwareDeps: []string{caps.HWEncodeVP8},
			Fixture:           "chromeScreenCapture",
		}, {
			Name:              "vp8_capture_window",
			Val:               rtcTest{verifyMode: peerconnection.VerifyHWEncoderUsed, profile: "VP8", displayMediaType: peerconnection.CaptureWindow},
			ExtraSoftwareDeps: []string{caps.HWEncodeVP8},
			Fixture:           "chromeWindowCapture",
		}, {
			Name:              "vp8_capture_tab",
			Val:               rtcTest{verifyMode: peerconnection.VerifyHWEncoderUsed, profile: "VP8", displayMediaType: peerconnection.CaptureTab},
			ExtraSoftwareDeps: []string{caps.HWEncodeVP8},
			Fixture:           "chromeTabCapture",
		}, {
			// This is a 2 temporal layers test, via the (experimental) API.
			// See https://www.w3.org/TR/webrtc-svc/#scalabilitymodes for SVC identifiers.
			Name:              "vp8_enc_svc_l1t2",
			Val:               rtcTest{verifyMode: peerconnection.VerifyHWEncoderUsed, profile: "VP8", svc: "L1T2"},
			ExtraSoftwareDeps: []string{caps.HWEncodeVP8},
			Fixture:           "chromeVideoWithFakeWebcamAndSVCEnabledWithHWVp8TemporalLayerEncoding",
		}, {
			// This is an encoding test of 3 temporal layers test, via the (experimental) API.
			// See https://www.w3.org/TR/webrtc-svc/#scalabilitymodes for SVC identifiers.
			Name:              "vp8_enc_svc_l1t3",
			Val:               rtcTest{verifyMode: peerconnection.VerifyHWEncoderUsed, profile: "VP8", svc: "L1T3"},
			ExtraSoftwareDeps: []string{caps.HWEncodeVP8},
			Fixture:           "chromeVideoWithFakeWebcamAndSVCEnabledWithHWVp8TemporalLayerEncoding",
		}, {
			Name:              "vp9_enc",
			Val:               rtcTest{verifyMode: peerconnection.VerifyHWEncoderUsed, profile: "VP9"},
			ExtraSoftwareDeps: []string{caps.HWEncodeVP9},
			Fixture:           "chromeVideoWithFakeWebcam",
		}, {
			Name:              "vp9_enc_cam",
			Val:               rtcTest{verifyMode: peerconnection.VerifyHWEncoderUsed, profile: "VP9"},
			ExtraSoftwareDeps: []string{caps.BuiltinCamera, caps.HWEncodeVP9},
			Fixture:           "chromeCameraPerf",
		}, {
			// This is a 2 temporal layers test, via the (experimental) API.
			// See https://www.w3.org/TR/webrtc-svc/#scalabilitymodes for SVC identifiers.
			Name:              "vp9_enc_svc_l1t2",
			Val:               rtcTest{verifyMode: peerconnection.VerifyHWEncoderUsed, profile: "VP9", svc: "L1T2"},
			ExtraSoftwareDeps: []string{caps.HWEncodeVP9},
			Fixture:           "chromeVideoWithFakeWebcamAndSVCEnabled",
		}, {
			// This is an encoding test of 3 temporal layers test, via the (experimental) API.
			// See https://www.w3.org/TR/webrtc-svc/#scalabilitymodes for SVC identifiers.
			Name:              "vp9_enc_svc_l1t3",
			Val:               rtcTest{verifyMode: peerconnection.VerifyHWEncoderUsed, profile: "VP9", svc: "L1T3"},
			ExtraSoftwareDeps: []string{caps.HWEncodeVP9},
			Fixture:           "chromeVideoWithFakeWebcamAndSVCEnabled",
		}, {
			// This is an encoding test of 3 spatial layers, 3 temporal layers (each) k-SVC test, via the (experimental) API.
			// See https://www.w3.org/TR/webrtc-svc/#scalabilitymodes for SVC identifiers.
			Name:              "vp9_enc_svc_l3t3_key",
			Val:               rtcTest{verifyMode: peerconnection.VerifyHWEncoderUsed, profile: "VP9", svc: "L3T3_KEY"},
			ExtraSoftwareDeps: []string{caps.HWEncodeVP9},
			ExtraHardwareDeps: hwdep.D(hwdep.SupportsVP9KSVCHWEncoding()),
			Fixture:           "chromeVideoWithFakeWebcamAndSVCEnabled",
		}},
	})
}

// RTCPeerConnection verifies that a PeerConnection works correcttly and, if
// specified, verifies it uses accelerated encoding / decoding.
func RTCPeerConnection(ctx context.Context, s *testing.State) {
	testOpt := s.Param().(rtcTest)
	if err := peerconnection.RunRTCPeerConnection(ctx, s.FixtValue().(*chrome.Chrome), s.DataFileSystem(), testOpt.verifyMode, testOpt.profile, testOpt.simulcast, testOpt.svc, testOpt.displayMediaType); err != nil {
		s.Error("Failed to run RunRTCPeerConnection: ", err)
	}
}
