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

const (
	defaultRTCStreamWidth  = 1280
	defaultRTCStreamHeight = 720
)

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
			Name: "h264",
			Val: peerconnection.RTCTestParams{
				VerifyDecoderMode: peerconnection.NoVerifyDecoderMode,
				VerifyEncoderMode: peerconnection.NoVerifyEncoderMode,
				Profile:           "H264",
				StreamWidth:       defaultRTCStreamWidth,
				StreamHeight:      defaultRTCStreamHeight,
			},
			ExtraSoftwareDeps: []string{"proprietary_codecs"},
			Fixture:           "chromeVideoWithFakeWebcam",
		}, {
			Name: "vp8",
			Val: peerconnection.RTCTestParams{
				VerifyDecoderMode: peerconnection.NoVerifyDecoderMode,
				VerifyEncoderMode: peerconnection.NoVerifyEncoderMode,
				Profile:           "VP8",
				StreamWidth:       defaultRTCStreamWidth,
				StreamHeight:      defaultRTCStreamHeight,
			},
			Fixture: "chromeVideoWithFakeWebcam",
		}, {
			Name: "vp8_simulcast",
			Val: peerconnection.RTCTestParams{
				VerifyDecoderMode: peerconnection.NoVerifyDecoderMode,
				VerifyEncoderMode: peerconnection.NoVerifyEncoderMode,
				Profile:           "VP8",
				StreamWidth:       defaultRTCStreamWidth,
				StreamHeight:      defaultRTCStreamHeight,
				Simulcasts:        3,
			},
			Fixture: "chromeVideoWithFakeWebcam",
		}, {
			Name: "vp9",
			Val: peerconnection.RTCTestParams{
				VerifyDecoderMode: peerconnection.NoVerifyDecoderMode,
				VerifyEncoderMode: peerconnection.NoVerifyEncoderMode,
				Profile:           "VP9",
				StreamWidth:       defaultRTCStreamWidth,
				StreamHeight:      defaultRTCStreamHeight,
			},
			Fixture: "chromeVideoWithFakeWebcam",
		}, {
			Name: "h264_dec",
			Val: peerconnection.RTCTestParams{
				VerifyDecoderMode: peerconnection.VerifyHWDecoderUsed,
				VerifyEncoderMode: peerconnection.NoVerifyEncoderMode,
				Profile:           "H264",
				StreamWidth:       defaultRTCStreamWidth,
				StreamHeight:      defaultRTCStreamHeight,
			},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "proprietary_codecs"},
			Fixture:           "chromeVideoWithFakeWebcam",
		}, {
			Name: "h264_dec_alt",
			Val: peerconnection.RTCTestParams{
				VerifyDecoderMode: peerconnection.VerifyHWDecoderUsed,
				VerifyEncoderMode: peerconnection.NoVerifyEncoderMode,
				Profile:           "H264",
				StreamWidth:       defaultRTCStreamWidth,
				StreamHeight:      defaultRTCStreamHeight,
			},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "video_decoder_legacy_supported", "proprietary_codecs"},
			Fixture:           "chromeVideoWithFakeWebcamAndAlternateVideoDecoder",
		}, {
			Name: "vp8_dec",
			Val: peerconnection.RTCTestParams{
				VerifyDecoderMode: peerconnection.VerifyHWDecoderUsed,
				VerifyEncoderMode: peerconnection.NoVerifyEncoderMode,
				Profile:           "VP8",
				StreamWidth:       defaultRTCStreamWidth,
				StreamHeight:      defaultRTCStreamHeight,
			},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8},
			Fixture:           "chromeVideoWithFakeWebcam",
		}, {
			Name: "vp8_dec_alt",
			Val: peerconnection.RTCTestParams{
				VerifyDecoderMode: peerconnection.VerifyHWDecoderUsed,
				VerifyEncoderMode: peerconnection.NoVerifyEncoderMode,
				Profile:           "VP8",
				StreamWidth:       defaultRTCStreamWidth,
				StreamHeight:      defaultRTCStreamHeight,
			},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8, "video_decoder_legacy_supported"},
			Fixture:           "chromeVideoWithFakeWebcamAndAlternateVideoDecoder",
		}, {
			Name: "vp9_dec",
			Val: peerconnection.RTCTestParams{
				VerifyDecoderMode: peerconnection.VerifyHWDecoderUsed,
				VerifyEncoderMode: peerconnection.NoVerifyEncoderMode,
				Profile:           "VP9",
				StreamWidth:       defaultRTCStreamWidth,
				StreamHeight:      defaultRTCStreamHeight,
			},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
			Fixture:           "chromeVideoWithFakeWebcam",
		}, {
			Name: "vp9_dec_alt",
			Val: peerconnection.RTCTestParams{
				VerifyDecoderMode: peerconnection.VerifyHWDecoderUsed,
				VerifyEncoderMode: peerconnection.NoVerifyEncoderMode,
				Profile:           "VP9",
				StreamWidth:       defaultRTCStreamWidth,
				StreamHeight:      defaultRTCStreamHeight,
			},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9, "video_decoder_legacy_supported"},
			Fixture:           "chromeVideoWithFakeWebcamAndAlternateVideoDecoder",
		}, {
			Name: "vp9_dec_1080p",
			Val: peerconnection.RTCTestParams{
				VerifyDecoderMode: peerconnection.VerifyHWDecoderUsed,
				VerifyEncoderMode: peerconnection.NoVerifyEncoderMode,
				Profile:           "VP9",
				StreamWidth:       1920,
				StreamHeight:      1080,
			},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
			Fixture:           "chromeVideoWithFakeWebcam",
		}, {
			// This is a decoding test of 2 temporal layers test, via the (experimental) API.
			// See https://www.w3.org/TR/webrtc-svc/#scalabilitymodes for SVC identifiers.
			Name: "vp9_dec_svc_l1t2",
			Val: peerconnection.RTCTestParams{
				VerifyDecoderMode: peerconnection.VerifyHWDecoderUsed,
				VerifyEncoderMode: peerconnection.NoVerifyEncoderMode,
				Profile:           "VP9",
				StreamWidth:       defaultRTCStreamWidth,
				StreamHeight:      defaultRTCStreamHeight,
				Svc:               "L1T2",
			},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
			Fixture:           "chromeVideoWithFakeWebcamAndSVCEnabled",
		}, {
			// This is a decoding test of 3 temporal layers test, via the (experimental) API.
			// See https://www.w3.org/TR/webrtc-svc/#scalabilitymodes for SVC identifiers.
			Name: "vp9_dec_svc_l1t3",
			Val: peerconnection.RTCTestParams{
				VerifyDecoderMode: peerconnection.VerifyHWDecoderUsed,
				VerifyEncoderMode: peerconnection.NoVerifyEncoderMode,
				Profile:           "VP9",
				StreamWidth:       defaultRTCStreamWidth,
				StreamHeight:      defaultRTCStreamHeight,
				Svc:               "L1T3",
			},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
			Fixture:           "chromeVideoWithFakeWebcamAndSVCEnabled",
		}, {
			// This is a decoding test of 3 spatial layers, 3 temporal layers (each) k-SVC test, via the (experimental) API.
			// See https://www.w3.org/TR/webrtc-svc/#scalabilitymodes for SVC identifiers.
			Name: "vp9_dec_svc_l3t3_key",
			Val: peerconnection.RTCTestParams{
				VerifyDecoderMode: peerconnection.VerifyHWDecoderUsed,
				VerifyEncoderMode: peerconnection.NoVerifyEncoderMode,
				Profile:           "VP9",
				StreamWidth:       defaultRTCStreamWidth,
				StreamHeight:      defaultRTCStreamHeight,
				Svc:               "L3T3_KEY",
			},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
			ExtraHardwareDeps: hwdep.D(hwdep.SupportsVP9KSVCHWDecoding()),
			Fixture:           "chromeVideoWithFakeWebcamAndSVCEnabled",
		}, {
			Name: "h264_enc",
			Val: peerconnection.RTCTestParams{
				VerifyDecoderMode: peerconnection.NoVerifyDecoderMode,
				VerifyEncoderMode: peerconnection.VerifyHWEncoderUsed,
				Profile:           "H264",
				StreamWidth:       defaultRTCStreamWidth,
				StreamHeight:      defaultRTCStreamHeight,
			},
			ExtraSoftwareDeps: []string{caps.HWEncodeH264, "proprietary_codecs"},
			Fixture:           "chromeVideoWithFakeWebcam",
		}, {
			Name: "h264_enc_cam",
			Val: peerconnection.RTCTestParams{
				VerifyDecoderMode: peerconnection.NoVerifyDecoderMode,
				VerifyEncoderMode: peerconnection.VerifyHWEncoderUsed,
				Profile:           "H264",
				StreamWidth:       defaultRTCStreamWidth,
				StreamHeight:      defaultRTCStreamHeight,
			},
			ExtraSoftwareDeps: []string{caps.BuiltinCamera, caps.HWEncodeH264, "proprietary_codecs"},
			Fixture:           "chromeCameraPerf",
		}, {
			Name: "vp8_enc",
			Val: peerconnection.RTCTestParams{
				VerifyDecoderMode: peerconnection.NoVerifyDecoderMode,
				VerifyEncoderMode: peerconnection.VerifyHWEncoderUsed,
				Profile:           "VP8",
				StreamWidth:       defaultRTCStreamWidth,
				StreamHeight:      defaultRTCStreamHeight,
			},
			ExtraSoftwareDeps: []string{caps.HWEncodeVP8},
			Fixture:           "chromeVideoWithFakeWebcam",
		}, {
			Name: "vp8_enc_cam",
			Val: peerconnection.RTCTestParams{
				VerifyDecoderMode: peerconnection.NoVerifyDecoderMode,
				VerifyEncoderMode: peerconnection.VerifyHWEncoderUsed,
				Profile:           "VP8",
				StreamWidth:       defaultRTCStreamWidth,
				StreamHeight:      defaultRTCStreamHeight,
			},
			ExtraSoftwareDeps: []string{caps.BuiltinCamera, caps.HWEncodeVP8},
			Fixture:           "chromeCameraPerf",
		}, {
			Name: "vp8_enc_simulcast",
			Val: peerconnection.RTCTestParams{
				VerifyDecoderMode: peerconnection.NoVerifyDecoderMode,
				VerifyEncoderMode: peerconnection.VerifyHWEncoderUsed,
				Profile:           "VP8",
				StreamWidth:       defaultRTCStreamWidth,
				StreamHeight:      defaultRTCStreamHeight,
				Simulcasts:        3,
			},
			ExtraSoftwareDeps: []string{caps.HWEncodeVP8},
			Fixture:           "chromeVideoWithFakeWebcam",
		}, {
			Name: "vp8_capture_monitor",
			Val: peerconnection.RTCTestParams{
				VerifyDecoderMode: peerconnection.NoVerifyDecoderMode,
				VerifyEncoderMode: peerconnection.VerifyHWEncoderUsed,
				Profile:           "VP8",
				StreamWidth:       defaultRTCStreamWidth,
				StreamHeight:      defaultRTCStreamHeight,
				DisplayMediaType:  peerconnection.CaptureMonitor,
			},
			ExtraHardwareDeps: hwdep.D(hwdep.InternalDisplay()),
			ExtraSoftwareDeps: []string{caps.HWEncodeVP8},
			Fixture:           "chromeScreenCapture",
		}, {
			Name: "vp8_capture_window",
			Val: peerconnection.RTCTestParams{
				VerifyDecoderMode: peerconnection.NoVerifyDecoderMode,
				VerifyEncoderMode: peerconnection.VerifyHWEncoderUsed,
				Profile:           "VP8",
				StreamWidth:       defaultRTCStreamWidth,
				StreamHeight:      defaultRTCStreamHeight,
				DisplayMediaType:  peerconnection.CaptureWindow,
			},
			ExtraSoftwareDeps: []string{caps.HWEncodeVP8},
			Fixture:           "chromeWindowCapture",
		}, {
			Name: "vp8_capture_tab",
			Val: peerconnection.RTCTestParams{
				VerifyDecoderMode: peerconnection.NoVerifyDecoderMode,
				VerifyEncoderMode: peerconnection.VerifyHWEncoderUsed,
				Profile:           "VP8",
				StreamWidth:       defaultRTCStreamWidth,
				StreamHeight:      defaultRTCStreamHeight,
				DisplayMediaType:  peerconnection.CaptureTab,
			},
			ExtraSoftwareDeps: []string{caps.HWEncodeVP8},
			Fixture:           "chromeTabCapture",
		}, {
			// This is a 2 temporal layers test, via the (experimental) API.
			// See https://www.w3.org/TR/webrtc-svc/#scalabilitymodes for SVC identifiers.
			Name: "vp8_enc_svc_l1t2",
			Val: peerconnection.RTCTestParams{
				VerifyDecoderMode: peerconnection.NoVerifyDecoderMode,
				VerifyEncoderMode: peerconnection.VerifyHWEncoderUsed,
				Profile:           "VP8",
				StreamWidth:       defaultRTCStreamWidth,
				StreamHeight:      defaultRTCStreamHeight,
				Svc:               "L1T2",
			},
			ExtraSoftwareDeps: []string{caps.HWEncodeVP8},
			Fixture:           "chromeVideoWithFakeWebcamAndSVCEnabledWithHWVp8TemporalLayerEncoding",
		}, {
			// This is an encoding test of 3 temporal layers test, via the (experimental) API.
			// See https://www.w3.org/TR/webrtc-svc/#scalabilitymodes for SVC identifiers.
			Name: "vp8_enc_svc_l1t3",
			Val: peerconnection.RTCTestParams{
				VerifyDecoderMode: peerconnection.NoVerifyDecoderMode,
				VerifyEncoderMode: peerconnection.VerifyHWEncoderUsed,
				Profile:           "VP8",
				StreamWidth:       defaultRTCStreamWidth,
				StreamHeight:      defaultRTCStreamHeight,
				Svc:               "L1T3",
			},
			ExtraSoftwareDeps: []string{caps.HWEncodeVP8},
			Fixture:           "chromeVideoWithFakeWebcamAndSVCEnabledWithHWVp8TemporalLayerEncoding",
		}, {
			Name: "vp9_enc",
			Val: peerconnection.RTCTestParams{
				VerifyDecoderMode: peerconnection.NoVerifyDecoderMode,
				VerifyEncoderMode: peerconnection.VerifyHWEncoderUsed,
				Profile:           "VP9",
				StreamWidth:       defaultRTCStreamWidth,
				StreamHeight:      defaultRTCStreamHeight,
			},
			Fixture: "chromeVideoWithFakeWebcam",
		}, {
			Name: "vp9_enc_1080p",
			Val: peerconnection.RTCTestParams{
				VerifyDecoderMode: peerconnection.NoVerifyDecoderMode,
				VerifyEncoderMode: peerconnection.VerifyHWEncoderUsed,
				Profile:           "VP9",
				StreamWidth:       1920,
				StreamHeight:      1080,
			},
			Fixture: "chromeVideoWithFakeWebcam",
		}, {
			Name: "vp9_enc_cam",
			Val: peerconnection.RTCTestParams{
				VerifyDecoderMode: peerconnection.NoVerifyDecoderMode,
				VerifyEncoderMode: peerconnection.VerifyHWEncoderUsed,
				Profile:           "VP9",
				StreamWidth:       defaultRTCStreamWidth,
				StreamHeight:      defaultRTCStreamHeight,
			},
			ExtraSoftwareDeps: []string{caps.BuiltinCamera, caps.HWEncodeVP9},
			Fixture:           "chromeCameraPerf",
		}, {
			// This is a 2 temporal layers test, via the (experimental) API.
			// See https://www.w3.org/TR/webrtc-svc/#scalabilitymodes for SVC identifiers.
			Name: "vp9_enc_svc_l1t2",
			Val: peerconnection.RTCTestParams{
				VerifyDecoderMode: peerconnection.NoVerifyDecoderMode,
				VerifyEncoderMode: peerconnection.VerifyHWEncoderUsed,
				Profile:           "VP9",
				StreamWidth:       defaultRTCStreamWidth,
				StreamHeight:      defaultRTCStreamHeight,
				Svc:               "L1T2",
			},
			ExtraSoftwareDeps: []string{caps.HWEncodeVP9},
			Fixture:           "chromeVideoWithFakeWebcamAndSVCEnabled",
		}, {
			// This is an encoding test of 3 temporal layers test, via the (experimental) API.
			// See https://www.w3.org/TR/webrtc-svc/#scalabilitymodes for SVC identifiers.
			Name: "vp9_enc_svc_l1t3",
			Val: peerconnection.RTCTestParams{
				VerifyDecoderMode: peerconnection.NoVerifyDecoderMode,
				VerifyEncoderMode: peerconnection.VerifyHWEncoderUsed,
				Profile:           "VP9",
				StreamWidth:       defaultRTCStreamWidth,
				StreamHeight:      defaultRTCStreamHeight,
				Svc:               "L1T3",
			},
			ExtraSoftwareDeps: []string{caps.HWEncodeVP9},
			Fixture:           "chromeVideoWithFakeWebcamAndSVCEnabled",
		}, {
			// This is an encoding test of 3 spatial layers, 3 temporal layers (each) k-SVC test, via the (experimental) API.
			// See https://www.w3.org/TR/webrtc-svc/#scalabilitymodes for SVC identifiers.
			Name: "vp9_enc_svc_l3t3_key",
			Val: peerconnection.RTCTestParams{
				VerifyDecoderMode: peerconnection.NoVerifyDecoderMode,
				VerifyEncoderMode: peerconnection.VerifyHWEncoderUsed,
				Profile:           "VP9",
				StreamWidth:       defaultRTCStreamWidth,
				StreamHeight:      defaultRTCStreamHeight,
				Svc:               "L3T3_KEY",
			},
			ExtraSoftwareDeps: []string{caps.HWEncodeVP9},
			Fixture:           "chromeVideoWithFakeWebcamAndSVCEnabled",
		}},
	})
}

// RTCPeerConnection verifies that a PeerConnection works correcttly and, if
// specified, verifies it uses accelerated encoding / decoding.
func RTCPeerConnection(ctx context.Context, s *testing.State) {
	params := s.Param().(peerconnection.RTCTestParams)
	if err := peerconnection.RunRTCPeerConnection(
		ctx, s.FixtValue().(*chrome.Chrome), s.DataFileSystem(), params); err != nil {
		s.Error("Failed to run RunRTCPeerConnection: ", err)
	}
}
