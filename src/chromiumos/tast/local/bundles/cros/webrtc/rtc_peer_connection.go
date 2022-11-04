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

type testOptions struct {
	simulcasts       int
	svc              string
	displayMediaType peerconnection.DisplayMediaType
}

func noVerifyCodecTest(profile string, opts testOptions) peerconnection.RTCTestParams {
	return peerconnection.MakeRTCTestParams(
		peerconnection.NoVerifyDecoderMode, peerconnection.NoVerifyEncoderMode,
		profile, opts.simulcasts, opts.svc, opts.displayMediaType)

}

func hwDecoderTest(profile string, opts testOptions) peerconnection.RTCTestParams {
	return peerconnection.MakeRTCTestParams(
		peerconnection.VerifyHWDecoderUsed, peerconnection.NoVerifyEncoderMode,
		profile, opts.simulcasts, opts.svc, opts.displayMediaType)
}

func hwEncoderTest(profile string, opts testOptions) peerconnection.RTCTestParams {
	return peerconnection.MakeRTCTestParams(
		peerconnection.NoVerifyDecoderMode, peerconnection.VerifyHWEncoderUsed,
		profile, opts.simulcasts, opts.svc, opts.displayMediaType)
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
			Val:               noVerifyCodecTest("H264", testOptions{}),
			ExtraSoftwareDeps: []string{"proprietary_codecs"},
			Fixture:           "chromeVideoWithFakeWebcam",
		}, {
			Name:    "vp8",
			Val:     noVerifyCodecTest("VP8", testOptions{}),
			Fixture: "chromeVideoWithFakeWebcam",
		}, {
			Name:    "vp8_simulcast",
			Val:     noVerifyCodecTest("VP8", testOptions{simulcasts: 3}),
			Fixture: "chromeVideoWithFakeWebcam",
		}, {
			Name:    "vp9",
			Val:     noVerifyCodecTest("VP9", testOptions{}),
			Fixture: "chromeVideoWithFakeWebcam",
		}, {
			Name:              "h264_dec",
			Val:               hwDecoderTest("H264", testOptions{}),
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "proprietary_codecs"},
			Fixture:           "chromeVideoWithFakeWebcam",
		}, {
			Name:              "h264_dec_alt",
			Val:               hwDecoderTest("H264", testOptions{}),
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "video_decoder_legacy_supported", "proprietary_codecs"},
			Fixture:           "chromeVideoWithFakeWebcamAndAlternateVideoDecoder",
		}, {
			Name:              "vp8_dec",
			Val:               hwDecoderTest("VP8", testOptions{}),
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8},
			Fixture:           "chromeVideoWithFakeWebcam",
		}, {
			Name:              "vp8_dec_alt",
			Val:               hwDecoderTest("VP8", testOptions{}),
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8, "video_decoder_legacy_supported"},
			Fixture:           "chromeVideoWithFakeWebcamAndAlternateVideoDecoder",
		}, {
			Name:              "vp9_dec",
			Val:               hwDecoderTest("VP9", testOptions{}),
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
			Fixture:           "chromeVideoWithFakeWebcam",
		}, {
			Name:              "vp9_dec_alt",
			Val:               hwDecoderTest("VP9", testOptions{}),
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9, "video_decoder_legacy_supported"},
			Fixture:           "chromeVideoWithFakeWebcamAndAlternateVideoDecoder",
		}, {
			// This is a decoding test of 2 temporal layers test, via the (experimental) API.
			// See https://www.w3.org/TR/webrtc-svc/#scalabilitymodes for SVC identifiers.
			Name:              "vp9_dec_svc_l1t2",
			Val:               hwDecoderTest("VP9", testOptions{svc: "L1T2"}),
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
			Fixture:           "chromeVideoWithFakeWebcamAndSVCEnabled",
		}, {
			// This is a decoding test of 3 temporal layers test, via the (experimental) API.
			// See https://www.w3.org/TR/webrtc-svc/#scalabilitymodes for SVC identifiers.
			Name:              "vp9_dec_svc_l1t3",
			Val:               hwDecoderTest("VP9", testOptions{svc: "L1T3"}),
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
			Fixture:           "chromeVideoWithFakeWebcamAndSVCEnabled",
		}, {
			// This is a decoding test of 3 spatial layers, 3 temporal layers (each) k-SVC test, via the (experimental) API.
			// See https://www.w3.org/TR/webrtc-svc/#scalabilitymodes for SVC identifiers.
			Name:              "vp9_dec_svc_l3t3_key",
			Val:               hwDecoderTest("VP9", testOptions{svc: "L3T3_KEY"}),
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
			ExtraHardwareDeps: hwdep.D(hwdep.SupportsVP9KSVCHWDecoding()),
			Fixture:           "chromeVideoWithFakeWebcamAndSVCEnabled",
		}, {
			Name:              "h264_enc",
			Val:               hwEncoderTest("H264", testOptions{}),
			ExtraSoftwareDeps: []string{caps.HWEncodeH264, "proprietary_codecs"},
			Fixture:           "chromeVideoWithFakeWebcam",
		}, {
			Name:              "h264_enc_cam",
			Val:               hwEncoderTest("H264", testOptions{}),
			ExtraSoftwareDeps: []string{caps.BuiltinCamera, caps.HWEncodeH264, "proprietary_codecs"},
			Fixture:           "chromeCameraPerf",
		}, {
			Name:              "vp8_enc",
			Val:               hwEncoderTest("VP8", testOptions{}),
			ExtraSoftwareDeps: []string{caps.HWEncodeVP8},
			Fixture:           "chromeVideoWithFakeWebcam",
		}, {
			Name:              "vp8_enc_cam",
			Val:               hwEncoderTest("VP8", testOptions{}),
			ExtraSoftwareDeps: []string{caps.BuiltinCamera, caps.HWEncodeVP8},
			Fixture:           "chromeCameraPerf",
		}, {
			Name:              "vp8_enc_simulcast",
			Val:               hwEncoderTest("VP8", testOptions{simulcasts: 3}),
			ExtraSoftwareDeps: []string{caps.HWEncodeVP8},
			Fixture:           "chromeVideoWithFakeWebcam",
		}, {
			Name:              "vp8_capture_monitor",
			Val:               hwEncoderTest("VP8", testOptions{displayMediaType: peerconnection.CaptureMonitor}),
			ExtraHardwareDeps: hwdep.D(hwdep.InternalDisplay()),
			ExtraSoftwareDeps: []string{caps.HWEncodeVP8},
			Fixture:           "chromeScreenCapture",
		}, {
			Name:              "vp8_capture_window",
			Val:               hwEncoderTest("VP8", testOptions{displayMediaType: peerconnection.CaptureWindow}),
			ExtraSoftwareDeps: []string{caps.HWEncodeVP8},
			Fixture:           "chromeWindowCapture",
		}, {
			Name:              "vp8_capture_tab",
			Val:               hwEncoderTest("VP8", testOptions{displayMediaType: peerconnection.CaptureTab}),
			ExtraSoftwareDeps: []string{caps.HWEncodeVP8},
			Fixture:           "chromeTabCapture",
		}, {
			// This is a 2 temporal layers test, via the (experimental) API.
			// See https://www.w3.org/TR/webrtc-svc/#scalabilitymodes for SVC identifiers.
			Name:              "vp8_enc_svc_l1t2",
			Val:               hwEncoderTest("VP8", testOptions{svc: "L1T2"}),
			ExtraSoftwareDeps: []string{caps.HWEncodeVP8},
			Fixture:           "chromeVideoWithFakeWebcamAndSVCEnabledWithHWVp8TemporalLayerEncoding",
		}, {
			// This is an encoding test of 3 temporal layers test, via the (experimental) API.
			// See https://www.w3.org/TR/webrtc-svc/#scalabilitymodes for SVC identifiers.
			Name:              "vp8_enc_svc_l1t3",
			Val:               hwEncoderTest("VP8", testOptions{svc: "L1T3"}),
			ExtraSoftwareDeps: []string{caps.HWEncodeVP8},
			Fixture:           "chromeVideoWithFakeWebcamAndSVCEnabledWithHWVp8TemporalLayerEncoding",
		}, {
			Name:    "vp9_enc",
			Val:     hwEncoderTest("VP9", testOptions{}),
			Fixture: "chromeVideoWithFakeWebcam",
		}, {
			Name:              "vp9_enc_cam",
			Val:               hwEncoderTest("VP9", testOptions{}),
			ExtraSoftwareDeps: []string{caps.BuiltinCamera, caps.HWEncodeVP9},
			Fixture:           "chromeCameraPerf",
		}, {
			// This is a 2 temporal layers test, via the (experimental) API.
			// See https://www.w3.org/TR/webrtc-svc/#scalabilitymodes for SVC identifiers.
			Name:              "vp9_enc_svc_l1t2",
			Val:               hwEncoderTest("VP9", testOptions{svc: "L1T2"}),
			ExtraSoftwareDeps: []string{caps.HWEncodeVP9},
			Fixture:           "chromeVideoWithFakeWebcamAndSVCEnabled",
		}, {
			// This is an encoding test of 3 temporal layers test, via the (experimental) API.
			// See https://www.w3.org/TR/webrtc-svc/#scalabilitymodes for SVC identifiers.
			Name:              "vp9_enc_svc_l1t3",
			Val:               hwEncoderTest("VP9", testOptions{svc: "L1T3"}),
			ExtraSoftwareDeps: []string{caps.HWEncodeVP9},
			Fixture:           "chromeVideoWithFakeWebcamAndSVCEnabled",
		}, {
			// This is an encoding test of 3 spatial layers, 3 temporal layers (each) k-SVC test, via the (experimental) API.
			// See https://www.w3.org/TR/webrtc-svc/#scalabilitymodes for SVC identifiers.
			Name:              "vp9_enc_svc_l3t3_key",
			Val:               hwEncoderTest("VP9", testOptions{svc: "L3T3_KEY"}),
			ExtraSoftwareDeps: []string{caps.HWEncodeVP9},
			Fixture:           "chromeVideoWithFakeWebcamAndSVCEnabled",
		}},
	})
}

// RTCPeerConnection verifies that a PeerConnection works correcttly and, if
// specified, verifies it uses accelerated encoding / decoding.
func RTCPeerConnection(ctx context.Context, s *testing.State) {
	if err := peerconnection.RunRTCPeerConnection(
		ctx, s.FixtValue().(*chrome.Chrome), s.DataFileSystem(),
		s.Param().(peerconnection.RTCTestParams)); err != nil {
		s.Error("Failed to run RunRTCPeerConnection: ", err)
	}
}
