// Copyright 2019 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package webrtc

import (
	"context"
	"time"

	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/local/bundles/cros/webrtc/peerconnection"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         RTCPeerConnectionPerf,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Measures WebRTC decode performance in terms of CPU usage and decode time with and without hardware acceleration",
		Contacts: []string{
			"hiroh@chromium.org",
			"mcasas@chromium.org",
			"chromeos-gfx-video@google.com",
		},
		Attr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
		SoftwareDeps: []string{"chrome"},
		Data:         append(peerconnection.DataFiles(), peerconnection.LoopbackFile),
		// TODO(crbug.com/1029548): Add more variations here, e.g. vp8.
		Params: []testing.Param{{
			Name:    "av1_sw",
			Val:     peerconnection.MakeSWTestOptions("AV1", 1280, 720),
			Fixture: "chromeVideoWithFakeWebcamAndNoHwAcceleration",
		}, {
			Name:              "h264_hw",
			Val:               peerconnection.MakeTestOptions("H264", 1280, 720),
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, caps.HWEncodeH264, "proprietary_codecs"},
			Fixture:           "chromeVideoWithFakeWebcam",
		}, {
			Name:              "h264_hw_lacros",
			Val:               peerconnection.MakeTestOptions("H264", 1280, 720),
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, caps.HWEncodeH264, "proprietary_codecs"},
			Fixture:           "chromeVideoWithFakeWebcamLacros",
		}, {
			Name:              "h264_sw",
			Val:               peerconnection.MakeSWTestOptions("H264", 1280, 720),
			ExtraSoftwareDeps: []string{"proprietary_codecs"},
			Fixture:           "chromeVideoWithFakeWebcamAndNoHwAcceleration",
		}, {
			Name:              "vp8_hw",
			Val:               peerconnection.MakeTestOptions("VP8", 1280, 720),
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8, caps.HWEncodeVP8},
			Fixture:           "chromeVideoWithFakeWebcam",
		}, {
			Name:              "vp8_hw_lacros",
			Val:               peerconnection.MakeTestOptions("VP8", 1280, 720),
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8, caps.HWEncodeVP8},
			Fixture:           "chromeVideoWithFakeWebcamLacros",
		}, {
			Name:    "vp8_sw",
			Val:     peerconnection.MakeSWTestOptions("VP8", 1280, 720),
			Fixture: "chromeVideoWithFakeWebcamAndNoHwAcceleration",
		}, {
			Name:              "vp9_hw",
			Val:               peerconnection.MakeTestOptions("VP9", 1280, 720),
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9, caps.HWEncodeVP9},
			Fixture:           "chromeVideoWithFakeWebcam",
		}, {
			Name:              "vp9_hw_lacros",
			Val:               peerconnection.MakeTestOptions("VP9", 1280, 720),
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9, caps.HWEncodeVP9},
			Fixture:           "chromeVideoWithFakeWebcamLacros",
		}, {
			Name:    "vp9_sw",
			Val:     peerconnection.MakeSWTestOptions("VP9", 1280, 720),
			Fixture: "chromeVideoWithFakeWebcamAndNoHwAcceleration",
		}, {
			Name:              "vp8_hw_capture_monitor",
			Val:               peerconnection.MakeCaptureTestOptions("VP8", 1280, 720, peerconnection.CaptureMonitor),
			ExtraHardwareDeps: hwdep.D(hwdep.InternalDisplay()),
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8, caps.HWEncodeVP8},
			Fixture:           "chromeScreenCapture",
		}, {
			Name:              "vp8_hw_capture_window",
			Val:               peerconnection.MakeCaptureTestOptions("VP8", 1280, 720, peerconnection.CaptureWindow),
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8, caps.HWEncodeVP8},
			Fixture:           "chromeWindowCapture",
		}, {
			Name:              "vp8_hw_capture_tab",
			Val:               peerconnection.MakeCaptureTestOptions("VP8", 1280, 720, peerconnection.CaptureTab),
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8, caps.HWEncodeVP8},
			Fixture:           "chromeTabCapture",
		}, {
			// This is a 2 temporal layers test, via the (experimental) API.
			// See https://www.w3.org/TR/webrtc-svc/#scalabilitymodes for SVC identifiers.
			Name:              "vp9_hw_svc_l1t2",
			Val:               peerconnection.MakeTestOptionsWithSVC("VP9", 1280, 720, "L1T2", true),
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9, caps.HWEncodeVP9},
			Fixture:           "chromeVideoWithFakeWebcamAndSVCEnabled",
		}, {
			// This is a 3 temporal layers test, via the (experimental) API.
			// See https://www.w3.org/TR/webrtc-svc/#scalabilitymodes for SVC identifiers.
			Name:              "vp9_hw_svc_l1t3",
			Val:               peerconnection.MakeTestOptionsWithSVC("VP9", 1280, 720, "L1T3", true),
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9, caps.HWEncodeVP9},
			Fixture:           "chromeVideoWithFakeWebcamAndSVCEnabled",
		}, {
			// This is a 3 spatial layers, 3 temporal layers (each) k-SVC test, via the (experimental) API.
			// See https://www.w3.org/TR/webrtc-svc/#scalabilitymodes for SVC identifiers.
			Name:              "vp9_hw_svc_l3t3_key",
			Val:               peerconnection.MakeTestOptionsWithSVC("VP9", 1280, 720, "L3T3_KEY", true),
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9, caps.HWEncodeVP9},
			ExtraHardwareDeps: hwdep.D(hwdep.SupportsVP9KSVCHWEncoding()),
			Fixture:           "chromeVideoWithFakeWebcamAndSVCEnabled",
		}, {
			Name:              "vp8_hw_multi_vp9_3x3",
			Val:               peerconnection.MakeTestOptionsWithVideoGrid("VP8", 1280, 720, 3, "tulip2-320x180.vp9.webm"),
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9, caps.HWDecodeVP8, caps.HWEncodeVP8},
			ExtraData:         []string{"tulip2-320x180.vp9.webm"},
			Fixture:           "chromeVideoWithFakeWebcam",
		}, {
			Name:              "vp8_hw_multi_vp9_4x4",
			Val:               peerconnection.MakeTestOptionsWithVideoGrid("VP8", 1280, 720, 4, "tulip2-320x180.vp9.webm"),
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9, caps.HWDecodeVP8, caps.HWEncodeVP8},
			ExtraData:         []string{"tulip2-320x180.vp9.webm"},
			Fixture:           "chromeVideoWithFakeWebcam",
			// Trogdor doesn't have enough hardware contexts to pass this test.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnPlatform("trogdor")),
		}, {
			Name:              "vp9_hw_multi_vp9_3x3",
			Val:               peerconnection.MakeTestOptionsWithVideoGrid("VP9", 1280, 720, 3, "tulip2-320x180.vp9.webm"),
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9, caps.HWEncodeVP9},
			ExtraData:         []string{"tulip2-320x180.vp9.webm"},
			Fixture:           "chromeVideoWithFakeWebcam",
		}, {
			Name:              "vp8_hw_multi_vp9_3x3_global_vaapi_lock_disabled",
			Val:               peerconnection.MakeTestOptionsWithVideoGrid("VP8", 1280, 720, 3, "tulip2-320x180.vp9.webm"),
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9, caps.HWDecodeVP8, caps.HWEncodeVP8, "thread_safe_libva_backend"},
			ExtraData:         []string{"tulip2-320x180.vp9.webm"},
			Fixture:           "chromeVideoWithFakeWebcamAndGlobalVaapiLockDisabled",
		}, {
			Name:              "vp8_hw_multi_vp9_4x4_global_vaapi_lock_disabled",
			Val:               peerconnection.MakeTestOptionsWithVideoGrid("VP8", 1280, 720, 4, "tulip2-320x180.vp9.webm"),
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9, caps.HWDecodeVP8, caps.HWEncodeVP8, "thread_safe_libva_backend"},
			ExtraData:         []string{"tulip2-320x180.vp9.webm"},
			Fixture:           "chromeVideoWithFakeWebcamAndGlobalVaapiLockDisabled",
		}, {
			Name:              "vp9_hw_multi_vp9_3x3_global_vaapi_lock_disabled",
			Val:               peerconnection.MakeTestOptionsWithVideoGrid("VP9", 1280, 720, 3, "tulip2-320x180.vp9.webm"),
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9, caps.HWEncodeVP9, "thread_safe_libva_backend"},
			ExtraData:         []string{"tulip2-320x180.vp9.webm"},
			Fixture:           "chromeVideoWithFakeWebcamAndGlobalVaapiLockDisabled",
		}, {
			Name:              "h264_180p_hw",
			Val:               peerconnection.MakeTestOptions("H264", 320, 180),
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, caps.HWEncodeH264, "proprietary_codecs"},
			Fixture:           "chromeVideoWithFakeWebcam",
		}, {
			Name:              "h264_180p_sw",
			Val:               peerconnection.MakeSWEncoderTestOptions("H264", 320, 180),
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, caps.HWEncodeH264, "proprietary_codecs"},
			Fixture:           "chromeVideoWithFakeWebcamAndSWEncoding",
		}, {
			Name:              "vp8_180p_hw",
			Val:               peerconnection.MakeTestOptions("VP8", 320, 180),
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8, caps.HWEncodeVP8},
			Fixture:           "chromeVideoWithFakeWebcam",
		}, {
			Name:              "vp8_180p_sw",
			Val:               peerconnection.MakeSWEncoderTestOptions("VP8", 320, 180),
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8, caps.HWEncodeVP8},
			Fixture:           "chromeVideoWithFakeWebcamAndSWEncoding",
		}, {
			Name:              "vp9_180p_hw",
			Val:               peerconnection.MakeTestOptions("VP9", 320, 180),
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9, caps.HWEncodeVP9},
			Fixture:           "chromeVideoWithFakeWebcam",
		}, {
			Name:              "vp9_180p_sw",
			Val:               peerconnection.MakeSWEncoderTestOptions("VP9", 320, 180),
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9, caps.HWEncodeVP9},
			Fixture:           "chromeVideoWithFakeWebcamAndSWEncoding",
		}, {
			Name:              "h264_360p_hw",
			Val:               peerconnection.MakeTestOptions("H264", 640, 360),
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, caps.HWEncodeH264, "proprietary_codecs"},
			Fixture:           "chromeVideoWithFakeWebcam",
		}, {
			Name:              "h264_360p_hw_lacros",
			Val:               peerconnection.MakeTestOptions("H264", 640, 360),
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, caps.HWEncodeH264, "proprietary_codecs"},
			Fixture:           "chromeVideoWithFakeWebcamLacros",
		}, {
			Name:              "h264_360p_sw",
			Val:               peerconnection.MakeSWEncoderTestOptions("H264", 640, 360),
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, caps.HWEncodeH264, "proprietary_codecs"},
			Fixture:           "chromeVideoWithFakeWebcamAndSWEncoding",
		}, {
			Name:              "vp8_360p_hw",
			Val:               peerconnection.MakeTestOptions("VP8", 640, 360),
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8, caps.HWEncodeVP8},
			Fixture:           "chromeVideoWithFakeWebcam",
		}, {
			Name:              "vp8_360p_hw_lacros",
			Val:               peerconnection.MakeTestOptions("VP8", 640, 360),
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8, caps.HWEncodeVP8},
			Fixture:           "chromeVideoWithFakeWebcamLacros",
		}, {
			Name:              "vp8_360p_sw",
			Val:               peerconnection.MakeSWEncoderTestOptions("VP8", 640, 360),
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8, caps.HWEncodeVP8},
			Fixture:           "chromeVideoWithFakeWebcamAndSWEncoding",
		}, {
			Name:              "vp9_360p_hw",
			Val:               peerconnection.MakeTestOptions("VP9", 640, 360),
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9, caps.HWEncodeVP9},
			Fixture:           "chromeVideoWithFakeWebcam",
		}, {
			Name:              "vp9_360p_hw_lacros",
			Val:               peerconnection.MakeTestOptions("VP9", 640, 360),
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9, caps.HWEncodeVP9},
			Fixture:           "chromeVideoWithFakeWebcamLacros",
		}, {
			Name:              "vp9_360p_sw",
			Val:               peerconnection.MakeSWEncoderTestOptions("VP9", 640, 360),
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9, caps.HWEncodeVP9},
			Fixture:           "chromeVideoWithFakeWebcamAndSWEncoding",
		}, {
			// VP8 simulcast compositing of two streams, 180p and 360p.
			// Both 180p and 360p streams are encoded by a software encoder.
			Name:              "vp8_simulcast_180_sw_360_sw",
			Val:               peerconnection.MakeSimulcastTestOptions("VP8", 640, 360, []bool{false, false}),
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8, caps.HWEncodeVP8},
			Fixture:           "chromeVideoWithFakeWebcamAndSWEncoding",
		}, {
			// VP8 simulcast compositing of two streams, 180p and 360p.
			// 180p is encoded by a software encoder and 360p is encoded by a hardware encoder.
			Name: "vp8_simulcast_180_sw_360_hw",
			Val:  peerconnection.MakeSimulcastTestOptions("VP8", 640, 360, []bool{false, true}),
			// Run VA-API only because V4L2 API encoders's supported resolution is less than 180p.
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8, caps.HWEncodeVP8, "vaapi"},
			Fixture:           "chromeVideoWithFakeWebcamAndEnableVaapiVideoMinResolution",
		}, {
			// VP8 simulcast compositing of two streams, 180p and 360p.
			// Both 180p and 360p streams are encoded by hardware encoders.
			Name:              "vp8_simulcast_180_hw_360_hw",
			Val:               peerconnection.MakeSimulcastTestOptions("VP8", 640, 360, []bool{true, true}),
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8, caps.HWEncodeVP8},
			Fixture:           "chromeVideoWithFakeWebcam",
		}, {
			// VP8 simulcast compositing of two streams, 180p, 360p and 720p.
			// The all streams are encoded by software encoders.
			Name:              "vp8_simulcast_180_sw_360_sw_720_sw",
			Val:               peerconnection.MakeSimulcastTestOptions("VP8", 1280, 720, []bool{false, false, false}),
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8, caps.HWEncodeVP8},
			Fixture:           "chromeVideoWithFakeWebcamAndSWEncoding",
		}, {
			// VP8 simulcast compositing of two streams, 180p, 360p and 720p.
			// 180p is encoded by a software encoder and the other two streams
			// are encoded by hardware encoders.
			Name: "vp8_simulcast_180_sw_360_hw_720_hw",
			Val:  peerconnection.MakeSimulcastTestOptions("VP8", 1280, 720, []bool{false, true, true}),
			// Run VA-API only because V4L2 API encoder's supported resolution is less than 180p.
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8, caps.HWEncodeVP8, "vaapi"},
			Fixture:           "chromeVideoWithFakeWebcamAndEnableVaapiVideoMinResolution",
		}, {
			// VP8 simulcast compositing of two streams, 180p, 360p and 720p.
			// The all streams are encoded by hardware encoders.
			Name:              "vp8_simulcast_180_hw_360_hw_720_hw",
			Val:               peerconnection.MakeSimulcastTestOptions("VP8", 1280, 720, []bool{true, true, true}),
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8, caps.HWEncodeVP8},
			Fixture:           "chromeVideoWithFakeWebcam",
		}, {
			Name:              "vp9_svc_l2t3_270p_sw",
			Val:               peerconnection.MakeTestOptionsWithSVC("VP9", 480, 270, "L3T3_KEY", false),
			ExtraHardwareDeps: hwdep.D(hwdep.SupportsVP9KSVCHWDecoding()),
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9, caps.HWEncodeVP9},
			Fixture:           "chromeVideoWithFakeWebcamAndSVCEnabledAndSWEncoding",
		}, {
			Name:              "vp9_svc_l2t3_270p_hw",
			Val:               peerconnection.MakeTestOptionsWithSVC("VP9", 480, 270, "L3T3_KEY", true),
			ExtraHardwareDeps: hwdep.D(hwdep.SupportsVP9KSVCHWDecoding()),
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9, caps.HWEncodeVP9},
			Fixture:           "chromeVideoWithFakeWebcamAndSVCEnabled",
		}, {
			Name:              "vp9_svc_l2t3_360p_sw",
			Val:               peerconnection.MakeTestOptionsWithSVC("VP9", 640, 360, "L3T3_KEY", false),
			ExtraHardwareDeps: hwdep.D(hwdep.SupportsVP9KSVCHWDecoding()),
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9, caps.HWEncodeVP9},
			Fixture:           "chromeVideoWithFakeWebcamAndSVCEnabledAndSWEncoding",
		}, {
			Name:              "vp9_svc_l2t3_360p_hw",
			Val:               peerconnection.MakeTestOptionsWithSVC("VP9", 640, 360, "L3T3_KEY", true),
			ExtraHardwareDeps: hwdep.D(hwdep.SupportsVP9KSVCHWDecoding()),
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9, caps.HWEncodeVP9},
			Fixture:           "chromeVideoWithFakeWebcamAndSVCEnabled",
		}},
		// Default timeout (i.e. 2 minutes) is not enough.
		Timeout: 10 * time.Minute,
	})
}

// RTCPeerConnectionPerf opens a WebRTC loopback page that loops a given capture stream to measure decode time and CPU usage.
func RTCPeerConnectionPerf(ctx context.Context, s *testing.State) {
	testOpt := s.Param().(peerconnection.RTCTestOptions)
	if err := peerconnection.RunRTCPeerConnectionPerf(ctx, s.FixtValue().(*chrome.Chrome), s.DataFileSystem(), s.OutDir(), testOpt); err != nil {
		s.Error("Failed to measure performance: ", err)
	}
}
